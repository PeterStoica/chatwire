package media

import (
	"net/http"
	"path/filepath"
	"strings"
)

const VoiceMimetype = "audio/ogg; codecs=opus"

func Sniff(name string, data []byte) string {
	sniffed := strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0])
	switch sniffed {
	case "application/octet-stream", "application/zip", "text/plain", "text/xml", "application/ogg":
	default:
		return sniffed
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".doc":
		return "application/msword"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".csv":
		return "text/csv"
	case ".txt", ".md":
		return "text/plain"
	case ".zip":
		return "application/zip"
	case ".opus", ".ogg", ".oga":
		return VoiceMimetype
	case ".m4a":
		return "audio/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".heic":
		return "image/heic"
	case ".mov":
		return "video/quicktime"
	case ".mp4":
		return "video/mp4"
	}
	if sniffed == "application/ogg" {
		return VoiceMimetype
	}
	return sniffed
}

func KindOf(mimetype string) Type {
	base := strings.TrimSpace(strings.Split(mimetype, ";")[0])
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
