package media

import (
	"net/http"
	"path/filepath"
	"strings"
)

const VoiceMimetype = "audio/ogg; codecs=opus"

func Essence(mimetype string) string {
	base, _, _ := strings.Cut(mimetype, ";")
	return strings.TrimSpace(base)
}

func Sniff(name string, data []byte) string {
	sniffed := Essence(http.DetectContentType(data))
	switch sniffed {
	case "application/octet-stream", "application/zip", "text/plain", "text/xml", "application/ogg":
	default:
		return sniffed
	}
	if known, ok := ByExtension(name); ok {
		return known
	}
	if sniffed == "application/ogg" {
		return VoiceMimetype
	}
	return sniffed
}

func ByExtension(name string) (string, bool) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		return "application/pdf", true
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document", true
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", true
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation", true
	case ".doc":
		return "application/msword", true
	case ".xls":
		return "application/vnd.ms-excel", true
	case ".csv":
		return "text/csv", true
	case ".txt", ".md":
		return "text/plain", true
	case ".zip":
		return "application/zip", true
	case ".opus", ".ogg", ".oga":
		return VoiceMimetype, true
	case ".m4a":
		return "audio/mp4", true
	case ".mp3":
		return "audio/mpeg", true
	case ".heic":
		return "image/heic", true
	case ".mov":
		return "video/quicktime", true
	case ".mp4":
		return "video/mp4", true
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".png":
		return "image/png", true
	case ".webp":
		return "image/webp", true
	case ".gif":
		return "image/gif", true
	}
	return "", false
}

func KindOf(mimetype string) Type {
	base := Essence(mimetype)
	switch {
	case base == "image/jpeg", base == "image/png", base == "image/webp", base == "image/gif":
		return Image
	case base == "video/mp4", base == "video/3gpp":
		return Video
	case mimetype == VoiceMimetype:
		return Voice
	case strings.HasPrefix(base, "audio/"):
		return Audio
	default:
		return Document
	}
}
