package media

import (
	"bytes"
	"encoding/binary"
	"slices"
)

const (
	opusRate      = 48000
	waveformSize  = 64
	waveformPeak  = 100
	oggHeaderSize = 27
	maxBoxDepth   = 8
)

type Recording struct {
	Seconds  uint32
	Waveform []byte
}

func OggOpus(data []byte) (Recording, bool) {
	var (
		packets   []int
		packet    int
		preSkip   uint64
		granule   uint64
		sawHeader bool
	)
	for len(data) >= oggHeaderSize && bytes.HasPrefix(data, []byte("OggS")) {
		segments := int(data[26])
		if len(data) < oggHeaderSize+segments {
			break
		}
		if position := binary.LittleEndian.Uint64(data[6:14]); position != ^uint64(0) {
			granule = max(granule, position)
		}
		table := data[oggHeaderSize : oggHeaderSize+segments]
		body := data[oggHeaderSize+segments:]
		offset := 0
		for _, lacing := range table {
			if offset+int(lacing) > len(body) {
				return Recording{}, false
			}
			if !sawHeader && packet == 0 {
				head := body[offset:min(offset+19, len(body))]
				if !bytes.HasPrefix(head, []byte("OpusHead")) || len(head) < 12 {
					return Recording{}, false
				}
				preSkip, sawHeader = uint64(binary.LittleEndian.Uint16(head[10:12])), true
			}
			packet += int(lacing)
			offset += int(lacing)
			if lacing < 255 {
				packets = append(packets, packet)
				packet = 0
			}
		}
		data = body[offset:]
	}
	if !sawHeader || granule <= preSkip {
		return Recording{}, false
	}
	audio := packets[min(2, len(packets)):]
	return Recording{Seconds: uint32((granule - preSkip + opusRate - 1) / opusRate), Waveform: waveform(audio)}, true
}

func waveform(sizes []int) []byte {
	out := make([]byte, waveformSize)
	if len(sizes) == 0 {
		return out
	}
	levels := make([]int, waveformSize)
	for i := range levels {
		from, to := i*len(sizes)/waveformSize, max((i+1)*len(sizes)/waveformSize, i*len(sizes)/waveformSize+1)
		chunk := sizes[min(from, len(sizes)-1):min(to, len(sizes))]
		sum := 0
		for _, s := range chunk {
			sum += s
		}
		levels[i] = sum / len(chunk)
	}
	peak := slices.Max(levels)
	if peak == 0 {
		return out
	}
	for i, level := range levels {
		out[i] = byte(level * waveformPeak / peak)
	}
	return out
}

type Movie struct {
	Seconds uint32
	Width   uint32
	Height  uint32
}

func MP4(data []byte) (Movie, bool) {
	var movie Movie
	found := false
	walkBoxes(data, 0, func(kind string, payload []byte) bool {
		switch kind {
		case "moov", "trak", "mdia":
			return true
		case "mvhd":
			if seconds, ok := movieSeconds(payload); ok {
				movie.Seconds, found = seconds, true
			}
		}
		return false
	})
	walkBoxes(data, 0, func(kind string, payload []byte) bool {
		if kind == "moov" {
			return true
		}
		if kind == "trak" && movie.Width == 0 && handler(payload) == "vide" {
			movie.Width, movie.Height = trackSize(payload)
		}
		return false
	})
	return movie, found
}

func walkBoxes(data []byte, depth int, visit func(kind string, payload []byte) bool) {
	for len(data) >= 8 && depth < maxBoxDepth {
		size, kind, header := uint64(binary.BigEndian.Uint32(data[:4])), string(data[4:8]), uint64(8)
		switch size {
		case 0:
			size = uint64(len(data))
		case 1:
			if len(data) < 16 {
				return
			}
			size, header = binary.BigEndian.Uint64(data[8:16]), 16
		}
		if size < header || size > uint64(len(data)) {
			return
		}
		if payload := data[header:size]; visit(kind, payload) {
			walkBoxes(payload, depth+1, visit)
		}
		data = data[size:]
	}
}

func movieSeconds(payload []byte) (uint32, bool) {
	if len(payload) < 4 {
		return 0, false
	}
	var timescale, duration uint64
	switch payload[0] {
	case 0:
		if len(payload) < 20 {
			return 0, false
		}
		timescale, duration = uint64(binary.BigEndian.Uint32(payload[12:16])), uint64(binary.BigEndian.Uint32(payload[16:20]))
	case 1:
		if len(payload) < 32 {
			return 0, false
		}
		timescale, duration = uint64(binary.BigEndian.Uint32(payload[20:24])), binary.BigEndian.Uint64(payload[24:32])
	default:
		return 0, false
	}
	if timescale == 0 {
		return 0, false
	}
	return uint32((duration + timescale - 1) / timescale), true
}

func handler(trak []byte) string {
	kind := ""
	walkBoxes(trak, 0, func(box string, payload []byte) bool {
		if box == "mdia" {
			return true
		}
		if box == "hdlr" && len(payload) >= 12 {
			kind = string(payload[8:12])
		}
		return false
	})
	return kind
}

func trackSize(trak []byte) (width, height uint32) {
	walkBoxes(trak, 0, func(box string, payload []byte) bool {
		if box != "tkhd" || len(payload) < 4 {
			return false
		}
		matrix := 40
		if payload[0] == 1 {
			matrix = 52
		}
		if len(payload) < matrix+44 {
			return false
		}
		a, d := binary.BigEndian.Uint32(payload[matrix:]), binary.BigEndian.Uint32(payload[matrix+16:])
		width, height = binary.BigEndian.Uint32(payload[matrix+36:])>>16, binary.BigEndian.Uint32(payload[matrix+40:])>>16
		if a == 0 && d == 0 {
			width, height = height, width
		}
		return false
	})
	return width, height
}
