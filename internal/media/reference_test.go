package media_test

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func TestUnwrappingEveryWrapper(t *testing.T) {
	t.Parallel()
	text := &wire.Message{Conversation: new("hi")}
	wrappers := []struct {
		name string
		wrap func(*wire.Message) *wire.Message
	}{
		{name: "device sent", wrap: func(m *wire.Message) *wire.Message {
			return &wire.Message{DeviceSentMessage: &wire.Message_DeviceSentMessage{Message: m}}
		}},
		{name: "ephemeral", wrap: func(m *wire.Message) *wire.Message {
			return &wire.Message{EphemeralMessage: &wire.Message_FutureProofMessage{Message: m}}
		}},
		{name: "view once", wrap: func(m *wire.Message) *wire.Message {
			return &wire.Message{ViewOnceMessage: &wire.Message_FutureProofMessage{Message: m}}
		}},
		{name: "view once v2", wrap: func(m *wire.Message) *wire.Message {
			return &wire.Message{ViewOnceMessageV2: &wire.Message_FutureProofMessage{Message: m}}
		}},
		{name: "view once v2 extension", wrap: func(m *wire.Message) *wire.Message {
			return &wire.Message{ViewOnceMessageV2Extension: &wire.Message_FutureProofMessage{Message: m}}
		}},
		{name: "document with caption", wrap: func(m *wire.Message) *wire.Message {
			return &wire.Message{DocumentWithCaptionMessage: &wire.Message_FutureProofMessage{Message: m}}
		}},
	}
	for _, w := range wrappers {
		t.Run(w.name, func(t *testing.T) {
			t.Parallel()
			if got := media.Unwrap(w.wrap(text)); got != text {
				t.Fatalf("Unwrap() = %v", got)
			}
			empty := w.wrap(nil)
			if got := media.Unwrap(empty); got != empty {
				t.Fatalf("a wrapper around nothing must come back as itself, got %v", got)
			}
		})
	}
	nest := func(depth int) *wire.Message {
		m := text
		for range depth {
			m = wrappers[1].wrap(m)
		}
		return m
	}
	if got := media.Unwrap(nest(8)); got != text {
		t.Fatalf("eight wrappers must unwrap fully, got %v", got)
	}
	if got := media.Unwrap(nest(9)); got.GetEphemeralMessage() == nil {
		t.Fatalf("unwrapping must stop after eight levels, got %v", got)
	}
	if media.Unwrap(nil) != nil {
		t.Fatal("Unwrap(nil) must be nil")
	}
}

