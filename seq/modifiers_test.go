package seq_test

import (
	"testing"
	"time"

	"github.com/desireco/entrain/seq"
)

func TestParseEventModifiers(t *testing.T) {
	s, err := seq.Parse(`
off: -
a: 200+10/20

NOW a fade15 ease-in
+0:00:01 off fade1.5
+0:00:01 off
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Events[0]; got.Fade != 15*time.Second || got.Curve != 'i' {
		t.Errorf("event 0 = %+v, want fade 15s ease-in", got)
	}
	if got := s.Events[1]; got.Fade != 1500*time.Millisecond || got.Curve != 0 {
		t.Errorf("event 1 = %+v, want fade 1.5s linear", got)
	}
	if got := s.Events[2]; got.Fade != -1 || got.Curve != 0 {
		t.Errorf("event 2 = %+v, want defaults", got)
	}
	if _, err := seq.Parse("off: -\na: 200+10/20\n\nNOW a fadeX\n"); err == nil {
		t.Error("fadeX should fail to parse")
	}
}

func TestFadeOverrideChangesBoundary(t *testing.T) {
	mk := func(mod string) *seq.Plan {
		return planOf(t, `
off: -
a: 200+10/20
b: 200+10/40

NOW a
+0:00:30 b `+mod+`
+0:01:00 off
`, seq.PlanOpts{})
	}
	p := mk("fade10")
	v := voice(p, "bin:205/195")
	if v == nil {
		t.Fatalf("no voice: %+v", p.Voices)
	}
	// The a→b glide spans [20s, 40s] with fade10 (clamped free).
	if got := v.AmpAt(19 * time.Second); got != 0.2 {
		t.Errorf("AmpAt(19s) = %g, want still 0.2 (glide not started)", got)
	}
	if got := v.AmpAt(25 * time.Second); abs(got-0.25) > 1e-9 {
		t.Errorf("AmpAt(25s) = %g, want 0.25 (quarter through the glide)", got)
	}
	// Default 30 s crossfade would already be gliding at 19 s.
	pd := mk("")
	vd := voice(pd, "bin:205/195")
	if got := vd.AmpAt(19 * time.Second); got <= 0.2 {
		t.Errorf("default crossfade should have started gliding by 19s, got %g", got)
	}
}

func TestEaseInShapesEntrance(t *testing.T) {
	mk := func(mod string) *seq.Plan {
		return planOf(t, `
off: -
a: pink/20
b: 200+10/40

NOW a
+0:00:30 b `+mod+`
+0:01:00 off
`, seq.PlanOpts{})
	}
	// b is new at 30 s: fade-in [30s, 45s]. A quarter in, ease-in sits at
	// ~6% of full amplitude; linear sits at 25%.
	eased := voice(mk("ease-in"), "bin:205/195")
	linear := voice(mk(""), "bin:205/195")
	if eased == nil || linear == nil {
		t.Fatalf("missing voice: %v / %v", eased, linear)
	}
	ge := eased.AmpAt(33*time.Second + 750*time.Millisecond)
	gl := linear.AmpAt(33*time.Second + 750*time.Millisecond)
	if ge > 0.06 || ge < 0.01 {
		t.Errorf("eased entrance AmpAt = %g, want ~6%% of 0.4", ge)
	}
	if gl < 0.08 || gl > 0.12 {
		t.Errorf("linear entrance AmpAt = %g, want ~25%% of 0.4", gl)
	}
}
