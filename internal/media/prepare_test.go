package media_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"slices"
	"testing"

	"github.com/PeterStoica/chatwire/internal/media"
)

func encoded(t *testing.T, width, height int, encode func(*bytes.Buffer, image.Image) error) []byte {
	t.Helper()
	picture := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			picture.Set(x, y, color.NRGBA{R: uint8(x * 255 / width), G: 40, B: uint8(y * 255 / height), A: 255})
		}
	}
	for y := range min(8, height) {
		for x := range min(8, width) {
			picture.Set(x, y, color.NRGBA{})
		}
	}
	var out bytes.Buffer
	if err := encode(&out, picture); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func withExif(photo []byte, orientation byte) []byte {
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, orientation, 0, 0, 0, 0, 0, 0}
	payload := slices.Concat([]byte("Exif\x00\x00"), tiff, []byte("GPS 44.43N 26.10E"))
	size := len(payload) + 2
	app1 := slices.Concat([]byte{0xFF, 0xE1, byte(size >> 8), byte(size)}, payload)
	return slices.Concat(photo[:2], app1, photo[2:])
}

func decode(t *testing.T, data []byte) image.Image {
	t.Helper()
	picture, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return picture
}

func TestPhotosLeaveWithoutTheirMetadata(t *testing.T) {
	t.Parallel()
	photo := encoded(t, 40, 20, func(b *bytes.Buffer, m image.Image) error { return jpeg.Encode(b, m, nil) })
	tagged := withExif(photo, 1)
	mimetype, out, kind := media.Prepare("image/jpeg", tagged)
	if mimetype != "image/jpeg" || kind != media.Image || bytes.Contains(out, []byte("GPS")) || bytes.Contains(out, []byte("Exif")) {
		t.Fatalf("Prepare() kept metadata: %s %v", mimetype, kind)
	}
	if !bytes.Equal(decode(t, out).(*image.YCbCr).Y, decode(t, photo).(*image.YCbCr).Y) {
		t.Fatal("cleaning changed the picture")
	}
	sideways := withExif(photo, 6)
	_, out, _ = media.Prepare("image/jpeg", sideways)
	upright := decode(t, out)
	if bounds := upright.Bounds(); bounds.Dx() != 20 || bounds.Dy() != 40 || bytes.Contains(out, []byte("GPS")) {
		t.Fatalf("a photo taken sideways came out %dx%d", bounds.Dx(), bounds.Dy())
	}
	dark := func(x, y int) bool {
		r, g, b, _ := upright.At(x, y).RGBA()
		return r>>8 < 40 && g>>8 < 40 && b>>8 < 40
	}
	if !dark(16, 3) || dark(3, 3) {
		t.Fatal("the photo was turned the wrong way: the top-left corner should end up top-right")
	}
}

func TestPicturesAreSentTheWayWhatsAppShowsThem(t *testing.T) {
	t.Parallel()
	screenshot := encoded(t, 30, 10, func(b *bytes.Buffer, m image.Image) error { return png.Encode(b, m) })
	mimetype, out, kind := media.Prepare("image/png", screenshot)
	if mimetype != "image/jpeg" || kind != media.Image {
		t.Fatalf("a PNG went out as %s %v", mimetype, kind)
	}
	if r, g, b, _ := decode(t, out).At(3, 3).RGBA(); r>>8 < 240 || g>>8 < 240 || b>>8 < 240 || r>>8 > 252 {
		t.Fatalf("a transparent pixel became %d,%d,%d instead of the light gray background", r>>8, g>>8, b>>8)
	}
	for _, mimetype := range []string{"image/gif", "image/webp"} {
		if got, data, kind := media.Prepare(mimetype, []byte("animated")); got != mimetype || kind != media.Document || string(data) != "animated" {
			t.Errorf("%s went out as %s %v", mimetype, got, kind)
		}
	}
	if _, _, kind := media.Prepare("image/png", []byte("not a png")); kind != media.Document {
		t.Errorf("a broken PNG went out as %v", kind)
	}
	if got, data, kind := media.Prepare("application/pdf", []byte("%PDF")); got != "application/pdf" || kind != media.Document || string(data) != "%PDF" {
		t.Errorf("a PDF went out as %s %v", got, kind)
	}
}
