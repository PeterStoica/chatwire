package fakemedia

import (
	"encoding/binary"
	"slices"
)

func VoiceNote(seconds uint64) []byte {
	head := []byte("OpusHead\x01\x01\x38\x01\x80\xbb\x00\x00\x00\x00\x00")
	audio := make([][]byte, 50)
	for i := range audio {
		audio[i] = make([]byte, 20+i)
	}
	return slices.Concat(page(0, head), page(0, []byte("OpusTags")), page(312+seconds*48000, audio...))
}

func page(granule uint64, packets ...[]byte) []byte {
	var table, body []byte
	for _, p := range packets {
		table = append(table, byte(len(p)))
		body = append(body, p...)
	}
	header := make([]byte, 27)
	copy(header, "OggS")
	binary.LittleEndian.PutUint64(header[6:14], granule)
	header[26] = byte(len(table))
	return slices.Concat(header, table, body)
}

func Video(seconds, width, height uint32) []byte {
	mvhd := make([]byte, 20)
	binary.BigEndian.PutUint32(mvhd[12:], 1000)
	binary.BigEndian.PutUint32(mvhd[16:], seconds*1000)
	tkhd := make([]byte, 84)
	binary.BigEndian.PutUint32(tkhd[40:], 0x10000)
	binary.BigEndian.PutUint32(tkhd[56:], 0x10000)
	binary.BigEndian.PutUint32(tkhd[76:], width<<16)
	binary.BigEndian.PutUint32(tkhd[80:], height<<16)
	hdlr := make([]byte, 12)
	copy(hdlr[8:], "vide")
	moov := box("moov", box("mvhd", mvhd), box("trak", box("tkhd", tkhd), box("mdia", box("hdlr", hdlr))))
	return slices.Concat(box("ftyp", []byte("isomiso2mp41")), moov, box("mdat", make([]byte, 32)))
}

func box(kind string, parts ...[]byte) []byte {
	payload := slices.Concat(parts...)
	out := binary.BigEndian.AppendUint32(nil, uint32(8+len(payload)))
	return append(append(out, kind...), payload...)
}
