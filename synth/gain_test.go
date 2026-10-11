package synth_test

import (
	"math"
	"testing"
	"time"

	"github.com/desireco/entrain/internal/dsp"
	"github.com/desireco/entrain/synth"
)

// TestGlideLinearRamp verifies a scheduled glide tracks a linear amplitude
// ramp: short-window RMS of a 1 kHz tone ≈ amplitude(window center)/√2.
func TestGlideLinearRamp(t *testing.T) {
	const glide = 1 * time.Second
	g := synth.NewGain(testSR, synth.NewTone(testSR, 1000), 0)
	g.GlideTo(0.5, glide)
	l, _ := deinterleave(synth.Render(g, testSR*1.05))

	const win = testSR / 20   // 50 ms windows
	for w := 2; w < 20; w++ { // skip the first windows: level ≈ 0 there
		center := (float64(w) + 0.5) * 0.05
		wantAmp := 0.5 * center
		want := wantAmp / math.Sqrt2
		got := dsp.RMS(l[w*win : (w+1)*win])
		if math.Abs(got-want) > 0.12*want {
			t.Fatalf("window %d (t≈%.2fs): RMS = %.4f, want %.4f ± 12%%", w, center, got, want)
		}
	}
}

func TestGainSetImmediate(t *testing.T) {
	g := synth.NewGain(testSR, synth.NewTone(testSR, 1000), 0.25)
	if a := g.Amplitude(); a != 0.25 {
		t.Fatalf("initial amplitude = %g, want 0.25", a)
	}
	g.Set(0.8)
	if a := g.Amplitude(); a != 0.8 {
		t.Fatalf("after Set = %g, want 0.8", a)
	}
	g.GlideTo(0.4, 100*time.Millisecond)
	if a := g.Amplitude(); a != 0.8 {
		t.Fatalf("glide start amplitude = %g, want 0.8 (ramp starts from current level)", a)
	}
	l, _ := g.Next()
	l, _ = g.Next() // advance into the ramp (first sample is still `from`)
	_ = l
	if a := g.Amplitude(); a >= 0.8 {
		t.Fatalf("mid-glide amplitude = %g, want < 0.8", a)
	}
}

// TestCrossfadeConstantPower: equal-power curves keep total loudness flat
// across a fade between two uncorrelated (white noise) voices.
func TestCrossfadeConstantPower(t *testing.T) {
	a := synth.NewNoise(testSR, synth.White, 11)
	b := synth.NewNoise(testSR, synth.White, 22)
	cf := synth.NewCrossfade(testSR, a, b, 500*time.Millisecond)
	l, _ := deinterleave(synth.Render(cf, testSR*1.5))

	windows := []struct {
		name     string
		from, to float64 // seconds
	}{
		{"start", 0.00, 0.20},
		{"mid-fade", 0.65, 0.85},
		{"after", 1.30, 1.50},
	}
	for _, w := range windows {
		from := int(w.from * testSR)
		to := int(w.to * testSR)
		got := dsp.RMS(l[from:to])
		if got < 0.9 || got > 1.1 {
			t.Errorf("%s window RMS = %.3f, want 1.0 ± 0.1", w.name, got)
		}
	}
}
