package media

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
)

const (
	jpegQuality     = 92
	markerSOF0      = 0xC0
	markerSOF15     = 0xCF
	markerDQT       = 0xDB
	markerDRI       = 0xDD
	markerSOI       = 0xD8
	markerEOI       = 0xD9
	markerSOS       = 0xDA
	markerAPP0      = 0xE0
	markerAPP1      = 0xE1
	markerAPP15     = 0xEF
	markerCOM       = 0xFE
	orientationTag  = 0x0112
	uprightRotation = 1
)

var (
	jfif       = []byte{0xFF, markerAPP0, 0, 16, 'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0}
	background = color.RGBA{R: 247, G: 247, B: 247, A: 255}
)

func Prepare(mimetype string, data []byte) (string, []byte, Type) {
	switch Essence(mimetype) {
	case "image/jpeg":
		if orientation(data) != uprightRotation {
			if picture, err := jpeg.Decode(bytes.NewReader(data)); err == nil {
				if out, ok := encodeJPEG(oriented(picture, orientation(data))); ok {
					return "image/jpeg", out, Image
				}
			}
		}
		if clean, ok := CleanJPEG(data); ok {
			return "image/jpeg", clean, Image
		}
		if picture, err := jpeg.Decode(bytes.NewReader(data)); err == nil {
			if out, ok := encodeJPEG(picture); ok {
				return "image/jpeg", out, Image
			}
		}
		return mimetype, data, Document
	case "image/png":
		if picture, err := png.Decode(bytes.NewReader(data)); err == nil {
			if out, ok := encodeJPEG(picture); ok {
				return "image/jpeg", out, Image
			}
		}
		return mimetype, data, Document
	}
	return mimetype, data, SentAs(mimetype)
}

func SentAs(mimetype string) Type {
	switch Essence(mimetype) {
	case "image/gif", "image/webp":
		return Document
	}
	return KindOf(mimetype)
}

func encodeJPEG(picture image.Image) ([]byte, bool) {
	bounds := picture.Bounds()
	canvas := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: background}, image.Point{}, draw.Src)
	draw.Draw(canvas, canvas.Bounds(), picture, bounds.Min, draw.Over)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, canvas, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, false
	}
	return out.Bytes(), true
}

func CleanJPEG(data []byte) ([]byte, bool) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != markerSOI {
		return nil, false
	}
	out := append([]byte{0xFF, markerSOI}, jfif...)
	rest := data[2:]
	for len(rest) >= 4 {
		if rest[0] != 0xFF {
			return nil, false
		}
		marker := rest[1]
		if marker == 0xFF {
			rest = rest[1:]
			continue
		}
		if marker == markerEOI {
			return append(out, 0xFF, markerEOI), true
		}
		size := int(binary.BigEndian.Uint16(rest[2:4]))
		if size < 2 || 2+size > len(rest) {
			return nil, false
		}
		if marker == markerSOS {
			return append(out, rest...), true
		}
		switch {
		case marker >= markerSOF0 && marker <= markerSOF15, marker == markerDQT, marker == markerDRI:
			out = append(out, rest[:2+size]...)
		case marker == markerAPP0, marker >= markerAPP1 && marker <= markerAPP15, marker == markerCOM:
		default:
			return nil, false
		}
		rest = rest[2+size:]
	}
	return nil, false
}

func orientation(data []byte) int {
	rest := data[min(2, len(data)):]
	for len(rest) >= 4 && rest[0] == 0xFF && rest[1] != markerSOS {
		size := int(binary.BigEndian.Uint16(rest[2:4]))
		if size < 2 || 2+size > len(rest) {
			break
		}
		if segment := rest[4 : 2+size]; rest[1] == markerAPP1 && bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			return exifOrientation(segment[6:])
		}
		rest = rest[2+size:]
	}
	return uprightRotation
}

func exifOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return uprightRotation
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return uprightRotation
	}
	ifd := int(order.Uint32(tiff[4:8]))
	if ifd+2 > len(tiff) {
		return uprightRotation
	}
	entries := int(order.Uint16(tiff[ifd:]))
	for i := range entries {
		at := ifd + 2 + i*12
		if at+12 > len(tiff) {
			break
		}
		if order.Uint16(tiff[at:]) == orientationTag {
			if value := int(order.Uint16(tiff[at+8:])); value >= 1 && value <= 8 {
				return value
			}
		}
	}
	return uprightRotation
}

func oriented(picture image.Image, rotation int) image.Image {
	bounds := picture.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	outW, outH := w, h
	if rotation >= 5 {
		outW, outH = h, w
	}
	out := image.NewRGBA(image.Rect(0, 0, outW, outH))
	for y := range outH {
		for x := range outW {
			var sx, sy int
			switch rotation {
			case 2:
				sx, sy = w-1-x, y
			case 3:
				sx, sy = w-1-x, h-1-y
			case 4:
				sx, sy = x, h-1-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, h-1-x
			case 7:
				sx, sy = w-1-y, h-1-x
			case 8:
				sx, sy = w-1-y, x
			default:
				sx, sy = x, y
			}
			out.Set(x, y, picture.At(bounds.Min.X+sx, bounds.Min.Y+sy))
		}
	}
	return out
}
