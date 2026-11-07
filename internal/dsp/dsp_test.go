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

func TestExtensiblePCM(t *testing.T) {
	// Stereo 48 kHz, 24-bit PCM. Each channel has one sample, +0.5 and -0.5.
	wav := []byte{
		'R', 'I', 'F', 'F', 66, 0, 0, 0, 'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ', 40, 0, 0, 0, 0xfe, 0xff, 2, 0,
		0x80, 0xbb, 0, 0, 0, 0x65, 4, 0, 6, 0, 24, 0,
		22, 0, 24, 0, 3, 0, 0, 0,
		1, 0, 0, 0, 0, 0, 0x10, 0, 0x80, 0, 0, 0xaa, 0, 0x38, 0x9b, 0x71,
		'd', 'a', 't', 'a', 6, 0, 0, 0, 0, 0, 0x40, 0, 0, 0xc0,
	}
	rate, x, bits, err := dsp.ReadWav(wav)
	if err != nil || rate != 48000 || bits != 24 || len(x) != 2 || x[0] != 0.5 || x[1] != -0.5 {
		t.Fatalf("decoded rate=%d bits=%d x=%v err=%v", rate, bits, x, err)
	}
	zeroRate := append([]byte(nil), wav...)
	clear(zeroRate[24:28])
	if _, _, _, err := dsp.ReadWav(zeroRate); err == nil {
		t.Fatal("zero sample rate accepted")
	}
	for _, tc := range []struct {
		name   string
		offset int
		value  byte
	}{
		{"short fmt", 16, 38},
		{"short extension", 36, 20},
		{"overlong extension", 36, 24},
		{"float subtype", 44, 3},
		{"unrelated GUID", 59, 0},
		{"invalid valid bits", 38, 25},
		{"surround channel pair", 40, 12},
		{"zero block alignment", 32, 0},
		{"wrong block alignment", 32, 4},
		{"incomplete data frame", 64, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := append([]byte(nil), wav...)
			b[tc.offset] = tc.value
			if _, _, _, err := dsp.ReadWav(b); err == nil {
				t.Fatal("malformed or unsupported WAV accepted")
			}
		})
	}
}
