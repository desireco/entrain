package seq_test

import (
	"testing"
	"time"

	"github.com/desireco/entrain/seq"
)

func planOf(t *testing.T, src string, opts seq.PlanOpts) *seq.Plan {
	t.Helper()
	s, err := seq.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	p, err := seq.BuildPlan(s, opts)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func voice(p *seq.Plan, key string) *seq.VoicePlan {
	for i := range p.Voices {
		if p.Voices[i].Key == key {
			return &p.Voices[i]
		}
	}
	return nil
}

func TestPlanBasicEnvelopes(t *testing.T) {
	p := planOf(t, mustRead(t, "testdata/basic.sbg"), seq.PlanOpts{})

	if p.Start != 0 || p.End != 60*time.Second {
		t.Fatalf("span = [%v, %v], want [0, 60s]", p.Start, p.End)
	}
	if p.Crossfade != seq.DefaultCrossfade {
		t.Fatalf("crossfade = %v, want default 30s", p.Crossfade)
	}

	// 180+40/30 → ears 200/160; fades in over 30s, out over the last 30s.
	v := voice(p, "bin:200/160")
	if v == nil {
		t.Fatalf("voices: %+v", p.Voices)
	}
	for _, tc := range []struct {
		t    time.Duration
		want float64
	}{
		{0, 0}, {15 * time.Second, 0.15}, {30 * time.Second, 0.30},
		{45 * time.Second, 0.15}, {60 * time.Second, 0},
	} {
		if got := v.AmpAt(tc.t); abs(got-tc.want) > 1e-9 {
			t.Errorf("amp(%v) = %g, want %g", tc.t, got, tc.want)
		}
	}

	// pink/20 → sbagen-calibrated bed level, same fade shape
	pk := voice(p, "pink")
	want := 0.20 * 0.155
	if got := pk.AmpAt(30 * time.Second); abs(got-want) > 1e-9 {
		t.Errorf("pink amp(30s) = %g, want %g", got, want)
	}
}

func TestPlanContinuityGlide(t *testing.T) {
	src := `
a: pink/40 100+4/20
b: pink/55 100+6/25
off: -

NOW a
+00:02:00 b
+00:04:00 off
`
	p := planOf(t, src, seq.PlanOpts{})

	// pink continues across the 2:00 changeover: amplitude glides
	// 0.40→0.55 over [1:30, 2:30], never dropping to 0.
	pk := voice(p, "pink")
	if pk == nil {
		t.Fatal("no pink voice")
	}
	for _, tc := range []struct {
		t    time.Duration
		want float64
	}{
		{90 * time.Second, 0.40 * 0.155},
		{120 * time.Second, (0.40 + 0.55) / 2 * 0.155},
		{150 * time.Second, 0.55 * 0.155},
	} {
		if got := pk.AmpAt(tc.t); abs(got-tc.want) > 1e-9 {
			t.Errorf("pink amp(%v) = %g, want %g", tc.t, got, tc.want)
		}
	}

	// 100+4 (ears 102/98) fades out before 2:00; 100+6 (103/97) fades in
	// after — different frequencies, so no glide.
	old, newv := voice(p, "bin:102/98"), voice(p, "bin:103/97")
	if old == nil || newv == nil {
		t.Fatalf("voices: %+v", p.Voices)
	}
	if got := old.AmpAt(120 * time.Second); got != 0 {
		t.Errorf("102/98 should be silent at the 2:00 boundary, amp = %g", got)
	}
	if got := newv.AmpAt(120 * time.Second); got != 0 {
		t.Errorf("103/97 should be silent at the 2:00 boundary, amp = %g", got)
	}
	if got := old.AmpAt(100 * time.Second); abs(got-0.20*2.0/3.0) > 1e-9 {
		t.Errorf("102/98 amp(1:40) = %g, want %.4f (mid fade-out over [1:30, 2:00])", got, 0.20*2.0/3.0)
	}
	if got := newv.AmpAt(140 * time.Second); abs(got-0.25*2.0/3.0) > 1e-9 {
		t.Errorf("103/97 amp(2:20) = %g, want %.4f (mid fade-in over [2:00, 2:30])", got, 0.25*2.0/3.0)
	}
}

func TestPlanShortSegmentsClampFades(t *testing.T) {
	// 20 s segments with a 30 s default crossfade: fades clamp to the
	// segment length.
	src := `
a: 100+4/20
b: 100+6/25
off: -

NOW a
+00:00:20 b
+00:00:40 off
`
	p := planOf(t, src, seq.PlanOpts{})
	if p.End != 40*time.Second {
		t.Fatalf("end = %v", p.End)
	}
	v := voice(p, "bin:102/98")
	// 20 s segments with a 30 s crossfade: each fade clamps to half the
	// segment, so the voice peaks at 0.20 mid-segment.
	for _, tc := range []struct {
		t    time.Duration
		want float64
	}{
		{0, 0}, {10 * time.Second, 0.20}, {20 * time.Second, 0}, {40 * time.Second, 0},
	} {
		if got := v.AmpAt(tc.t); abs(got-tc.want) > 1e-9 {
			t.Errorf("amp(%v) = %g, want %g", tc.t, got, tc.want)
		}
	}
}

func TestPlanLengthLimit(t *testing.T) {
	p := planOf(t, mustRead(t, "testdata/basic.sbg"), seq.PlanOpts{LengthLimit: 45 * time.Second})
	if p.End != 45*time.Second {
		t.Fatalf("end = %v, want 45s (limit < natural span)", p.End)
	}
	v := voice(p, "bin:200/160")
	if got := v.AmpAt(45 * time.Second); got != 0 {
		t.Errorf("limit cut should fade to silence by the end, amp = %g", got)
	}
	// The cut at 45s fades out over [22.5s, 45s] (clamped to half the
	// remaining segment), so 30s is already partway down.
	if got := v.AmpAt(30 * time.Second); abs(got-0.20) > 1e-9 {
		t.Errorf("amp(30s) = %g, want 0.20 (mid fade-out to the cut)", got)
	}
}

func TestPlanSingleEventOpenEnded(t *testing.T) {
	s, err := seq.Parse("ts: 200+10/40\nNOW ts")
	if err != nil {
		t.Fatal(err)
	}
	// No limit: open-ended (sbagen plays until stopped) — plannable.
	p, err := seq.BuildPlan(s, seq.PlanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !p.OpenEnded {
		t.Fatal("single event without -L should be open-ended")
	}

	// With a limit it renders: fade in, hold, fade out.
	p, err = seq.BuildPlan(s, seq.PlanOpts{LengthLimit: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	v := voice(p, "bin:205/195")
	if got := v.AmpAt(30 * time.Second); abs(got-0.40) > 1e-9 {
		t.Errorf("mid amp = %g, want 0.40", got)
	}
	if got := v.AmpAt(time.Minute); got != 0 {
		t.Errorf("end amp = %g, want 0", got)
	}
}

func TestPlanDedupesSameTimeEvents(t *testing.T) {
	src := `
a: 100+4/20
b: 200+6/25
off: -

NOW a
+00:01:00 b
+00:01:00 off
`
	p := planOf(t, src, seq.PlanOpts{})
	if len(p.Events) != 2 {
		t.Fatalf("events = %+v, want the later same-time event to win", p.Events)
	}
	if p.Events[1].Set != "off" {
		t.Errorf("kept set %q, want off", p.Events[1].Set)
	}
	found := false
	for _, w := range p.Warnings {
		if len(w.Msg) > 0 && w.Msg[0] == 't' { // "two events at …"
			found = true
		}
	}
	if !found {
		t.Errorf("missing same-time warning: %+v", p.Warnings)
	}
}

func TestPlanHonorsFileFadeOption(t *testing.T) {
	src := "-F 3000\n\na: 100+4/20\noff: -\n\nNOW a\n+00:01:00 off\n"
	p := planOf(t, src, seq.PlanOpts{})
	if p.Crossfade != 3*time.Second {
		t.Fatalf("crossfade = %v, want 3s from -F", p.Crossfade)
	}
}

func TestPlanEarFreqsBySign(t *testing.T) {
	src := `
plus: 100+4/20
minus: 100-4/20
off: -

NOW plus
+00:00:10 minus
+00:00:20 off
`
	p := planOf(t, src, seq.PlanOpts{})
	if voice(p, "bin:102/98") == nil || voice(p, "bin:98/102") == nil {
		t.Fatalf("'+' and '-' must produce distinct per-ear components: %+v", p.Voices)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
