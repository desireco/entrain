package seq_test

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/desireco/entrain/seq"
)

func TestEMDRShapeRenders(t *testing.T) {
	const sr = 8000
	cache := map[string][]float64{}
	for _, shape := range []string{"", "tone", "bell", "noise"} {
		attr := ""
		if shape != "" {
			attr = "shape=" + shape
		}
		src := fmt.Sprintf("## extended syntax: entrain\n-changeover 0.01\nt: emdr:420/30 %s rate1\noff: -\nNOW t\n+0:00:03 off\n", attr)
		p := planOf(t, src, seq.PlanOpts{})
		x := p.Render(sr)
		cache[shape] = x
		// The frame immediately before each shaped release reaches silence
		// should be small as well; zero on the next frame alone can hide a click.
		if release := map[string]int{"bell": 340 * sr / 1000, "noise": 170 * sr / 1000}[shape]; release > 0 {
			if math.Abs(x[2*(release-1)]) > 1e-4 || x[2*release] != 0 {
				t.Errorf("%q: abrupt cue release (%g, %g)", shape, x[2*(release-1)], x[2*release])
			}
		}
		if !slices.Equal(x, p.Render(sr)) {
			t.Fatalf("%q: renders differ", shape)
		}
		// Check four consecutive half-cycles away from the timeline end.
		for half := range 4 {
			active := half % 2
			var peak float64
			for f := range sr / 2 {
				i := 2 * (half*sr/2 + f)
				cue := x[i+active]
				if x[i+1-active] != 0 || math.IsNaN(cue) || math.Abs(cue) > 0.3000001 {
					t.Fatalf("%q: wrong routing or amplitude at frame %d", shape, i/2)
				}
				peak = max(peak, math.Abs(cue))
				if f == 0 || f >= sr*2/5 {
					if math.Abs(cue) > 1e-10 {
						t.Fatalf("%q: nonzero cue outside burst at frame %d", shape, i/2)
					}
				}
			}
			if peak < 0.02 {
				t.Fatalf("%q: missing cue in half-cycle %d", shape, half)
			}
		}
	}
	if !slices.Equal(cache[""], cache["tone"]) {
		t.Error("explicit tone differs from original EMDR")
	}
	if slices.Equal(cache["bell"], cache["tone"]) || slices.Equal(cache["noise"], cache["tone"]) || slices.Equal(cache["bell"], cache["noise"]) {
		t.Error("expected three distinct timbres")
	}
}

func TestEMDRShapePlanIdentity(t *testing.T) {
	p := planOf(t, "## extended syntax: entrain\na: emdr:420/18 rate0.8\nb: emdr:420/18 shape=tone rate0.8\nc: emdr:420/18 rate0.8 shape=bell\nd: emdr:420/18 shape=noise rate0.8\nNOW a\n+0:00:01 b\n+0:00:02 c\n+0:00:03 d\n", seq.PlanOpts{})
	if len(p.Voices) != 3 {
		t.Fatalf("got %d voices, want three with explicit and implicit tone sharing a voice", len(p.Voices))
	}
	var display strings.Builder
	p.WritePlan(&display)
	for _, attr := range []string{"shape=tone", "shape=bell", "shape=noise"} {
		if !strings.Contains(display.String(), attr) {
			t.Errorf("dry-run lost %s", attr)
		}
	}
	for _, bad := range []string{"emdr:420/18 shape=", "emdr:420/18 shape=invalid", "emdr:420/18 shape=tone shape=bell", "pink/20 shape=noise", "flip:250+10/20 shape=bell"} {
		if _, err := seq.Parse("a: " + bad + "\n"); err == nil {
			t.Errorf("accepted invalid shape syntax: %s", bad)
		}
	}
}
