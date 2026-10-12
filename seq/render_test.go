package seq_test

import (
	"math"
	"testing"
	"time"

	"github.com/desireco/entrain/internal/dsp"
	"github.com/desireco/entrain/seq"
)

// window extracts one channel over [from, to) seconds of a render.
func window(x []float64, rate, from, to int) []float64 {
	l, _ := dsp.Deinterleave(x)
	return l[from*rate : to*rate]
}

func TestRenderFocusSessionSpectrum(t *testing.T) {
	s, err := seq.Parse(mustRead(t, "testdata/basic.sbg"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := seq.BuildPlan(s, seq.PlanOpts{Crossfade: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	const rate = 44100
	x := p.Render(rate)
	if got := len(x) / 2 / rate; got != 60 {
		t.Fatalf("render = %ds, want 60s", got)
	}

	// Full-amplitude window away from both fades; 26 s of integer-Hz bins.
	// '+' specs put the higher tone on the left ear (sbagen convention):
	// L = 200 + 332, R = 160 + 308.
	l, r := dsp.Deinterleave(x)
	wl, wr := l[2*rate:28*rate], r[2*rate:28*rate]
	n := float64(len(wl))
	half := n / 2
	for _, tc := range []struct {
		ch   []float64
		f    float64
		amp  float64
		note string
	}{
		{wl, 200, 0.30, "L 200 (gamma high)"},
		{wl, 332, 0.25, "L 332 (beta high)"},
		{wr, 160, 0.30, "R 160 (gamma low)"},
		{wr, 308, 0.25, "R 308 (beta low)"},
	} {
		got := dsp.Goertzel(tc.ch, rate, tc.f)
		if math.Abs(got-tc.amp*half) > 0.03*tc.amp*half {
			t.Errorf("%s: magnitude = %.0f, want %.0f ± 3%%", tc.note, got, tc.amp*half)
		}
	}
	// Wrong-ear tones absent
	for _, f := range []float64{160, 308} {
		if got := dsp.Goertzel(wl, rate, f); got > 0.02*half {
			t.Errorf("L has energy at %g Hz: %.0f", f, got)
		}
	}

	// Pink bed calibrated to sbagen loudness: total L RMS ≈ tones + bed.
	if r := dsp.RMS(wl); r < 0.25 || r > 0.31 {
		t.Errorf("L RMS = %.4f, want ≈0.2776 (tones + calibrated bed)", r)
	}
}

func TestRenderContinuityNoDropout(t *testing.T) {
	// Pink-only sets: the bed continues across the changeover with a pure
	// amplitude glide — no fade to zero, no restart.
	src := `
a: pink/40
b: pink/55
off: -

NOW a
+00:00:10 b
+00:00:20 off
`
	s, err := seq.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	p, err := seq.BuildPlan(s, seq.PlanOpts{Crossfade: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	const rate = 44100
	l := window(p.Render(rate), rate, 0, 20)

	before := dsp.RMS(l[5*rate : 8*rate])          // pink at the 0.40 level
	during := dsp.RMS(l[95*rate/10 : 105*rate/10]) // across the 10s glide
	after := dsp.RMS(l[12*rate : 15*rate])         // pink at the 0.55 level

	wantBefore, wantAfter := 0.40*0.155, 0.55*0.155
	if math.Abs(before-wantBefore) > 0.15*wantBefore || math.Abs(after-wantAfter) > 0.15*wantAfter {
		t.Fatalf("levels: before=%.4f (want %.4f), after=%.4f (want %.4f)", before, wantBefore, after, wantAfter)
	}
	// Through the glide the bed never drops below the pre-glide level
	// (the amplitude only rises) and stays well above zero.
	if during < before*0.95 {
		t.Errorf("dropout at the changeover: during=%.4f before=%.4f", during, before)
	}
	if during > after*1.05 {
		t.Errorf("glide overshot: during=%.4f after=%.4f", during, after)
	}
}

func TestRenderDeterministic(t *testing.T) {
	s, err := seq.Parse(mustRead(t, "testdata/basic.sbg"))
	if err != nil {
		t.Fatal(err)
	}
	p, _ := seq.BuildPlan(s, seq.PlanOpts{LengthLimit: 5 * time.Second})
	a, b := p.Render(44100), p.Render(44100)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("renders diverged at frame %d", i/2)
		}
	}
}

func TestRender48kSmoke(t *testing.T) {
	s, err := seq.Parse(mustRead(t, "testdata/basic.sbg"))
	if err != nil {
		t.Fatal(err)
	}
	p, _ := seq.BuildPlan(s, seq.PlanOpts{Crossfade: time.Second})
	const rate = 48000
	wl := window(p.Render(rate), rate, 2, 28)
	half := float64(len(wl)) / 2
	if got := dsp.Goertzel(wl, rate, 200); math.Abs(got-0.30*half) > 0.03*0.30*half {
		t.Errorf("48k L@200 = %.0f, want %.0f ± 3%%", got, 0.30*half)
	}
}

func TestRenderBrownBedContinuity(t *testing.T) {
	// brown/30 behaves like pink: bed glides amplitude across changeovers
	// with no dropout, at the shared sbagen-calibrated loudness.
	src := `
a: brown/30
b: brown/50
off: -

NOW a
+00:00:10 b
+00:00:20 off
`
	s, err := seq.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	p, err := seq.BuildPlan(s, seq.PlanOpts{Crossfade: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	const rate = 44100
	l := window(p.Render(rate), rate, 0, 20)
	before := dsp.RMS(l[5*rate : 8*rate])
	during := dsp.RMS(l[95*rate/10 : 105*rate/10])
	after := dsp.RMS(l[12*rate : 15*rate])
	if math.Abs(before-0.30*0.155) > 0.15*0.30*0.155 || math.Abs(after-0.50*0.155) > 0.15*0.50*0.155 {
		t.Fatalf("levels: before=%.4f after=%.4f", before, after)
	}
	if during < before*0.9 {
		t.Errorf("dropout at changeover: during=%.4f before=%.4f", during, before)
	}
}
