package media_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/PeterStoica/chatwire/internal/media"
)

func key(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, media.KeySize)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

func TestRoundTripWithHashes(t *testing.T) {
	mediaKey := key(t)
	for _, size := range []int{0, 1, 16, 1000} {
		plaintext := bytes.Repeat([]byte{7}, size)
		sealed, err := media.Encrypt(mediaKey, media.Document, plaintext)
		if err != nil {
			t.Fatal(err)
		}
		if sealed.FileLength != size || sealed.FileSHA256 != sha256.Sum256(plaintext) || sealed.FileEncSHA256 != sha256.Sum256(sealed.File) || len(sealed.File) != (size/16+1)*16+10 {
			t.Fatalf("size %d: %+v", size, sealed)
		}
		got, err := media.Decrypt(mediaKey, media.Document, sealed.File, sealed.FileEncSHA256[:], sealed.FileSHA256[:])
		if err != nil || !bytes.Equal(got, plaintext) {
			t.Fatalf("size %d: Decrypt() = %x, %v", size, got, err)
		}
	}
}

func TestTypesShareKeysLikeWhatsApp(t *testing.T) {
	mediaKey := key(t)
	derive := func(kind media.Type) media.Keys {
		t.Helper()
		keys, err := media.Derive(mediaKey, kind)
		if err != nil {
			t.Fatal(err)
		}
		return keys
	}
	for _, same := range [][]media.Type{{media.Image, media.Sticker, "xma-image"}, {media.Video, media.GIF}, {media.Audio, media.Voice}} {
		for _, kind := range same[1:] {
			if derive(kind) != derive(same[0]) {
				t.Errorf("%s and %s should share keys", kind, same[0])
			}
		}
	}
	distinct := map[media.Keys]media.Type{}
	for _, kind := range []media.Type{media.Document, media.Image, media.Video, media.Audio, media.AppState, media.History} {
		k := derive(kind)
		if other, ok := distinct[k]; ok {
			t.Errorf("%s and %s share keys", kind, other)
		}
		distinct[k] = kind
	}
	if _, err := media.Derive(mediaKey, "hologram"); !errors.Is(err, media.ErrType) {
		t.Fatalf("unknown type: %v", err)
	}
	for _, size := range []int{0, 31, 33} {
		if _, err := media.Derive(make([]byte, size), media.Image); !errors.Is(err, media.ErrKey) {
			t.Errorf("%d-byte key: %v", size, err)
		}
		if _, err := media.Encrypt(make([]byte, size), media.Image, nil); !errors.Is(err, media.ErrKey) {
			t.Errorf("Encrypt with a %d-byte key: %v", size, err)
		}
	}
}

func TestDecryptRefusals(t *testing.T) {
	mediaKey := key(t)
	sealed, err := media.Encrypt(mediaKey, media.Image, []byte("a picture"))
	if err != nil {
		t.Fatal(err)
	}
	keys, err := media.Derive(mediaKey, media.Image)
	if err != nil {
		t.Fatal(err)
	}
	signed := func(ciphertext []byte) []byte {
		mac := hmac.New(sha256.New, keys.MAC[:])
		mac.Write(keys.IV[:])
		mac.Write(ciphertext)
		return append(bytes.Clone(ciphertext), mac.Sum(nil)[:10]...)
	}
	badPadding := make([]byte, 16)
	block, err := aes.NewCipher(keys.Cipher[:])
	if err != nil {
		t.Fatal(err)
	}
	cipher.NewCBCEncrypter(block, keys.IV[:]).CryptBlocks(badPadding, bytes.Repeat([]byte{0}, 16))
	flipped := bytes.Clone(sealed.File)
	flipped[0] ^= 1
	for _, tt := range []struct {
		name       string
		kind       media.Type
		file       []byte
		enc, plain []byte
		want       error
	}{
		{"wrong type", media.Video, sealed.File, nil, nil, media.ErrMAC},
		{"flipped byte", media.Image, flipped, nil, nil, media.ErrMAC},
		{"only a mac", media.Image, sealed.File[len(sealed.File)-10:], nil, nil, media.ErrTooShort},
		{"not whole blocks", media.Image, append(bytes.Clone(sealed.File), 1), nil, nil, media.ErrTooShort},
		{"authentic but badly padded", media.Image, signed(badPadding), nil, nil, media.ErrPadding},
		{"encrypted hash mismatch", media.Image, sealed.File, make([]byte, 32), nil, media.ErrHash},
		{"plaintext hash mismatch", media.Image, sealed.File, nil, make([]byte, 32), media.ErrHash},
		{"unknown type", "hologram", sealed.File, nil, nil, media.ErrType},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := media.Decrypt(mediaKey, tt.kind, tt.file, tt.enc, tt.plain); !errors.Is(err, tt.want) {
				t.Fatalf("Decrypt() = %v, want %v", err, tt.want)
			}
		})
	}
	oversized := bytes.Repeat([]byte{17}, 16)
	cipher.NewCBCEncrypter(block, keys.IV[:]).CryptBlocks(oversized, oversized)
	if _, err := media.Decrypt(mediaKey, media.Image, signed(oversized), nil, nil); !errors.Is(err, media.ErrPadding) {
		t.Fatalf("a pad of 17: %v", err)
	}
	lying := bytes.Repeat([]byte{2}, 16)
	lying[14] = 3
	cipher.NewCBCEncrypter(block, keys.IV[:]).CryptBlocks(lying, lying)
	if _, err := media.Decrypt(mediaKey, media.Image, signed(lying), nil, nil); !errors.Is(err, media.ErrPadding) {
		t.Fatalf("inconsistent pad bytes: %v", err)
	}
}

