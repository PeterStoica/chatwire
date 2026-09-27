package media_test

import (
	"bytes"
	"image"
	"image/jpeg"
	"slices"
	"testing"

	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/testkit/fakemedia"
)

func FuzzOggOpus(f *testing.F) {
	f.Add(fakemedia.VoiceNote(3))
	f.Add([]byte("OggS"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if voice, ok := media.OggOpus(data); ok && len(voice.Waveform) != 64 {
			t.Fatalf("waveform of %d points", len(voice.Waveform))
		}
	})
}

func FuzzMP4(f *testing.F) {
	f.Add(fakemedia.Video(5, 640, 480))
	f.Add([]byte{0, 0, 0, 1, 'm', 'o', 'o', 'v'})
	f.Fuzz(func(t *testing.T, data []byte) {
		media.MP4(data)
	})
}

func headerMarkers(jpeg []byte) []byte {
	var markers []byte
	rest := jpeg[2:]
	for len(rest) >= 4 && rest[0] == 0xFF && rest[1] != 0xDA && rest[1] != 0xD9 {
		markers = append(markers, rest[1])
		next := 2 + (int(rest[2])<<8 | int(rest[3]))
		if next > len(rest) {
			break
		}
		rest = rest[next:]
	}
	return markers
}

func FuzzPreparingPhotos(f *testing.F) {
	var photo bytes.Buffer
	if err := jpeg.Encode(&photo, image.NewGray(image.Rect(0, 0, 4, 4)), nil); err != nil {
		f.Fatal(err)
	}
	f.Add(photo.Bytes())
	f.Add(withExif(photo.Bytes(), 6))
	f.Fuzz(func(t *testing.T, data []byte) {
		if clean, ok := media.CleanJPEG(data); ok {
			if kept := headerMarkers(clean); len(kept) == 0 || kept[0] != 0xE0 || slices.ContainsFunc(kept[1:], func(m byte) bool { return m >= 0xE0 && m <= 0xEF || m == 0xFE }) {
				t.Fatalf("metadata survived cleaning: markers %x", kept)
			}
		}
		media.Prepare("image/jpeg", data)
	})
}
