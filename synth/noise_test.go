package synth_test

import (
	"math"
	"testing"

	"github.com/desireco/entrain/internal/dsp"
	"github.com/desireco/entrain/synth"
)

// genNoise renders n mono-left/mono-right sample pairs of the given color.
func genNoise(color synth.NoiseColor, seed uint64, n int) (l, r []float64) {
	return deinterleave(synth.Render(synth.NewNoise(testSR, color, seed), n))
}

// TestNoiseQualityBar checks the palette's standing quality bar: unit RMS per
// channel, no DC offset, and decorrelated stereo.
func TestNoiseQualityBar(t *testing.T) {
	for _, tc := range []struct {
		color synth.NoiseColor
		dcTol float64 // |mean| bound; correlated colors converge slower
	}{
		{synth.White, 0.01},
		{synth.Pink, 0.06},
		{synth.Brown, 0.06},
	} {
		l, r := genNoise(tc.color, 1, 2_000_000)

		if rms := dsp.RMS(l); rms < 0.94 || rms > 1.06 {
			t.Errorf("%v: L RMS = %.4f, want 1.00 ± 0.06", tc.color, rms)
		}
		if m := dsp.Mean(l); math.Abs(m) > tc.dcTol {
			t.Errorf("%v: L mean = %.4f, want |mean| < %g", tc.color, m, tc.dcTol)
		}
		if c := dsp.Corr(l, r); math.Abs(c) > 0.05 {
			t.Errorf("%v: L/R correlation = %.4f, want |corr| < 0.05 (decorrelated stereo)", tc.color, c)
		}
	}
}

// TestNoiseSpectralSlope verifies the palette's defining characteristic:
// white flat, pink −3 dB/octave, brown −6 dB/octave. Band power is the mean
// |X|² over log-spaced integer-Hz bins inside each octave (exact DFT bins
// via 1-second windows), averaged over several windows to tame estimator
// variance.
func TestNoiseSpectralSlope(t *testing.T) {
	for _, tc := range []struct {
		color   synth.NoiseColor
		want    float64 // dB per octave
		tolMean float64
	}{
		{synth.White, 0, 1.5},
		{synth.Pink, -3, 1.5},
		{synth.Brown, -6, 1.5},
	} {
		l, _ := genNoise(tc.color, 2, 4*44100)

		centers := []float64{125, 250, 500, 1000, 2000, 4000}
		var sum float64
		var slopes []float64
		for i := 1; i < len(centers); i++ {
			s := 10*math.Log10(bandPower(l, centers[i])) - 10*math.Log10(bandPower(l, centers[i-1]))
			slopes = append(slopes, s)
			sum += s
		}
		mean := sum / float64(len(slopes))
		if math.Abs(mean-tc.want) > tc.tolMean {
			t.Errorf("%v: mean slope = %.2f dB/oct, want %.1f ± %.1f", tc.color, mean, tc.want, tc.tolMean)
		}
		for i, s := range slopes {
			if math.Abs(s-tc.want) > 3.5 {
				t.Errorf("%v: octave %g→%g Hz slope = %.2f dB/oct, want %.1f ± 3.5", tc.color, centers[i], centers[i+1], s, tc.want)
			}
		}
	}
}

func TestNoiseDeterministic(t *testing.T) {
	l1, _ := genNoise(synth.Pink, 7, 500)
	l2, _ := genNoise(synth.Pink, 7, 500)
	for i := range l1 {
		if l1[i] != l2[i] {
			t.Fatalf("same seed diverged at sample %d", i)
		}
	}
	l3, _ := genNoise(synth.Pink, 8, 100)
	same := true
	for i := range l3 {
		if l1[i] != l3[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different seeds produced identical output")
	}
}

// bandPower estimates mean |X|² inside the octave centered on fc.
func bandPower(x []float64, fc float64) float64 {
	sr := float64(testSR)
	win := testSR // 1 s windows → bin index == integer frequency

	seen := map[int]bool{}
	var bins []float64
	for i := range 24 {
		f := math.Round(fc * math.Pow(2, float64(i)/23-0.5))
		if f >= 1 && f < sr/2 && !seen[int(f)] {
			seen[int(f)] = true
			bins = append(bins, f)
		}
	}

	var sum float64
	var count int
	for w := range len(x) / win {
		chunk := x[w*win : (w+1)*win]
		for _, f := range bins {
			m := dsp.Goertzel(chunk, sr, f)
			sum += m * m
			count++
		}
	}
	return sum / float64(count)
}