func TestDescribingPictures(t *testing.T) {
	t.Parallel()
	encode := func(t *testing.T, format string, w, h int) []byte {
		t.Helper()
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := range h {
			for x := range w {
				img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 200, A: 255})
			}
		}
		var buf bytes.Buffer
		var err error
		switch format {
		case "png":
			err = png.Encode(&buf, img)
		case "gif":
			err = gif.Encode(&buf, img, nil)
		default:
			err = jpeg.Encode(&buf, img, nil)
		}
		if err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	tests := []struct {
		name                 string
		format               string
		w, h, thumbW, thumbH int
	}{
		{name: "landscape jpeg", format: "jpeg", w: 400, h: 200, thumbW: 72, thumbH: 36},
		{name: "portrait png", format: "png", w: 100, h: 300, thumbW: 24, thumbH: 72},
		{name: "small gif kept at size", format: "gif", w: 50, h: 40, thumbW: 50, thumbH: 40},
		{name: "exactly the side", format: "png", w: 72, h: 10, thumbW: 72, thumbH: 10},
		{name: "one pixel wide strip", format: "png", w: 1000, h: 1, thumbW: 72, thumbH: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pic, ok := media.Describe(encode(t, tt.format, tt.w, tt.h))
			if !ok || int(pic.Width) != tt.w || int(pic.Height) != tt.h {
				t.Fatalf("Describe() = %dx%d, %v", pic.Width, pic.Height, ok)
			}
			thumb, err := jpeg.Decode(bytes.NewReader(pic.Thumbnail))
			if err != nil {
				t.Fatal(err)
			}
			if b := thumb.Bounds(); b.Dx() != tt.thumbW || b.Dy() != tt.thumbH {
				t.Fatalf("thumbnail %dx%d, want %dx%d", b.Dx(), b.Dy(), tt.thumbW, tt.thumbH)
			}
		})
	}
	for _, data := range [][]byte{nil, []byte("not a picture"), []byte("\xff\xd8\xff broken jpeg"), []byte("GIF89a")} {
		if _, ok := media.Describe(data); ok {
			t.Errorf("Describe(%q) succeeded", data)
		}
	}
}

func TestSniffingFileTypes(t *testing.T) {
	t.Parallel()
	png8 := []byte("\x89PNG\r\n\x1a\n0000")
	mp4 := append([]byte{0, 0, 0, 0x18}, []byte("ftypmp42\x00\x00\x00\x00isommp42")...)
	tests := []struct {
		name, file string
		data       []byte
		mimetype   string
		kind       media.Type
	}{
		{name: "png by content whatever the name", file: "x.pdf", data: png8, mimetype: "image/png", kind: media.Image},
		{name: "jpeg", file: "a", data: []byte("\xff\xd8\xff\xe0 jfif"), mimetype: "image/jpeg", kind: media.Image},
		{name: "gif", file: "a", data: []byte("GIF89a...."), mimetype: "image/gif", kind: media.Image},
		{name: "webp", file: "a", data: []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), mimetype: "image/webp", kind: media.Image},
		{name: "mp4 video", file: "clip", data: mp4, mimetype: "video/mp4", kind: media.Video},
		{name: "mp4 by name", file: "clip.mp4", data: []byte{1, 2, 3}, mimetype: "video/mp4", kind: media.Video},
		{name: "pdf by content", file: "report", data: []byte("%PDF-1.7 ..."), mimetype: "application/pdf", kind: media.Document},
		{name: "pdf by name", file: "report.PDF", data: []byte{0, 1, 2}, mimetype: "application/pdf", kind: media.Document},
		{name: "docx is a zip", file: "cv.docx", data: []byte("PK\x03\x04...."), mimetype: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", kind: media.Document},
		{name: "xlsx", file: "a.xlsx", data: []byte("PK\x03\x04...."), mimetype: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", kind: media.Document},
		{name: "pptx", file: "a.pptx", data: []byte("PK\x03\x04...."), mimetype: "application/vnd.openxmlformats-officedocument.presentationml.presentation", kind: media.Document},
		{name: "old word", file: "a.doc", data: []byte{0xd0, 0xcf, 0x11}, mimetype: "application/msword", kind: media.Document},
		{name: "old excel", file: "a.xls", data: []byte{0xd0, 0xcf, 0x11}, mimetype: "application/vnd.ms-excel", kind: media.Document},
		{name: "plain zip", file: "a.zip", data: []byte("PK\x03\x04...."), mimetype: "application/zip", kind: media.Document},
		{name: "unnamed zip", file: "a", data: []byte("PK\x03\x04...."), mimetype: "application/zip", kind: media.Document},
		{name: "csv", file: "a.csv", data: []byte("a,b\n1,2\n"), mimetype: "text/csv", kind: media.Document},
		{name: "markdown", file: "notes.md", data: []byte("# hi\n"), mimetype: "text/plain", kind: media.Document},
		{name: "text without a name", file: "notes", data: []byte("hello"), mimetype: "text/plain", kind: media.Document},
		{name: "ogg voice note", file: "note", data: []byte("OggS\x00\x02"), mimetype: media.VoiceMimetype, kind: media.Voice},
		{name: "opus by name", file: "note.opus", data: []byte{1, 2, 3}, mimetype: media.VoiceMimetype, kind: media.Voice},
		{name: "m4a", file: "song.m4a", data: []byte{1, 2, 3}, mimetype: "audio/mp4", kind: media.Audio},
		{name: "mp3 by content", file: "song", data: []byte("ID3\x03\x00\x00"), mimetype: "audio/mpeg", kind: media.Audio},
		{name: "mp3 by name", file: "song.mp3", data: []byte{1, 2, 3}, mimetype: "audio/mpeg", kind: media.Audio},
		{name: "heic stays a document", file: "IMG.HEIC", data: []byte{1, 2, 3}, mimetype: "image/heic", kind: media.Document},
		{name: "mov stays a document", file: "a.mov", data: []byte{1, 2, 3}, mimetype: "video/quicktime", kind: media.Document},
		{name: "xml", file: "a", data: []byte("<?xml version=\"1.0\"?><a/>"), mimetype: "text/xml", kind: media.Document},
		{name: "unknown bytes", file: "blob.bin", data: []byte{0, 1, 2, 3}, mimetype: "application/octet-stream", kind: media.Document},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := media.Sniff(tt.file, tt.data); got != tt.mimetype {
				t.Fatalf("Sniff() = %q, want %q", got, tt.mimetype)
			}
			if got := media.KindOf(tt.mimetype); got != tt.kind {
				t.Fatalf("KindOf(%q) = %q, want %q", tt.mimetype, got, tt.kind)
			}
		})
	}
	if media.KindOf("video/3gpp") != media.Video || media.KindOf("audio/ogg") != media.Audio {
		t.Fatal("3gpp is a video, and only opus in ogg is a voice note")
	}
}

