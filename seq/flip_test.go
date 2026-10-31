package seq_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/desireco/entrain/internal/dsp"
	"github.com/desireco/entrain/seq"
)

func channel(x []float64, ch int) []float64 {
	out := make([]float64, 0, len(x)/2)
	for i := ch; i < len(x); i += 2 {
		out = append(out, x[i])
	}
	return out
}

func windowOf(x []float64, sr, from, to float64) []float64 {
	return x[int(from*sr):int(to*sr)]
}

func TestParseFlipAndEMDR(t *testing.T) {
	s, err := seq.Parse(`
f: flip:105+10/25 rate0.5
e: emdr:600/20
g: flip:200-6/15
h: flip:200+10/20 rate2
`)
	if err != nil {
		t.Fatal(err)
	}
	f := s.Sets["f"].Specs[0]
	if f.Kind != seq.Flip || f.Carrier != 105 || f.Beat != 10 || f.Amp != 25 || f.Sign != 1 || f.Rate != 0.5 {
		t.Errorf("f spec = %+v", f)
	}
	e := s.Sets["e"].Specs[0]
	if e.Kind != seq.EMDR || e.Carrier != 600 || e.Amp != 20 || e.Rate != 1.0 {
		t.Errorf("e spec (want default rate 1.0) = %+v", e)
	}
	g := s.Sets["g"].Specs[0]
	if g.Kind != seq.Flip || g.Sign != -1 || g.Rate != 0.05 {
		t.Errorf("g spec (want default rate 0.05, minus sign) = %+v", g)
	}
	var rateWarns int
	for _, w := range s.Warnings {
		if strings.Contains(w.Msg, "flip rate") {
			rateWarns++
		}
	}
	if rateWarns != 1 {
		t.Errorf("warnings = %v, want exactly one flip-rate warning", s.Warnings)
	}

	// A rate attribute on a non-flip/emdr spec is a parse error, not a
	// silently ignored token.
	if _, err := seq.Parse("t: pink/30 rate0.5\n\nNOW t\n+0:00:01 off\n"); err == nil {
		t.Error("rate attribute after pink/ should fail to parse")
	}
	// rate0 / negative rates are rejected.
	if _, err := seq.Parse("t: flip:200+10/20 rate0\n\nNOW t\n+0:00:01 off\n"); err == nil {
		t.Error("rate0 should fail to parse")
	}
}

func TestFlipKeyIncludesRate(t *testing.T) {
	p := planOf(t, `
off: -
a: flip:200+10/20 rate0.05
b: flip:200+10/20 rate0.1

NOW a
+0:00:01 b
+0:00:02 off
`, seq.PlanOpts{})
	flips := 0
	for i := range p.Voices {
		if strings.HasPrefix(p.Voices[i].Key, "flip:") {
			flips++
		}
	}
	if flips != 2 {
		t.Errorf("different flip rates must be different voices, got %d", flips)
	}

	// Same rate (explicit or defaulted) continues as one voice.
	p = planOf(t, `
off: -
a: flip:200+10/20
b: flip:200+10/20

NOW a
+0:00:01 b
+0:00:02 off
`, seq.PlanOpts{})
	flips = 0
	for i := range p.Voices {
		if strings.HasPrefix(p.Voices[i].Key, "flip:") {
			flips++
		}
	}
	if flips != 1 {
		t.Errorf("same-rate flips must share one voice, got %d", flips)
	}
}

func TestFlipPlanRendersSwappingEars(t *testing.T) {
	p := planOf(t, `
off: -
f: flip:105+10/25 rate0.5

NOW f
+0:00:02 off
`, seq.PlanOpts{})
	const sr = 8000
	x := p.Render(sr)
	L, R := channel(x, 0), channel(x, 1)

	// Early window: 110 Hz (left tone) dominates left, 100 Hz right.
	eL, eR := windowOf(L, sr, 0, 0.3), windowOf(R, sr, 0, 0.3)
	if hiL, hiR := dsp.Goertzel(eL, sr, 110), dsp.Goertzel(eR, sr, 110); hiL <= 3*hiR {
		t.Errorf("early: left 110 Hz %g should dominate right %g", hiL, hiR)
	}
	if loR, loL := dsp.Goertzel(eR, sr, 100), dsp.Goertzel(eL, sr, 100); loR <= 3*loL {
		t.Errorf("early: right 100 Hz %g should dominate left %g", loR, loL)
	}
	// Window centered on the t=1 s swap: ears have traded places.
	sL, sR := windowOf(L, sr, 0.8, 1.2), windowOf(R, sr, 0.8, 1.2)
	if hiR, hiL := dsp.Goertzel(sR, sr, 110), dsp.Goertzel(sL, sr, 110); hiR <= 5*hiL {
		t.Errorf("swapped: right 110 Hz %g should dominate left %g", hiR, hiL)
	}
	if loL, loR := dsp.Goertzel(sL, sr, 100), dsp.Goertzel(sR, sr, 100); loL <= 5*loR {
		t.Errorf("swapped: left 100 Hz %g should dominate right %g", loL, loR)
	}
}

