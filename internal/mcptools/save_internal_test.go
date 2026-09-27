package mcptools

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PeterStoica/chatwire/internal/media"
)

func TestSavedMediaStaysInsideItsFolder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		id       string
		ref      media.Reference
		wantFile string
	}{
		{name: "document keeps its extension", id: "3EB0A1", ref: media.Reference{FileName: "report.pdf"}, wantFile: "3EB0A1-report.pdf"},
		{name: "every character class boundary", id: "az`{AZ@[09/:-_", wantFile: "az__AZ__09__-_"},
		{name: "traversal in the id", id: "../../etc/passwd", ref: media.Reference{Mimetype: "image/jpeg"}, wantFile: "______etc_passwd.jpg"},
		{name: "traversal in the file name", id: "3EB0A2", ref: media.Reference{FileName: `a/../../evil.sh`}, wantFile: "3EB0A2-evil.sh"},
		{name: "backslash traversal", id: `..\..\x`, ref: media.Reference{FileName: `..\..\run.bat`}, wantFile: "______x-______run.bat"},
		{name: "alternate data stream", id: "3EB0A3", ref: media.Reference{FileName: "notes.txt:secret"}, wantFile: "3EB0A3-notes.txt_secr"},
		{name: "voice note from its mimetype", id: "3EB0A4", ref: media.Reference{Mimetype: "audio/ogg; codecs=opus"}, wantFile: "3EB0A4.ogg"},
		{name: "unknown mimetype gets no extension", id: "3EB0A5", ref: media.Reference{Mimetype: "application/x-nothing-known"}, wantFile: "3EB0A5"},
		{name: "a name of only dots", id: "3EB0A6", ref: media.Reference{FileName: "..", Mimetype: "application/pdf"}, wantFile: "3EB0A6.pdf"},
		{name: "non-ASCII becomes one underscore per character", id: "3EB0é漢", ref: media.Reference{FileName: "x.jpég"}, wantFile: "3EB0__-x.jp_g"},
		{name: "long id and extension are cut", id: strings.Repeat("A", 100), ref: media.Reference{FileName: "x.averyveryverylongextension"}, wantFile: strings.Repeat("A", maxNameLength) + "-x.averyver"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			data := []byte(tt.name)
			path, err := saveMedia(dir, tt.id, tt.ref, data)
			if err != nil {
				t.Fatal(err)
			}
			if filepath.Dir(path) != dir || filepath.Base(path) != tt.wantFile {
				t.Fatalf("saved at %q, want %q in %q", path, tt.wantFile, dir)
			}
			if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, data) {
				t.Fatalf("file holds %q, %v", got, err)
			}
		})
	}
}

func TestClippingQuotes(t *testing.T) {
	t.Parallel()
	exactly := strings.Repeat("ă", maxQuoted)
	for _, tt := range []struct{ in, want string }{
		{"", ""},
		{"  two\n  lines  ", "two lines"},
		{exactly, exactly},
		{exactly + "b", exactly + "…"},
	} {
		if got := clip(tt.in, maxQuoted); got != tt.want {
			t.Errorf("clip(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAGenericDocumentTypeFollowsItsName(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		ref  media.Reference
		want string
	}{
		{ref: media.Reference{Mimetype: "application/octet-stream", FileName: "random-08567f.txt"}, want: "text/plain; charset=utf-8"},
		{ref: media.Reference{FileName: "a.pdf"}, want: "application/pdf"},
		{ref: media.Reference{Mimetype: "image/jpeg", FileName: "a.pdf"}, want: "image/jpeg"},
		{ref: media.Reference{Mimetype: "application/octet-stream", FileName: "a.unknownext"}, want: "application/octet-stream"},
		{ref: media.Reference{}, want: "application/octet-stream"},
	} {
		if got := mimetypeOf(tt.ref); got != tt.want {
			t.Errorf("mimetypeOf(%+v) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}