func TestThumbnailsKeepThePicture(t *testing.T) {
	t.Parallel()
	quadrants := []color.RGBA{{R: 255, A: 255}, {B: 255, A: 255}, {G: 255, A: 255}, {R: 255, G: 255, B: 255, A: 255}}
	paint := func(img *image.Paletted, r image.Rectangle) {
		midX, midY := r.Min.X+r.Dx()/2, r.Min.Y+r.Dy()/2
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				q := 0
				if x >= midX {
					q++
				}
				if y >= midY {
					q += 2
				}
				img.Set(x, y, quadrants[q])
			}
		}
	}
	palette := color.Palette{quadrants[0], quadrants[1], quadrants[2], quadrants[3]}
	near := func(got color.Color, want color.RGBA) bool {
		r, g, b, _ := got.RGBA()
		within := func(a uint32, b uint8) bool {
			return a>>8 >= uint32(b)-60 && a>>8 <= uint32(b)+60 || b == 0 && a>>8 <= 60
		}
		return within(r, want.R) && within(g, want.G) && within(b, want.B)
	}
	check := func(t *testing.T, data []byte, w, h int) {
		t.Helper()
		pic, ok := media.Describe(data)
		if !ok || int(pic.Width) != w || int(pic.Height) != h {
			t.Fatalf("Describe() = %dx%d, %v", pic.Width, pic.Height, ok)
		}
		thumb, err := jpeg.Decode(bytes.NewReader(pic.Thumbnail))
		if err != nil {
			t.Fatal(err)
		}
		b := thumb.Bounds()
		for i, at := range []image.Point{{1, 1}, {b.Dx() - 2, 1}, {1, b.Dy() - 2}, {b.Dx() - 2, b.Dy() - 2}} {
			if got := thumb.At(at.X, at.Y); !near(got, quadrants[i]) {
				t.Fatalf("corner %d of a %dx%d thumbnail is %v, want %v", i, b.Dx(), b.Dy(), got, quadrants[i])
			}
		}
	}
	whole := image.NewPaletted(image.Rect(0, 0, 400, 200), palette)
	paint(whole, whole.Bounds())
	var png8 bytes.Buffer
	if err := png.Encode(&png8, whole); err != nil {
		t.Fatal(err)
	}
	check(t, png8.Bytes(), 400, 200)
	frame := image.NewPaletted(image.Rect(30, 20, 230, 120), palette)
	paint(frame, frame.Bounds())
	var animated bytes.Buffer
	if err := gif.EncodeAll(&animated, &gif.GIF{Image: []*image.Paletted{frame}, Delay: []int{0}, Config: image.Config{Width: 300, Height: 200, ColorModel: palette}}); err != nil {
		t.Fatal(err)
	}
	check(t, animated.Bytes(), 200, 100)
}
