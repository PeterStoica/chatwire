package media

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PeterStoica/chatwire/internal/node"
)

var (
	ErrConn = errors.New("media: unexpected media_conn reply")
	ErrPath = errors.New("media: direct path leaves the media host")
)

type Host struct {
	Hostname string
	Fallback bool
}

type Conn struct {
	Auth    string
	AuthTTL time.Duration
	TTL     time.Duration
	Hosts   []Host
	Fetched time.Time
}

func ConnRequest() node.Node {
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: "to", Value: node.Address(node.JID{Server: node.ServerUser})}, {Key: "xmlns", Value: node.Text("w:m")},
			{Key: "type", Value: node.Text("set")}, {Key: "id", Value: node.Value{}},
		},
		Children: []node.Node{{Tag: "media_conn"}},
	}
}

func ParseConn(reply node.Node, now time.Time) (Conn, error) {
	conn, ok := reply.Child("media_conn")
	if kind, _ := reply.Attr("type").Text(); reply.Tag != "iq" || kind != "result" || !ok {
		return Conn{}, fmt.Errorf("%w: %s", ErrConn, reply)
	}
	auth, _ := conn.Attr("auth").Text()
	ttl, err := seconds(conn, "ttl")
	if err != nil {
		return Conn{}, err
	}
	authTTL, err := seconds(conn, "auth_ttl")
	if err != nil {
		return Conn{}, err
	}
	out := Conn{Auth: auth, TTL: ttl, AuthTTL: authTTL, Fetched: now}
	for _, host := range conn.Children {
		name, _ := host.Attr("hostname").Text()
		if host.Tag != "host" || name == "" {
			continue
		}
		kind, _ := host.Attr("type").Text()
		out.Hosts = append(out.Hosts, Host{Hostname: name, Fallback: kind == "fallback"})
	}
	if auth == "" || len(out.Hosts) == 0 {
		return Conn{}, fmt.Errorf("%w: no auth or no hosts", ErrConn)
	}
	return out, nil
}

func seconds(n node.Node, key string) (time.Duration, error) {
	raw, _ := n.Attr(key).Text()
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%w: %s=%q", ErrConn, key, raw)
	}
	return time.Duration(value) * time.Second, nil
}

func (c Conn) Fresh(now time.Time) bool {
	ttl := c.TTL
	if c.AuthTTL > 0 {
		ttl = min(ttl, c.AuthTTL)
	}
	return len(c.Hosts) > 0 && now.Before(c.Fetched.Add(ttl))
}

func DownloadURL(hostname, directPath string, fileEncSHA256 []byte, t Type) (string, error) {
	base := &url.URL{Scheme: "https", Host: hostname}
	target, err := base.Parse(directPath)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrPath, err)
	}
	if target.Scheme != "https" || target.Host != hostname {
		return "", fmt.Errorf("%w: %q", ErrPath, directPath)
	}
	query := target.Query()
	if len(fileEncSHA256) > 0 {
		query.Set("hash", base64.URLEncoding.EncodeToString(fileEncSHA256))
	}
	query.Set("mms-type", string(t))
	query.Set("__wa-mms", "")
	target.RawQuery = query.Encode()
	return target.String(), nil
}

func UploadURL(hostname string, t Type, fileEncSHA256 []byte, auth string, mediaID uint64) string {
	hash := strings.NewReplacer("+", "-", "/", "_").Replace(base64.StdEncoding.EncodeToString(fileEncSHA256))
	query := "auth=" + formEncode(auth) + "&token=" + formEncode(hash) + "&media_id=" + strconv.FormatUint(mediaID, 10)
	return "https://" + hostname + "/mms/" + string(t) + "/" + hash + "?" + query
}

func formEncode(s string) string {
	var out strings.Builder
	for _, b := range []byte(s) {
		switch {
		case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9', b == '*', b == '-', b == '.', b == '_':
			out.WriteByte(b)
		case b == ' ':
			out.WriteByte('+')
		default:
			out.WriteString(fmt.Sprintf("%%%02X", b))
		}
	}
	return out.String()
}
