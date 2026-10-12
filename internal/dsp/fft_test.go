package dsp_test

import (
	"math"
	"testing"

	"github.com/desireco/entrain/internal/dsp"
)

func TestFFTSine(t *testing.T) {
	// 440 Hz at N=65536/44100 lands on bin 440·65536/44100 ≈ 654.
	const n = 1 << 16
	x := make([]float64, n)
	for i := range x {
		x[i] = math.Sin(2 * math.Pi * 440 * float64(i) / testSR)
	}
	mag := dsp.Spectrum(x)
	if got := len(mag); got != n/2+1 {
		t.Fatalf("spectrum length = %d", got)
	}
	best, bestMag := 0, 0.0
	for i, m := range mag {
		if m > bestMag {
			best, bestMag = i, m
		}
	}
	if best != 654 {
		t.Fatalf("peak bin = %d, want 654", best)
	}
	// Hann-windowed sine magnitude ≈ A·N/4 (A=1 → N/4).
	if want := float64(n) / 4; math.Abs(bestMag-want) > 0.02*want {
		t.Fatalf("peak magnitude = %.1f, want ≈%.1f", bestMag, want)
	}
}

func TestPeaksInterpolated(t *testing.T) {
	// 123.4 Hz sits between bins; parabolic interpolation must recover it.
	const n = 1 << 16
	x := make([]float64, n)
	for i := range x {
		x[i] = 0.5 * math.Sin(2*math.Pi*123.4*float64(i)/testSR)
	}
	peaks := dsp.Peaks(dsp.Spectrum(x), testSR, 20, 2000, 5)
	if len(peaks) == 0 {
		t.Fatal("no peaks found")
	}
	if math.Abs(peaks[0].Freq-123.4) > 0.05 {
		t.Fatalf("interpolated peak = %.3f Hz, want 123.4 ± 0.05", peaks[0].Freq)
	}
}

func TestPeaksResolvesCloseTones(t *testing.T) {
	// 160 and 200 Hz (a 40 Hz binaural pair's ears) must come out as two
	// distinct peaks.
	const n = 1 << 16
	x := make([]float64, n)
	for i := range x {
		x[i] = 0.3*math.Sin(2*math.Pi*160*float64(i)/testSR) +
			0.25*math.Sin(2*math.Pi*200*float64(i)/testSR)
	}
	peaks := dsp.Peaks(dsp.Spectrum(x), testSR, 20, 2000, 5)
	if len(peaks) < 2 {
		t.Fatalf("peaks = %+v, want ≥2", peaks)
	}
	f0, f1 := peaks[0].Freq, peaks[1].Freq
	if math.Abs(f0-160) > 0.1 || math.Abs(f1-200) > 0.1 {
		t.Fatalf("peaks = %.3f, %.3f Hz; want 160/200", f0, f1)
	}
}

func TestPeaksProminenceFiltersNoise(t *testing.T) {
	// White noise alone: with a high prominence floor, few or no peaks.
	const n = 1 << 14
	x := make([]float64, n)
	seed := 1.0
	next := func() float64 { seed = math.Mod(seed*16807, 2147483647); return seed / 2147483647 }
	for i := range x {
		x[i] = next()*2 - 1
	}
	peaks := dsp.Peaks(dsp.Spectrum(x), testSR, 20, 2000, 20)
	if len(peaks) > 2 {
		t.Fatalf("noise produced %d prominent peaks; prominence filter too loose", len(peaks))
	}
}
