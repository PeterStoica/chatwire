package media

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
)

const (
	thumbnailSide    = 72
	thumbnailQuality = 60
)

type Picture struct {
	Width     uint32
	Height    uint32
	Thumbnail []byte
}

func decodeImage(data []byte) (image.Image, bool) {
	var (
		img image.Image
		err error
	)
	switch r := bytes.NewReader(data); {
	case bytes.HasPrefix(data, []byte("\xff\xd8\xff")):
		img, err = jpeg.Decode(r)
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		img, err = png.Decode(r)
	case bytes.HasPrefix(data, []byte("GIF8")):
		img, err = gif.Decode(r)
	default:
		return nil, false
	}
	return img, err == nil
}

func Describe(data []byte) (Picture, bool) {
	img, ok := decodeImage(data)
	if !ok {
		return Picture{}, false
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	longest := max(width, height, 1)
	thumbWidth, thumbHeight := max(1, width*min(longest, thumbnailSide)/longest), max(1, height*min(longest, thumbnailSide)/longest)
	thumb := image.NewRGBA(image.Rect(0, 0, thumbWidth, thumbHeight))
	for y := range thumbHeight {
		for x := range thumbWidth {
			thumb.Set(x, y, img.At(bounds.Min.X+x*width/thumbWidth, bounds.Min.Y+y*height/thumbHeight))
		}
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, thumb, &jpeg.Options{Quality: thumbnailQuality}); err != nil {
		return Picture{}, false
	}
	return Picture{Width: uint32(width & math.MaxUint32), Height: uint32(height & math.MaxUint32), Thumbnail: out.Bytes()}, true
}
