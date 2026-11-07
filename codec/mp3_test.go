package codec

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func mp3Bytes(t *testing.T, x []float64, bitrate int, meta Metadata) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "a.mp3")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteMp3(f, 44100, bitrate, x, meta); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// mp3Header decodes the first frame header of an MP3 stream, skipping
// the ID3v2 tag every render now carries.
type mp3Header struct {
	mpeg1          bool
	layer3         bool
	bitrateKbps    int
	sampleRate     int
	padding        bool
	channelsStereo bool
}

func parseMp3Header(b []byte) (mp3Header, bool) {
	if len(b) >= 10 && string(b[:3]) == "ID3" {
		// Synchsafe tag body size: seven bits per byte, big-endian.
		size := int(b[6]&0x7f)<<21 | int(b[7]&0x7f)<<14 | int(b[8]&0x7f)<<7 | int(b[9]&0x7f)
		b = b[10+size:]
	}
	if len(b) < 4 || b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return mp3Header{}, false
	}
	h := mp3Header{
		mpeg1:  b[1]&0x18 == 0x18, // version bits 11 = MPEG-1
		layer3: b[1]&0x06 == 0x02, // layer bits 01 = Layer III
	}
	idx := int(b[2] >> 4)
	h.bitrateKbps = mpeg1Layer3Bitrates[idx]
	switch b[2] >> 2 & 0x3 {
	case 0:
		h.sampleRate = 44100
	case 1:
		h.sampleRate = 48000
	case 2:
		h.sampleRate = 32000
	}
	h.padding = b[2]&0x02 != 0
	h.channelsStereo = b[3]>>6 == 0
	return h, true
}

// TestMp3Header checks the first frame header carries the requested
// bitrate/rate/stereo without needing a decoder dependency.
func TestMp3Header(t *testing.T) {
	const sr = 44100
	n := sr // 1 s
	x := make([]float64, 2*n)
	for i := range n {
		s := 0.5 * math.Sin(2*math.Pi*440*float64(i)/sr)
		x[2*i], x[2*i+1] = s, s
	}

	b := mp3Bytes(t, x, 320, Metadata{Title: "Header Check"})
	h, ok := parseMp3Header(b)
	if !ok {
		t.Fatalf("no frame header at start of stream: % x", b[:8])
	}
	if !h.mpeg1 || !h.layer3 || !h.channelsStereo {
		t.Fatalf("header = %+v, want MPEG-1 Layer III stereo", h)
	}
	if h.bitrateKbps != 320 {
		t.Errorf("bitrate = %d kbps, want 320", h.bitrateKbps)
	}
	if h.sampleRate != 44100 {
		t.Errorf("sample rate = %d, want 44100", h.sampleRate)
	}

	// Rough size check: 320 kbps ≈ 40 kB/s, within frame-padding slop.
	wantBytes := 320_000 / 8
	if lo, hi := wantBytes*9/10, wantBytes*11/10; len(b) < lo || len(b) > hi {
		t.Errorf("1 s @ 320 kbps encoded to %d bytes, want ≈%d", len(b), wantBytes)
	}
}

func TestMp3Header48k(t *testing.T) {
	x := make([]float64, 48000) // 0.5 s
	for i := range x {
		x[i] = 0.2 * math.Sin(2*math.Pi*300*float64(i)/48000)
	}
	path := filepath.Join(t.TempDir(), "a.mp3")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// mono-shaped input still encodes as stereo per WriteMp3 contract
	if err := WriteMp3(f, 48000, 192, x, Metadata{Title: "48k"}); err != nil {
		t.Fatal(err)
	}
	f.Close()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h, ok := parseMp3Header(b)
	if !ok || h.bitrateKbps != 192 || h.sampleRate != 48000 {
		t.Fatalf("48k/192 header = %+v ok=%v", h, ok)
	}
}