func TestReferencesOfEveryMediaKind(t *testing.T) {
	t.Parallel()
	mediaKey := bytes.Repeat([]byte{7}, media.KeySize)
	path, sum, encSum := new("/v/file.enc"), []byte{1, 2}, []byte{3, 4}
	tests := []struct {
		name string
		msg  *wire.Message
		want media.Reference
	}{
		{
			name: "image",
			msg:  &wire.Message{ImageMessage: &wire.Message_ImageMessage{DirectPath: path, MediaKey: mediaKey, FileSha256: sum, FileEncSha256: encSum, Mimetype: new("image/jpeg"), Caption: new("sea"), FileLength: new(uint64(10))}},
			want: media.Reference{Type: media.Image, Mimetype: "image/jpeg", Caption: "sea", Length: 10},
		},
		{
			name: "video",
			msg:  &wire.Message{VideoMessage: &wire.Message_VideoMessage{DirectPath: path, MediaKey: mediaKey, FileSha256: sum, FileEncSha256: encSum, Mimetype: new("video/mp4"), Caption: new("clip")}},
			want: media.Reference{Type: media.Video, Mimetype: "video/mp4", Caption: "clip"},
		},
		{
			name: "gif",
			msg:  &wire.Message{VideoMessage: &wire.Message_VideoMessage{DirectPath: path, MediaKey: mediaKey, FileSha256: sum, FileEncSha256: encSum, GifPlayback: new(true)}},
			want: media.Reference{Type: media.GIF},
		},
		{
			name: "round video",
			msg:  &wire.Message{PtvMessage: &wire.Message_VideoMessage{DirectPath: path, MediaKey: mediaKey, FileSha256: sum, FileEncSha256: encSum}},
			want: media.Reference{Type: media.Video},
		},
		{
			name: "audio",
			msg:  &wire.Message{AudioMessage: &wire.Message_AudioMessage{DirectPath: path, MediaKey: mediaKey, FileSha256: sum, FileEncSha256: encSum, Mimetype: new("audio/mpeg")}},
			want: media.Reference{Type: media.Audio, Mimetype: "audio/mpeg"},
		},
		{
			name: "voice note",
			msg:  &wire.Message{AudioMessage: &wire.Message_AudioMessage{DirectPath: path, MediaKey: mediaKey, FileSha256: sum, FileEncSha256: encSum, Ptt: new(true)}},
			want: media.Reference{Type: media.Voice},
		},
		{
			name: "document",
			msg:  &wire.Message{DocumentMessage: &wire.Message_DocumentMessage{DirectPath: path, MediaKey: mediaKey, FileSha256: sum, FileEncSha256: encSum, FileName: new("cv.pdf"), Caption: new("my cv")}},
			want: media.Reference{Type: media.Document, FileName: "cv.pdf", Caption: "my cv"},
		},
		{
			name: "document with caption wrapper",
			msg: &wire.Message{DocumentWithCaptionMessage: &wire.Message_FutureProofMessage{Message: &wire.Message{
				DocumentMessage: &wire.Message_DocumentMessage{DirectPath: path, MediaKey: mediaKey, FileSha256: sum, FileEncSha256: encSum, FileName: new("a.txt")},
			}}},
			want: media.Reference{Type: media.Document, FileName: "a.txt"},
		},
		{
			name: "sticker",
			msg:  &wire.Message{StickerMessage: &wire.Message_StickerMessage{DirectPath: path, MediaKey: mediaKey, FileSha256: sum, FileEncSha256: encSum, Mimetype: new("image/webp")}},
			want: media.Reference{Type: media.Sticker, Mimetype: "image/webp"},
		},
		{
			name: "view once image",
			msg: &wire.Message{ViewOnceMessageV2: &wire.Message_FutureProofMessage{Message: &wire.Message{
				ImageMessage: &wire.Message_ImageMessage{DirectPath: path, MediaKey: mediaKey, FileSha256: sum, FileEncSha256: encSum},
			}}},
			want: media.Reference{Type: media.Image},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := media.ReferenceOf(tt.msg)
			want := tt.want
			want.DirectPath, want.MediaKey, want.FileSHA256, want.FileEncSHA256 = *path, mediaKey, sum, encSum
			if !ok || got.Type != want.Type || got.DirectPath != want.DirectPath || !bytes.Equal(got.MediaKey, want.MediaKey) ||
				!bytes.Equal(got.FileSHA256, want.FileSHA256) || !bytes.Equal(got.FileEncSHA256, want.FileEncSHA256) ||
				got.Mimetype != want.Mimetype || got.Caption != want.Caption || got.FileName != want.FileName || got.Length != want.Length {
				t.Fatalf("ReferenceOf() = %+v, %v; want %+v", got, ok, want)
			}
			renewed, ok := media.ReferenceOf(media.WithDirectPath(tt.msg, "/v/renewed.enc"))
			want.DirectPath = "/v/renewed.enc"
			if !ok || renewed.DirectPath != want.DirectPath || renewed.Type != want.Type || !bytes.Equal(renewed.MediaKey, want.MediaKey) {
				t.Fatalf("after WithDirectPath: %+v, %v; want %+v", renewed, ok, want)
			}
			if again, _ := media.ReferenceOf(tt.msg); again.DirectPath != *path {
				t.Fatalf("WithDirectPath changed the original: %q", again.DirectPath)
			}
			hosted := media.Hosted{URL: "https://mmg.example/n", DirectPath: "/v/new.enc", MediaKey: bytes.Repeat([]byte{9}, media.KeySize), FileSHA256: []byte{5}, FileEncSHA256: []byte{6}, FileLength: 77, Stamp: 1790000000}
			rehosted, ok := media.Rehost(tt.msg, hosted)
			moved, found := media.ReferenceOf(rehosted)
			if !ok || !found || moved.DirectPath != hosted.DirectPath || !bytes.Equal(moved.MediaKey, hosted.MediaKey) || !bytes.Equal(moved.FileSHA256, hosted.FileSHA256) ||
				!bytes.Equal(moved.FileEncSHA256, hosted.FileEncSHA256) || moved.Length != hosted.FileLength || moved.Type != want.Type || moved.Mimetype != want.Mimetype ||
				moved.Caption != want.Caption || moved.FileName != want.FileName {
				t.Fatalf("Rehost() = %+v, %v", moved, ok)
			}
			if again, _ := media.ReferenceOf(tt.msg); again.DirectPath != *path || !bytes.Equal(again.MediaKey, mediaKey) {
				t.Fatalf("Rehost changed the original: %+v", again)
			}
		})
	}
}

func TestMessagesWithoutDownloadableMedia(t *testing.T) {
	t.Parallel()
	mediaKey := bytes.Repeat([]byte{7}, media.KeySize)
	tests := []struct {
		name string
		msg  *wire.Message
	}{
		{name: "text", msg: &wire.Message{Conversation: new("hi")}},
		{name: "no direct path", msg: &wire.Message{ImageMessage: &wire.Message_ImageMessage{MediaKey: mediaKey}}},
		{name: "short media key", msg: &wire.Message{ImageMessage: &wire.Message_ImageMessage{DirectPath: new("/v/x"), MediaKey: mediaKey[:31]}}},
		{name: "long media key", msg: &wire.Message{ImageMessage: &wire.Message_ImageMessage{DirectPath: new("/v/x"), MediaKey: append(mediaKey, 0)}}},
		{name: "nil", msg: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got, ok := media.ReferenceOf(tt.msg); ok {
				t.Fatalf("ReferenceOf() = %+v", got)
			}
		})
	}
}

func TestRenewingAPathLeavesMessagesWithoutMediaAlone(t *testing.T) {
	t.Parallel()
	for _, msg := range []*wire.Message{nil, {Conversation: new("hi")}, {ReactionMessage: &wire.Message_ReactionMessage{Text: new("👍")}}} {
		if renewed := media.WithDirectPath(msg, "/v/renewed.enc"); !proto.Equal(renewed, msg) {
			t.Fatalf("WithDirectPath(%v) = %v, want an unchanged copy", msg, renewed)
		}
		if rehosted, ok := media.Rehost(msg, media.Hosted{DirectPath: "/v/x"}); ok || !proto.Equal(rehosted, msg) {
			t.Fatalf("Rehost(%v) = %v, %v; want an unchanged copy", msg, rehosted, ok)
		}
	}
}