func TestEMDRPlanRendersAlternatingBursts(t *testing.T) {
	p := planOf(t, `
off: -
e: emdr:600/50 rate2

NOW e
+0:00:01 off
`, seq.PlanOpts{})
	const sr = 8000
	x := p.Render(sr)
	L, R := channel(x, 0), channel(x, 1)

	// First burst [0, 62.5 ms): left only; right exactly zero.
	if got := dsp.RMS(windowOf(R, sr, 0.005, 0.055)); got != 0 {
		t.Errorf("first burst right RMS = %g, want exactly 0", got)
	}
	if got := dsp.RMS(windowOf(L, sr, 0.005, 0.055)); got < 0.001 {
		t.Errorf("first burst left RMS = %g, want an audible burst", got)
	}
	// Second half-cycle: right only; left exactly zero.
	if got := dsp.RMS(windowOf(L, sr, 0.255, 0.305)); got != 0 {
		t.Errorf("second burst left RMS = %g, want exactly 0", got)
	}
	if got := dsp.RMS(windowOf(R, sr, 0.255, 0.305)); got < 0.01 {
		t.Errorf("second burst right RMS = %g, want an audible burst", got)
	}
}

func TestFlipRenderIsDeterministic(t *testing.T) {
	p := planOf(t, `
off: -
f: flip:105+10/25 rate0.5

NOW f
+0:00:02 off
`, seq.PlanOpts{})
	a := p.Render(8000)
	b := p.Render(8000)
	if !reflect.DeepEqual(a, b) {
		t.Error("flip render is not deterministic")
	}
}

func TestParseDoppler(t *testing.T) {
	s, err := seq.Parse(`
d: doppler:250+10/20 rate0.1 depth30
n: doppler:250+5/20
z: doppler:250+5/20 depth0
`)
	if err != nil {
		t.Fatal(err)
	}
	d := s.Sets["d"].Specs[0]
	if d.Kind != seq.Doppler || d.Carrier != 250 || d.Beat != 10 || d.Rate != 0.1 || d.Depth != 30 || d.Sign != 1 {
		t.Errorf("d spec = %+v", d)
	}
	n := s.Sets["n"].Specs[0]
	if n.Rate != 0.05 || n.Depth != 15 {
		t.Errorf("n spec (want defaults rate 0.05, depth 15) = %+v", n)
	}
	z := s.Sets["z"].Specs[0]
	if z.Depth != 0 {
		t.Errorf("explicit depth0 must not be overridden, got %g", z.Depth)
	}
	// A signed depth is not an attribute token; it fails to parse.
	if _, err := seq.Parse("d: doppler:250+10/20 depth-5\n"); err == nil {
		t.Error("depth-5 should fail to parse")
	}
}

func TestDopplerPlanRendersSweep(t *testing.T) {
	// rate 0.5 → swing peaks at t = 0.5 s; depth 600 cents → left tone
	// (110 Hz base) sweeps to 110·2^0.5 ≈ 155.6 Hz and dwells near it.
	src := func(depth string) string {
		return "off: -\nd: doppler:105+10/25 rate0.5 depth" + depth + "\n\nNOW d\n+0:00:02 off\n"
	}
	const sr = 8000
	p0 := planOf(t, src("0"), seq.PlanOpts{})
	p6 := planOf(t, src("600"), seq.PlanOpts{})
	x0, x6 := p0.Render(sr), p6.Render(sr)
	L0, L6 := channel(x0, 0), channel(x6, 0)

	// Window centered on the peak: base 110 Hz is an exact bin here, so the
	// depth-0 render leaks nothing onto the swept frequency.
	w0, w6 := windowOf(L0, sr, 0.4, 0.6), windowOf(L6, sr, 0.4, 0.6)
	g0, g6 := dsp.Goertzel(w0, sr, 153), dsp.Goertzel(w6, sr, 153)
	if g6 <= 5*g0 {
		t.Errorf("swept render: 153 Hz magnitude %g should dominate depth-0 %g", g6, g0)
	}
	// The sweep passes through the base frequency: energy remains at 110 Hz.
	if g := dsp.Goertzel(windowOf(L6, sr, 0.9, 1.1), sr, 110); g <= 0 {
		t.Errorf("base tone missing in swept render: %g", g)
	}
}
