package media_test

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"

	"github.com/PeterStoica/chatwire/internal/media"
)

func oggPage(granule uint64, packets ...[]byte) []byte {
	var table, body []byte
	for _, p := range packets {
		n := len(p)
		for n >= 255 {
			table = append(table, 255)
			n -= 255
		}
		table = append(table, byte(n))
		body = append(body, p...)
	}
	header := make([]byte, 27)
	copy(header, "OggS")
	binary.LittleEndian.PutUint64(header[6:14], granule)
	header[26] = byte(len(table))
	return slices.Concat(header, table, body)
}

func opusHead(preSkip uint16) []byte {
	head := []byte("OpusHead\x01\x01\x00\x00\x80\xbb\x00\x00\x00\x00\x00")
	binary.LittleEndian.PutUint16(head[10:12], preSkip)
	return head
}

func TestVoiceNotesGetTheirLengthAndAWaveform(t *testing.T) {
	t.Parallel()
	var audio [][]byte
	for i := range 200 {
		audio = append(audio, bytes.Repeat([]byte{1}, 20+i%40+(i/100)*300))
	}
	file := slices.Concat(
		oggPage(0, opusHead(312)),
		oggPage(0, []byte("OpusTags")),
		oggPage(48000, audio[:100]...),
		oggPage(312+3*48000+100, audio[100:]...),
	)
	voice, ok := media.OggOpus(file)
	if !ok || voice.Seconds != 4 || len(voice.Waveform) != 64 {
		t.Fatalf("OggOpus() = %d s, %d waveform points, %v", voice.Seconds, len(voice.Waveform), ok)
	}
	if slices.Max(voice.Waveform) != 100 || voice.Waveform[0] >= voice.Waveform[63] {
		t.Fatalf("waveform %v does not rise with the louder second half", voice.Waveform)
	}
	for name, data := range map[string][]byte{
		"vorbis":    oggPage(0, []byte("\x01vorbis")),
		"not ogg":   []byte("ID3 an mp3"),
		"truncated": oggPage(0, opusHead(312))[:30],
		"no audio":  oggPage(0, opusHead(312)),
	} {
		if _, ok := media.OggOpus(data); ok {
			t.Errorf("%s was taken for an Opus voice note", name)
		}
	}
}

func box(kind string, parts ...[]byte) []byte {
	payload := slices.Concat(parts...)
	out := binary.BigEndian.AppendUint32(nil, uint32(8+len(payload)))
	return append(append(out, kind...), payload...)
}

func mvhd(version byte, timescale uint32, duration uint64) []byte {
	if version == 1 {
		out := make([]byte, 32)
		out[0] = 1
		binary.BigEndian.PutUint32(out[20:], timescale)
		binary.BigEndian.PutUint64(out[24:], duration)
		return box("mvhd", out)
	}
	out := make([]byte, 20)
	binary.BigEndian.PutUint32(out[12:], timescale)
	binary.BigEndian.PutUint32(out[16:], uint32(duration))
	return box("mvhd", out)
}

func trak(handler string, width, height uint32, rotated bool) []byte {
	tkhd := make([]byte, 84)
	a, d := uint32(0x10000), uint32(0x10000)
	if rotated {
		a, d = 0, 0
	}
	binary.BigEndian.PutUint32(tkhd[40:], a)
	binary.BigEndian.PutUint32(tkhd[56:], d)
	binary.BigEndian.PutUint32(tkhd[76:], width<<16)
	binary.BigEndian.PutUint32(tkhd[80:], height<<16)
	hdlr := make([]byte, 12)
	copy(hdlr[8:], handler)
	return box("trak", box("tkhd", tkhd), box("mdia", box("hdlr", hdlr)))
}

func TestVideosGetTheirLengthAndShape(t *testing.T) {
	t.Parallel()
	ftyp := box("ftyp", []byte("isom"))
	for _, tt := range []struct {
		name string
		file []byte
		want media.Movie
	}{
		{name: "landscape", file: slices.Concat(ftyp, box("moov", mvhd(0, 1000, 12500), trak("soun", 0, 0, false), trak("vide", 1920, 1080, false))), want: media.Movie{Seconds: 13, Width: 1920, Height: 1080}},
		{name: "portrait from a phone", file: slices.Concat(ftyp, box("moov", mvhd(1, 600, 6000), trak("vide", 1920, 1080, true))), want: media.Movie{Seconds: 10, Width: 1080, Height: 1920}},
		{name: "moov after the data", file: slices.Concat(ftyp, box("mdat", make([]byte, 64)), box("moov", mvhd(0, 44100, 44100*3))), want: media.Movie{Seconds: 3}},
	} {
		if got, ok := media.MP4(tt.file); !ok || got != tt.want {
			t.Errorf("%s: MP4() = %+v, %v; want %+v", tt.name, got, ok, tt.want)
		}
	}
	for name, data := range map[string][]byte{
		"no moov":        ftyp,
		"zero timescale": box("moov", mvhd(0, 0, 10)),
		"lying size":     append(binary.BigEndian.AppendUint32(nil, 4096), "moov"...),
		"garbage":        []byte("not a movie at all"),
	} {
		if _, ok := media.MP4(data); ok {
			t.Errorf("%s was read as a movie", name)
		}
	}
}
