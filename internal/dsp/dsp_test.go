package dsp_test

import (
	"math"
	"testing"

	"github.com/desireco/entrain/internal/dsp"
)

const testSR = 44100

func sine(f float64, n int, amp float64) []float64 {
	x := make([]float64, n)
	for i := range x {
		x[i] = amp * math.Sin(2*math.Pi*f*float64(i)/testSR)
	}
	return x
}

func TestGoertzelExactBin(t *testing.T) {
	n := 44100
	got := dsp.Goertzel(sine(440, n, 1), testSR, 440)
	if want := float64(n) / 2; math.Abs(got-want) > 1e-6*want {
		t.Fatalf("Goertzel(sine@440) = %g, want %g (A·N/2)", got, want)
	}
}

func TestGoertzelOrthogonalBin(t *testing.T) {
	n := 44100
	got := dsp.Goertzel(sine(440, n, 1), testSR, 441)
	if want := 0.001 * float64(n) / 2; got > want {
		t.Fatalf("Goertzel(sine@440, bin 441) = %g, want < %g (orthogonal bin)", got, want)
	}
}

func TestStats(t *testing.T) {
	one := []float64{1, 2, 3, 4}
	if m := dsp.Mean(one); m != 2.5 {
		t.Fatalf("Mean = %g, want 2.5", m)
	}
	if r := dsp.RMS(one); math.Abs(r-math.Sqrt(7.5)) > 1e-12 {
		t.Fatalf("RMS = %g, want %g", r, math.Sqrt(7.5))
	}
	if c := dsp.Corr(one, one); math.Abs(c-1) > 1e-12 {
		t.Fatalf("Corr(x,x) = %g, want 1", c)
	}
	if c := dsp.Corr(one, []float64{-1, -2, -3, -4}); math.Abs(c+1) > 1e-12 {
		t.Fatalf("Corr(x,−x) = %g, want −1", c)
	}
}

func TestReadWav16(t *testing.T) {
	// Hand-built minimal PCM WAV: 1 Hz of two int16 samples (100, −100).
	b := []byte{
		'R', 'I', 'F', 'F', 36, 0, 0, 0, 'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ', 16, 0, 0, 0,
		1, 0, // PCM
		2, 0, // stereo
		0x44, 0xAC, 0, 0, // 44100 Hz
		0x10, 0xB1, 0x02, 0, // byte rate
		4, 0, // block align
		16, 0, // bits
		'd', 'a', 't', 'a', 4, 0, 0, 0,
		100, 0, 0x9C, 0xFF, // int16 100, −100
	}
	sr, x, err := dsp.ReadWav16(b)
	if err != nil {
		t.Fatalf("ReadWav16: %v", err)
	}
	if sr != 44100 || len(x) != 2 {
		t.Fatalf("sr=%d len=%d, want 44100/2", sr, len(x))
	}
	if math.Abs(x[0]-100.0/32768) > 1e-9 || math.Abs(x[1]+100.0/32768) > 1e-9 {
		t.Fatalf("samples = %v, want [100/32768, −100/32768]", x)
	}
	if _, _, err := dsp.ReadWav16([]byte("not a wav")); err == nil {
		t.Fatal("garbage input accepted")
	}
}
