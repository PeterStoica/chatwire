package mcptools

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/messenger"
)

type MediaInput struct {
	MessageID string `json:"message_id" jsonschema:"the id of a message from read_whatsapp_messages that has media"`
}

type MediaReport struct {
	State    string `json:"state"`
	Type     string `json:"type,omitempty"`
	Mimetype string `json:"mimetype,omitempty"`
	Caption  string `json:"caption,omitempty"`
	FileName string `json:"file_name,omitempty"`
	Path     string `json:"path,omitempty"`
	Size     int    `json:"size,omitempty"`
	Detail   string `json:"detail"`
}

const (
	maxInlineImage     = 4 << 20
	maxNameLength      = 64
	maxExtensionLength = 8
)

func getMedia(s Sender, opts Options) mcp.ToolHandlerFor[MediaInput, MediaReport] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in MediaInput) (*mcp.CallToolResult, MediaReport, error) {
		if _, linked := s.Self(); !linked {
			return mediaReport(MediaReport{State: stateNotLinked, Detail: notLinked})
		}
		fetchCtx, cancel := context.WithTimeout(ctx, sendTimeout)
		defer cancel()
		ref, data, err := s.Media(fetchCtx, strings.TrimSpace(in.MessageID))
		switch {
		case errors.Is(err, messenger.ErrUnknownMessage):
			return mediaReport(MediaReport{State: stateUnknownMessage, Detail: noSuchMessage})
		case errors.Is(err, messenger.ErrNoMedia):
			return mediaReport(MediaReport{State: "no_media", Detail: "That message has no photo, video, voice note or document."})
		case err != nil:
			return mediaReport(MediaReport{State: stateFailed, Detail: fmt.Sprintf("Could not download it: %v", err)})
		}
		ref.Mimetype = mimetypeOf(ref)
		report := MediaReport{State: "ok", Type: string(ref.Type), Mimetype: ref.Mimetype, Caption: ref.Caption, FileName: ref.FileName, Size: len(data)}
		if strings.HasPrefix(ref.Mimetype, "image/") && len(data) <= maxInlineImage {
			report.Detail = describeMedia(report)
			return withJSON(report, &mcp.ImageContent{MIMEType: ref.Mimetype, Data: data}), report, nil
		}
		path, err := saveMedia(opts.MediaDir, in.MessageID, ref, data)
		if err != nil {
			return mediaReport(MediaReport{State: stateFailed, Detail: fmt.Sprintf("Downloaded, but could not save it: %v", err)})
		}
		report.Path = path
		report.Detail = describeMedia(report) + " Saved to " + path + "."
		return mediaReport(report)
	}
}

func mediaReport(report MediaReport) (*mcp.CallToolResult, MediaReport, error) {
	return nil, report, nil
}

func mimetypeOf(ref media.Reference) string {
	given := media.Essence(ref.Mimetype)
	if given != "" && given != "application/octet-stream" {
		return ref.Mimetype
	}
	if known, ok := media.ByExtension(ref.FileName); ok {
		return known
	}
	if guessed := mime.TypeByExtension(filepath.Ext(ref.FileName)); guessed != "" {
		return guessed
	}
	return cmp.Or(ref.Mimetype, "application/octet-stream")
}

func describeMedia(r MediaReport) string {
	out := fmt.Sprintf("%s, %s, %d bytes.", r.Type, r.Mimetype, r.Size)
	if r.FileName != "" {
		out = fmt.Sprintf("%s %q, %s, %d bytes.", r.Type, r.FileName, r.Mimetype, r.Size)
	}
	if r.Caption != "" {
		out += " Caption: " + r.Caption
	}
	return out
}

func saveMedia(dir, id string, ref media.Reference, data []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	name := safeName(id, maxNameLength)
	if ref.FileName != "" {
		base := filepath.Base(ref.FileName)
		if stem := safeName(strings.TrimSuffix(base, filepath.Ext(base)), maxNameLength); stem != "" && strings.Trim(stem, "_") != "" {
			name += "-" + stem
		}
	}
	if extension := safeName(extensionOf(ref), maxExtensionLength); extension != "" {
		name += "." + extension
	}
	path := filepath.Join(dir, name)
	return path, os.WriteFile(path, data, 0o600)
}

func extensionOf(ref media.Reference) string {
	if extension := strings.TrimPrefix(filepath.Ext(ref.FileName), "."); extension != "" {
		return extension
	}
	switch media.Essence(ref.Mimetype) {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	case "video/mp4":
		return "mp4"
	case "audio/ogg":
		return "ogg"
	case "audio/mpeg":
		return "mp3"
	case "audio/mp4":
		return "m4a"
	case "audio/aac":
		return "aac"
	case "audio/amr":
		return "amr"
	case "video/3gpp":
		return "3gp"
	case "application/pdf":
		return "pdf"
	}
	return ""
}

func safeName(s string, limit int) string {
	safe := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, s)
	return safe[:min(len(safe), limit)]
}
