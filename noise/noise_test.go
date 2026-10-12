package noise_test

import (
	"math"
	"testing"

	"github.com/desireco/entrain/internal/dsp"
	"github.com/desireco/entrain/noise"
)

const sr = 44100

func TestAlphaSlopes(t *testing.T) {
	for _, tc := range []struct {
		alpha float64
		want  float64 // dB per octave
	}{
		{0.5, -1.5},
		{1.0, -3.0},
		{2.0, -6.0},
	} {
		x := noise.Alpha(tc.alpha, sr, 42, 4*sr) // 4 s ≈ 1 chunk+joint
		l, _ := dsp.Deinterleave(x)

		centers := []float64{125, 250, 500, 1000, 2000, 4000}
		var sum float64
		for i := 1; i < len(centers); i++ {
			s := 10*math.Log10(bandPower(l, centers[i])) - 10*math.Log10(bandPower(l, centers[i-1]))
			sum += s
		}
		mean := sum / float64(len(centers)-1)
		if math.Abs(mean-tc.want) > 1.5 {
			t.Errorf("alpha %.1f: mean slope %.2f dB/oct, want %.1f ± 1.5", tc.alpha, mean, tc.want)
		}
	}
}

func TestAlphaLevelAndDecorrelation(t *testing.T) {
	x := noise.Alpha(1.0, sr, 7, 2*sr)
	l, r := dsp.Deinterleave(x)
	if got := dsp.RMS(l); got < 0.9 || got > 1.1 {
		t.Errorf("L RMS = %.3f, want unit", got)
	}
	if got := dsp.Mean(l); math.Abs(got) > 0.05 {
		t.Errorf("DC = %.3f", got)
	}
	if c := dsp.Corr(l, r); math.Abs(c) > 0.05 {
		t.Errorf("L/R corr = %.3f", c)
	}
}

func TestAlphaDeterministicAndLength(t *testing.T) {
	a := noise.Alpha(1.5, sr, 9, sr)
	b := noise.Alpha(1.5, sr, 9, sr)
	if len(a) != 2*sr {
		t.Fatalf("length = %d", len(a))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("same seed diverged")
		}
	}
}

func TestBasicColors(t *testing.T) {
	for _, color := range []string{"white", "pink", "brown"} {
		x, err := noise.Basic(color, sr, 3, sr)
		if err != nil {
			t.Fatal(err)
		}
		l, _ := dsp.Deinterleave(x)
		if got := dsp.RMS(l); got < 0.93 || got > 1.07 {
			t.Errorf("%s RMS = %.3f", color, got)
		}
	}
	if _, err := noise.Basic("violet", sr, 1, 100); err == nil {
		t.Error("unknown color should error")
	}
}

// bandPower: mean |X|² over log-spaced integer-Hz bins inside the octave
// around fc (1 s windows → exact bins).
func bandPower(x []float64, fc float64) float64 {
	var bins []float64
	seen := map[int]bool{}
	for i := range 24 {
		f := math.Round(fc * math.Pow(2, float64(i)/23-0.5))
		if f >= 1 && f < sr/2 && !seen[int(f)] {
			seen[int(f)] = true
			bins = append(bins, f)
		}
	}
	var sum float64
	var n int
	for w := 0; w+sr <= len(x); w += sr {
		chunk := x[w : w+sr]
		for _, f := range bins {
			m := dsp.Goertzel(chunk, sr, f)
			sum += m * m
			n++
		}
	}
	return sum / float64(n)
}