func TestMp3RejectsBadConfig(t *testing.T) {
	var buf bytes.Buffer
	x := make([]float64, 4096)
	if err := WriteMp3(&buf, 44100, 321, x, Metadata{}); err == nil {
		t.Fatal("321 kbps should be rejected")
	}
	buf.Reset()
	if err := WriteMp3(&buf, 22050, 320, x, Metadata{}); err == nil {
		t.Fatal("22050 Hz should be rejected (MPEG-1 rates only)")
	}
}

func TestMp3Deterministic(t *testing.T) {
	x := make([]float64, 16384)
	for i := range x {
		x[i] = 0.3 * math.Sin(2*math.Pi*220*float64(i/2)/44100)
	}
	a, b := mp3Bytes(t, x, 320, Metadata{Title: "Same"}), mp3Bytes(t, x, 320, Metadata{Title: "Same"})
	if !bytes.Equal(a, b) {
		t.Fatal("same input encoded to different bytes")
	}
}

func TestSubFrameMp3HasTwoDecodableFrames(t *testing.T) {
	b := mp3Bytes(t, make([]float64, 88), 320, Metadata{}) // 1 ms
	tag := 0
	if string(b[:3]) == "ID3" {
		tag = 10 + int(b[6]&0x7f)<<21 + int(b[7]&0x7f)<<14 + int(b[8]&0x7f)<<7 + int(b[9]&0x7f)
	}
	first, ok := parseMp3Header(b)
	if !ok {
		t.Fatal("missing first MP3 frame")
	}
	size := 144000 * first.bitrateKbps / first.sampleRate
	if first.padding {
		size++
	}
	if len(b) <= tag+size {
		t.Fatal("a single frame cannot be auto-detected by common players")
	}
	if _, ok := parseMp3Header(b[tag+size:]); !ok {
		t.Fatal("missing second MP3 frame")
	}
}

// TestMp3Metadata walks the ID3v2.3 tag: TIT2 carries the session title
// (UTF-16LE round-trip), TSSE names the writer, and a render without a
// title ships only the encoder frame.
func TestMp3Metadata(t *testing.T) {
	b := mp3Bytes(t, make([]float64, 4096), 320, Metadata{Title: "Deep Sleep Descent"})
	if string(b[:3]) != "ID3" || b[3] != 3 || b[5] != 0 {
		t.Fatalf("want ID3v2.3 header, got % x", b[:6])
	}
	body := int(b[6]&0x7f)<<21 | int(b[7]&0x7f)<<14 | int(b[8]&0x7f)<<7 | int(b[9]&0x7f)
	if 10+body > len(b) {
		t.Fatalf("tag size %d exceeds stream len %d", body, len(b))
	}
	want := map[string]string{"TIT2": "Deep Sleep Descent", "TSSE": mp3EncoderTag}
	for at := 10; at+len(want) > 0 && at+10 <= 10+body; {
		id := string(b[at : at+4])
		size := int(binary.BigEndian.Uint32(b[at+4 : at+8]))
		if size <= 0 || at+10+size > 10+body {
			t.Fatalf("frame %q at %d has bad size %d", id, at, size)
		}
		text, ok := want[id]
		if !ok {
			t.Fatalf("unexpected frame %q", id)
		}
		if got := decodeId3Text(b[at+10 : at+10+size]); got != text {
			t.Fatalf("%s = %q, want %q", id, got, text)
		}
		delete(want, id)
		at += 10 + size
	}
	if len(want) != 0 {
		t.Fatalf("missing frames: %v", want)
	}

	// No title: only the encoder frame, still a parseable header after it.
	plain := mp3Bytes(t, make([]float64, 4096), 320, Metadata{})
	plainBody := int(plain[6]&0x7f)<<21 | int(plain[7]&0x7f)<<14 | int(plain[8]&0x7f)<<7 | int(plain[9]&0x7f)
	if bytes.Contains(plain[10:10+plainBody], []byte("TIT2")) {
		t.Fatal("untitled render must not carry TIT2")
	}
	if _, ok := parseMp3Header(plain); !ok {
		t.Fatal("frame header lost after the ID3 tag")
	}
}
