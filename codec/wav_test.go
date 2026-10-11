package codec

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/desireco/entrain/internal/dsp"
)

func writeRead(t *testing.T, sr int, x []float64) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.wav")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteWav16(f, sr, x); err != nil {
		t.Fatal(err)
	}
	f.Close()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestWavHeader(t *testing.T) {
	sr := 44100
	n := 44100 // 1 s
	x := make([]float64, 2*n)
	for i := range n {
		s := 0.5 * math.Sin(2*math.Pi*440*float64(i)/float64(sr))
		x[2*i], x[2*i+1] = s, s
	}
	b := writeRead(t, sr, x)

	if string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		t.Fatal("missing RIFF/WAVE signature")
	}
	if dataLen := int(b[40]) | int(b[41])<<8 | int(b[42])<<16 | int(b[43])<<24; dataLen != 4*n {
		t.Fatalf("data chunk size = %d, want %d", dataLen, 4*n)
	}

	gotSR, y, err := dsp.ReadWav16(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if gotSR != sr || len(y) != 2*n {
		t.Fatalf("sr=%d len=%d, want %d/%d", gotSR, len(y), sr, 2*n)
	}

	l, r := dsp.Deinterleave(y)
	if got := dsp.Goertzel(l, float64(sr), 440); math.Abs(got-0.5*float64(n)/2) > 0.03*float64(n)/2 {
		t.Fatalf("L@440 = %.0f, want %.0f ± 3%%", got, 0.5*float64(n)/2)
	}
	// Identical channels stay correlated to within dither's per-sample
	// decorrelation (~1 LSB of noise vs a 0.35-RMS signal ⇒ ~1e-9).
	if c := dsp.Corr(l, r); c < 1-1e-6 {
		t.Fatalf("identical channels came back decorrelated: corr = %.7f", c)
	}
}

func TestWavClampsInsteadOfWrapping(t *testing.T) {
	x := make([]float64, 512)
	for i := range x {
		x[i] = []float64{2.5, -2.5, 1.00003, -1.00003, 0}[i%5]
	}
	_, y, err := dsp.ReadWav16(writeRead(t, 44100, x))
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range y {
		if s > 1.0 || s < -1.0001 {
			t.Fatalf("sample %d = %g escaped clamp range", i, s)
		}
	}
}

func TestWavDeterministic(t *testing.T) {
	x := make([]float64, 4096)
	for i := range x {
		x[i] = 0.3 * math.Sin(2*math.Pi*220*float64(i)/44100)
	}
	a := writeRead(t, 44100, x)
	b := writeRead(t, 44100, x)
	if !bytes.Equal(a, b) {
		t.Fatal("same input encoded to different bytes")
	}
}

// TestDitherCarriesSmallSignal checks the point of dithering: a −80 dBFS
// tone survives the 16-bit round trip measurably (below one quantization
// step per peak, it would distort into step noise without dither).
func TestDitherCarriesSmallSignal(t *testing.T) {
	sr := 44100
	n := 44100
	const amp = 0.0001 // −80 dBFS
	x := make([]float64, 2*n)
	for i := range n {
		s := amp * math.Sin(2*math.Pi*1000*float64(i)/float64(sr))
		x[2*i], x[2*i+1] = s, s
	}
	_, y, err := dsp.ReadWav16(writeRead(t, sr, x))
	if err != nil {
		t.Fatal(err)
	}
	l, _ := dsp.Deinterleave(y)
	want := amp * float64(n) / 2
	if got := dsp.Goertzel(l, float64(sr), 1000); math.Abs(got-want) > 0.25*want {
		t.Fatalf("−80 dBFS tone measured %.3f, want %.3f ± 25%%", got, want)
	}
}
