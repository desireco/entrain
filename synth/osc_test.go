package synth_test

import (
	"math"
	"testing"

	"github.com/desireco/entrain/internal/dsp"
	"github.com/desireco/entrain/synth"
)

const testSR = 44100

func oscSamples(o *synth.Osc, n int) []float64 {
	x := make([]float64, n)
	for i := range x {
		x[i] = o.Next()
	}
	return x
}

func TestOscFrequency(t *testing.T) {
	n := 44100 // 1 s → 440 Hz lands on exact bin 440
	x := oscSamples(synth.NewOsc(testSR, 440), n)

	if got := dsp.Goertzel(x, testSR, 440); math.Abs(got-float64(n)/2) > 0.005*float64(n)/2 {
		t.Fatalf("peak at 440 Hz = %g, want %g", got, float64(n)/2)
	}

	crossings := 0
	for i := 1; i < n; i++ {
		if (x[i-1] < 0) != (x[i] < 0) {
			crossings++
		}
	}
	if want := 2 * 440; math.Abs(float64(crossings-want)) > 0.02*float64(want) {
		t.Fatalf("zero crossings = %d, want ≈%d", crossings, want)
	}
}

func TestOscRetuneKeepsPhase(t *testing.T) {
	o := synth.NewOsc(testSR, 100)
	for range 1234 { // land phase mid-cycle, far from 0
		o.Next()
	}
	p := o.Phase()
	if p <= 0 || p >= 1 {
		t.Fatalf("phase %g not in (0,1)", p)
	}

	o.SetFreq(50)
	if o.Phase() != p {
		t.Fatalf("SetFreq reset phase: %g → %g", p, o.Phase())
	}

	// The next sample must come from the preserved phase, not from 0.
	if got, want := o.Next(), math.Sin(2*math.Pi*p); math.Abs(got-want) > 1e-12 {
		t.Fatalf("post-retune sample = %g, want sin(2π·%g) = %g", got, p, want)
	}

	// A fresh oscillator at the new frequency starts at phase 0 — that
	// difference is exactly the glitch SetFreq avoids.
	if fresh := synth.NewOsc(testSR, 50); fresh.Phase() == o.Phase() {
		t.Fatal("retuned oscillator phase matches a fresh one; phase was not preserved")
	}
}

func TestToneIdenticalEars(t *testing.T) {
	tone := synth.NewTone(testSR, 300)
	for range 100 {
		l, r := tone.Next()
		if l != r {
			t.Fatal("Tone emits different samples per ear")
		}
	}
}

func TestBinauralEarFrequencies(t *testing.T) {
	n := 88200 // 2 s → integer-Hz bins
	l, r := deinterleave(synth.Render(synth.NewBinaural(testSR, 180, 40), n))

	half := float64(n) / 2
	for _, tc := range []struct {
		ch   []float64
		f    float64
		want float64 // 0 → expect no energy
	}{
		{l, 160, 1}, {l, 200, 0}, // left ear: carrier−beat/2 only
		{r, 200, 1}, {r, 160, 0}, // right ear: carrier+beat/2 only
		{l, 180, 0}, {r, 180, 0}, // nothing at the carrier itself
	} {
		got := dsp.Goertzel(tc.ch, testSR, tc.f)
		if tc.want > 0 && math.Abs(got-half) > 0.005*half {
			t.Fatalf("%v Hz: magnitude %g, want %g", tc.f, got, half)
		}
		if tc.want == 0 && got > 0.002*half {
			t.Fatalf("%v Hz: magnitude %g, want ~0", tc.f, got)
		}
	}
}

func TestBinauralRetune(t *testing.T) {
	n := 88200
	b := synth.NewBinaural(testSR, 180, 40)
	b.Set(320, 24) // move to the focus session's second pair
	l, r := deinterleave(synth.Render(b, n))
	half := float64(n) / 2

	if got := dsp.Goertzel(l, testSR, 308); math.Abs(got-half) > 0.005*half {
		t.Fatalf("L@308 after retune = %g, want %g", got, half)
	}
	if got := dsp.Goertzel(r, testSR, 332); math.Abs(got-half) > 0.005*half {
		t.Fatalf("R@332 after retune = %g, want %g", got, half)
	}
}

func deinterleave(x []float64) (l, r []float64) { return dsp.Deinterleave(x) }
