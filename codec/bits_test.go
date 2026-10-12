package codec

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/desireco/entrain/internal/dsp"
	"github.com/mewkiz/flac"
)

// TestWav24HeaderAndRoundtrip checks 24-bit WAV output: header depth and
// a decode that preserves quiet detail below the 16-bit floor.
func TestWav24HeaderAndRoundtrip(t *testing.T) {
	const sr = 44100
	n := sr / 2
	x := make([]float64, 2*n)
	for i := range n {
		// A −90 dBFS tone: ~0.5 LSB at 16 bits, ~128 LSB at 24.
		s := 3.2e-5 * math.Sin(2*math.Pi*440*float64(i)/sr)
		x[2*i], x[2*i+1] = s, s
	}
	path := filepath.Join(t.TempDir(), "a.wav")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteWav(f, sr, 24, x); err != nil {
		t.Fatal(err)
	}
	f.Close()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := int(raw[34]) | int(raw[35])<<8; got != 24 {
		t.Fatalf("bits per sample = %d, want 24", got)
	}
	gotSR, y, bits, err := dsp.ReadWav(raw)
	if err != nil || bits != 24 || gotSR != sr {
		t.Fatalf("decode: %v (sr=%d bits=%d)", err, gotSR, bits)
	}
	// The −90 dBFS tone survives at 24 bits with its amplitude intact
	// (at 16 bits it would sit at ~0.5 LSB and drown in the dither floor).
	l, _ := dsp.Deinterleave(y)
	want := 3.2e-5 * float64(n) / 2
	if got := dsp.Goertzel(l, sr, 440); got < 0.7*want || got > 1.3*want {
		t.Errorf("−90 dBFS tone at 24 bits: magnitude %.3f, want %.3f ± 30%%", got, want)
	}
}

// TestWavFlac24BitIdentical: WAV and FLAC at the same depth carry
// bit-identical audio through the shared quantizer.
func TestWavFlac24BitIdentical(t *testing.T) {
	const sr = 44100
	n := sr / 2
	x := make([]float64, 2*n)
	for i := range n {
		s := 0.4 * math.Sin(2*math.Pi*330*float64(i)/sr)
		x[2*i], x[2*i+1] = s, -s
	}

	wavPath := filepath.Join(t.TempDir(), "a.wav")
	f, err := os.Create(wavPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteWav(f, sr, 24, x); err != nil {
		t.Fatal(err)
	}
	f.Close()
	wavRaw, _ := os.ReadFile(wavPath)
	_, wavX, _, err := dsp.ReadWav(wavRaw)
	if err != nil {
		t.Fatal(err)
	}

	flacPath := filepath.Join(t.TempDir(), "a.flac")
	f, err = os.Create(flacPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteFlac(f, sr, 24, x); err != nil {
		t.Fatal(err)
	} // encoder closed the file

	stream, err := flac.ParseFile(flacPath)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if stream.Info.BitsPerSample != 24 {
		t.Fatalf("flac bits = %d, want 24", stream.Info.BitsPerSample)
	}
	var flacX []float64
	for {
		fr, err := stream.ParseNext()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatal(err)
		}
		for i := range fr.Subframes[0].Samples {
			flacX = append(flacX,
				float64(fr.Subframes[0].Samples[i])/8388608,
				float64(fr.Subframes[1].Samples[i])/8388608)
		}
	}
	if len(flacX) != len(wavX) {
		t.Fatalf("lengths: flac %d vs wav %d", len(flacX), len(wavX))
	}
	for i := range flacX {
		if flacX[i] != wavX[i] {
			t.Fatalf("24-bit mismatch at %d: flac %v wav %v", i, flacX[i], wavX[i])
		}
	}
}

func TestBitDepthRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.wav")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := WriteWav(f, 44100, 8, make([]float64, 64)); err == nil {
		t.Fatal("8-bit wav should be rejected")
	}
}
