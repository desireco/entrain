package codec

import (
	"encoding/binary"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// Metadata carries render labels into container tags (ID3v2 on MP3).
// Empty fields omit their frames; more fields (album, genre) can slot
// in later as further text frames.
type Metadata struct {
	Title string
}

// mp3EncoderTag names the writing software in every tagged MP3 render.
const mp3EncoderTag = "Entrain"

// FileStem is the default label for renders of unnamed sessions: the
// output file name without extension, so the player shows the same
// word the command line printed.
func FileStem(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

// id3v2Tag builds an ID3v2.3 tag: TIT2 from the title when set, and
// TSSE naming the writer. Text frames carry UTF-16LE with a BOM
// (encoding $01) so non-Latin titles survive; frame sizes are
// big-endian as v2.3 specifies, and the tag body size is synchsafe so
// the header never reads as a frame sync.
func id3v2Tag(meta Metadata) []byte {
	frames := make([][]byte, 0, 2)
	if meta.Title != "" {
		frames = append(frames, id3TextFrame("TIT2", meta.Title))
	}
	frames = append(frames, id3TextFrame("TSSE", mp3EncoderTag))
	var body []byte
	for _, f := range frames {
		body = append(body, f...)
	}
	n := len(body)
	tag := make([]byte, 10, 10+n)
	copy(tag, "ID3")
	tag[3], tag[4], tag[5] = 3, 0, 0 // version 2.3, no flags
	for i, shift := range [4]uint{21, 14, 7, 0} {
		tag[6+i] = byte(n>>shift) & 0x7f
	}
	return append(tag, body...)
}

// id3TextFrame encodes one v2.3 text frame with its terminating nulls.
func id3TextFrame(id, s string) []byte {
	uni := utf16.Encode([]rune(s))
	payload := make([]byte, 3, 3+2*len(uni)+2)
	payload[0] = 0x01 // text encoding: UTF-16 with BOM
	payload[1], payload[2] = 0xff, 0xfe
	for _, r := range uni {
		payload = append(payload, byte(r), byte(r>>8))
	}
	payload = append(payload, 0, 0)
	frame := make([]byte, 10, 10+len(payload))
	copy(frame, id)
	binary.BigEndian.PutUint32(frame[4:], uint32(len(payload)))
	return append(frame, payload...)
}

// decodeId3Text is the inverse of id3TextFrame's payload encoding, used
// by tests (and any future verify path) to round-trip titles.
func decodeId3Text(payload []byte) string {
	if len(payload) < 4 || payload[0] != 0x01 {
		return ""
	}
	body := payload[1:]
	if body[0] == 0xff && body[1] == 0xfe {
		body = body[2:]
	}
	uni := make([]uint16, 0, len(body)/2)
	for i := 0; i+1 < len(body); i += 2 {
		r := uint16(body[i]) | uint16(body[i+1])<<8
		if r == 0 {
			break
		}
		uni = append(uni, r)
	}
	return string(utf16.Decode(uni))
}
