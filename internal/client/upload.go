package client

import (
	"bytes"
	"cmp"
	"context"
	"encoding/binary"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/PeterStoica/chatwire/internal/dial"
	"github.com/PeterStoica/chatwire/internal/media"
)

const (
	MaxUpload   = 100 << 20
	maxReply    = 1 << 16
	mediaIDMask = 1<<53 - 1
)

var (
	ErrUpload     = errors.New("client: upload failed")
	errUploadAuth = errors.New("client: the media servers refused the upload key")
	ErrTooLarge   = errors.New("client: file too large")
)

type Uploaded struct {
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileSHA256    []byte
	FileEncSHA256 []byte
	FileLength    uint64
}

type uploadReply struct {
	URL        string `json:"url"`
	DirectPath string `json:"direct_path"`
}

func (c *Client) Upload(ctx context.Context, t media.Type, data []byte) (Uploaded, error) {
	if len(data) > MaxUpload {
		return Uploaded{}, fmt.Errorf("%w: %d bytes, at most %d", ErrTooLarge, len(data), MaxUpload)
	}
	secret := make([]byte, media.KeySize+8)
	if _, err := io.ReadFull(c.cfg.Link.Random, secret); err != nil {
		return Uploaded{}, fmt.Errorf("client: media key: %w", err)
	}
	mediaKey, mediaID := secret[:media.KeySize], binary.BigEndian.Uint64(secret[media.KeySize:])&mediaIDMask|1
	sealed, err := media.Encrypt(mediaKey, t, data)
	if err != nil {
		return Uploaded{}, err
	}
	var failures []error
	for range 2 {
		conn, err := c.mediaConn(ctx)
		if err != nil {
			return Uploaded{}, err
		}
		hosts := slices.Clone(conn.Hosts)
		slices.SortStableFunc(hosts, func(a, b media.Host) int { return cmp.Compare(boolRank(a.Fallback), boolRank(b.Fallback)) })
		refused := false
		for _, host := range hosts {
			reply, err := c.post(ctx, media.UploadURL(host.Hostname, t, sealed.FileEncSHA256[:], conn.Auth, mediaID), sealed.File)
			if err != nil {
				failures = append(failures, err)
				refused = refused || errors.Is(err, errUploadAuth)
				continue
			}
			return Uploaded{
				URL: reply.URL, DirectPath: reply.DirectPath, MediaKey: mediaKey,
				FileSHA256: sealed.FileSHA256[:], FileEncSHA256: sealed.FileEncSHA256[:], FileLength: uint64(len(data)),
			}, nil
		}
		if !refused {
			break
		}
		c.mu.Lock()
		c.media = media.Conn{}
		c.mu.Unlock()
	}
	return Uploaded{}, fmt.Errorf("%w: %w", ErrUpload, errors.Join(failures...))
}

func (c *Client) post(ctx context.Context, address string, body []byte) (uploadReply, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(body))
	if err != nil {
		return uploadReply{}, err
	}
	request.Header.Set("Origin", dial.Origin)
	response, err := c.cfg.HTTP.Do(request)
	if err != nil {
		return uploadReply{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return uploadReply{}, fmt.Errorf("%w: %s answered %d", errUploadAuth, request.URL.Host, response.StatusCode)
	}
	if response.StatusCode != http.StatusOK {
		return uploadReply{}, fmt.Errorf("%s answered %d", request.URL.Host, response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxReply))
	if err != nil {
		return uploadReply{}, err
	}
	var reply uploadReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		return uploadReply{}, fmt.Errorf("%s answered %q: %w", request.URL.Host, raw, err)
	}
	if reply.URL == "" || reply.DirectPath == "" {
		return uploadReply{}, fmt.Errorf("%s answered without a url or direct path: %q", request.URL.Host, raw)
	}
	return reply, nil
}
