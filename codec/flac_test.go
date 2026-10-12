package codec

import (
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/desireco/entrain/internal/dsp"
	"github.com/mewkiz/flac"
)

// TestFlacLosslessRoundtrip verifies FLAC output decodes back to the exact
// int16 stream the WAV path writes (shared quantizer stage, same seed).
func TestFlacLosslessRoundtrip(t *testing.T) {
	const sr = 44100
	n := sr / 2 // 0.5 s
	x := make([]float64, 2*n)
	for i := range n {
		s := 0.5 * math.Sin(2*math.Pi*440*float64(i)/sr)
		x[2*i], x[2*i+1] = s, s
	}

	wavPath := filepath.Join(t.TempDir(), "a.wav")
	f, err := os.Create(wavPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteWav16(f, sr, x); err != nil {
		t.Fatal(err)
	}
	f.Close()

	flacPath := filepath.Join(t.TempDir(), "a.flac")
	f, err = os.Create(flacPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteFlac16(f, sr, x); err != nil {
		t.Fatal(err)
	} // encoder closed the file

	// WAV reference: what the 16-bit stream must be.
	wavRaw, err := os.ReadFile(wavPath)
	if err != nil {
		t.Fatal(err)
	}
	_, wavX, err := dsp.ReadWav16(wavRaw)
	if err != nil {
		t.Fatal(err)
	}

	stream, err := flac.ParseFile(flacPath)
	if err != nil {
		t.Fatalf("decode flac: %v", err)
	}
	defer stream.Close()
	if stream.Info.SampleRate != sr || stream.Info.NChannels != 2 || stream.Info.BitsPerSample != 16 {
		t.Fatalf("streaminfo = %v/%v/%v", stream.Info.SampleRate, stream.Info.NChannels, stream.Info.BitsPerSample)
	}
	if stream.Info.NSamples != uint64(n) {
		t.Fatalf("nsamples = %d, want %d", stream.Info.NSamples, n)
	}

	var got []float64
	for {
		fr, err := stream.ParseNext()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		for i := range fr.Subframes[0].Samples {
			l := fr.Subframes[0].Samples[i]
			r := fr.Subframes[1].Samples[i]
			got = append(got, float64(l)/32768, float64(r)/32768)
		}
	}
	if len(got) != len(wavX) {
		t.Fatalf("flac decoded %d samples, wav has %d", len(got), len(wavX))
	}
	for i := range got {
		if got[i] != wavX[i] {
			t.Fatalf("lossless mismatch at %d: flac %v vs wav %v (Δ=%g LSB)",
				i, got[i], wavX[i], abs32(got[i]-wavX[i])*32768)
		}
	}
}

func TestFlacDeterministic(t *testing.T) {
	x := make([]float64, 8192)
	for i := range x {
		x[i] = 0.3 * math.Sin(2*math.Pi*220*float64(i/2)/44100)
	}
	a, b := flacBytes(t, x), flacBytes(t, x)
	if string(a) != string(b) {
		t.Fatal("same input encoded to different bytes")
	}
}

func flacBytes(t *testing.T, x []float64) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "a.flac")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteFlac16(f, 44100, x); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func abs32(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
