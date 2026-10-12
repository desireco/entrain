package seq_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/desireco/entrain/seq"
)

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func parseFile(t *testing.T, path string) *seq.Sequence {
	t.Helper()
	s, err := seq.Parse(mustRead(t, path))
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return s
}

func TestParseBasic(t *testing.T) {
	s := parseFile(t, "testdata/basic.sbg")

	if len(s.Sets) != 2 {
		t.Fatalf("sets = %v, want focus + all-off", s.SetOrder)
	}
	focus := s.Sets["focus"]
	want := []struct {
		kind          seq.ToneKind
		carrier, beat float64
		sign          int
		amp           float64
	}{
		{seq.Pink, 0, 0, 0, 20},
		{seq.Binaural, 180, 40, 1, 30},
		{seq.Binaural, 320, 24, 1, 25},
	}
	if len(focus.Specs) != len(want) {
		t.Fatalf("focus specs = %d, want %d", len(focus.Specs), len(want))
	}
	for i, w := range want {
		got := focus.Specs[i]
		if got.Kind != w.kind || got.Carrier != w.carrier || got.Beat != w.beat ||
			got.Sign != w.sign || got.Amp != w.amp {
			t.Errorf("spec[%d] = %+v, want %+v", i, got, w)
		}
	}

	// NOW at 0, all-off at +60s
	if len(s.Events) != 2 || s.Events[0].When != 0 || s.Events[1].When != 60*time.Second {
		t.Fatalf("events = %+v", s.Events)
	}
	if s.Events[0].Set != "focus" || s.Events[1].Set != "all-off" {
		t.Fatalf("event sets = %v, %v", s.Events[0].Set, s.Events[1].Set)
	}
	// 180+40/30 is a 40 Hz gamma beat — outside sbagen's typical range, so
	// the linter correctly fires exactly one informational warning.
	if len(s.Warnings) != 1 || !strings.Contains(s.Warnings[0].Msg, "beat 40 Hz") {
		t.Fatalf("warnings = %+v, want exactly the 40 Hz beat lint", s.Warnings)
	}
}

func TestParseClockTimesAndMidnightWrap(t *testing.T) {
	s := parseFile(t, "testdata/clock.sbg")
	if len(s.Events) != 2 {
		t.Fatalf("events = %+v", s.Events)
	}
	if s.Events[0].When != 0 {
		t.Errorf("22:00 should anchor t0, got %v", s.Events[0].When)
	}
	if got, want := s.Events[1].When, 8*time.Hour; got != want {
		t.Errorf("06:00 after 22:00 = %v, want %v (midnight wrap)", got, want)
	}
}

func TestParseUnsupportedVoicesAndOptions(t *testing.T) {
	s := parseFile(t, "testdata/unsupported.sbg")

	if !s.Opts.StartEnd || s.Opts.Mix != "bed.ogg" || s.Opts.FadeMS != "3000" {
		t.Fatalf("opts = %+v", s.Opts)
	}
	if s.Opts.Predef != "" {
		t.Fatalf("-p should be absent, got %q", s.Opts.Predef)
	}

	a := s.Sets["a"]
	kinds := map[seq.ToneKind]int{}
	for _, sp := range a.Specs {
		kinds[sp.Kind]++
	}
	if kinds[seq.Pink] != 1 || kinds[seq.Binaural] != 1 || kinds[seq.Bell] != 1 || kinds[seq.Mix] != 1 {
		t.Errorf("set a kinds = %v", kinds)
	}
	b := s.Sets["b"]
	bk := map[seq.ToneKind]int{}
	for _, sp := range b.Specs {
		bk[sp.Kind]++
	}
	if bk[seq.Spin] != 1 || bk[seq.Binaural] != 1 || bk[seq.Wave] != 1 {
		t.Errorf("set b kinds = %v", bk)
	}
	// 300-6/15: sign −1 → right ear carries the higher tone
	for _, sp := range b.Specs {
		if sp.Kind == seq.Binaural && sp.Sign != -1 {
			t.Errorf("300-6 should parse with Sign=-1, got %d", sp.Sign)
		}
	}

	var warns []string
	for _, w := range s.Warnings {
		warns = append(warns, w.Msg)
	}
	for _, want := range []string{
		"bell voice not supported yet",
		"spin voice not supported yet",
		"wave voices not supported yet",
		`fade spec "==" not supported yet`,
		`fade spec "<>" not supported yet`,
		"'->' slide periods not supported yet",
	} {
		found := false
		for _, w := range warns {
			if strings.Contains(w, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing warning containing %q; got %v", want, warns)
		}
	}

	// Times: NOW=0, then +20s, +40s, +1h (h:mm:ss and h:mm forms)
	for i, want := range []time.Duration{0, 20 * time.Second, 40 * time.Second, time.Hour} {
		if s.Events[i].When != want {
			t.Errorf("event[%d].When = %v, want %v", i, s.Events[i].When, want)
		}
	}
}

func TestParseNestedBlocks(t *testing.T) {
	s := parseFile(t, "testdata/blocks.sbg")
	// h:mm block offsets: burst spans 15 minutes, hour repeats it at +1:00.
	if len(s.Events) != 9 {
		t.Fatalf("events = %d, want 9: %+v", len(s.Events), s.Events)
	}
	wantTimes := []time.Duration{
		0, 5 * time.Minute, 10 * time.Minute, 15 * time.Minute,
		time.Hour, time.Hour + 5*time.Minute, time.Hour + 10*time.Minute, time.Hour + 15*time.Minute,
		2 * time.Hour,
	}
	for i, w := range wantTimes {
		if s.Events[i].When != w {
			t.Errorf("event[%d].When = %v, want %v", i, s.Events[i].When, w)
		}
	}
	if s.Events[0].Set != "up" || s.Events[1].Set != "off" || s.Events[8].Set != "off" {
		t.Errorf("block expansion produced wrong sets: %+v", s.Events)
	}
}

func TestParseChakrasShapedNesting(t *testing.T) {
	// The nesting shape of examples/basics/prog-chakras-1.sbg (deeply
	// nested blocks, 3-minute entries) with fresh numbers and names.
	src := `
inner: {
 +0:00 s1
 +0:03 s2
}
outer: {
 +0:00 inner
 +0:06 inner
}
0:00:01 outer
1:00:01 off

s1: 100+1.5/30
s2: 100+0.92/30
off: -
`
	s, err := seq.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	// First event anchors t0 (0:00:01 → 0); block entries keep h:mm
	// minutes: inner spans 0..3m, outer repeats at +6m; off at 1h.
	if len(s.Events) != 5 {
		t.Fatalf("events = %d, want 5: %+v", len(s.Events), s.Events)
	}
	exp := []time.Duration{0, 3 * time.Minute, 6 * time.Minute, 9 * time.Minute, time.Hour}
	for i, w := range exp {
		if s.Events[i].When != w {
			t.Errorf("event[%d].When = %v, want %v", i, s.Events[i].When, w)
		}
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name, src, wantErr string
	}{
		{"bad spec", "a: 100+/10\nNOW a", "unrecognized"},
		{"unknown set", "NOW nosuch", "unknown tone-set"},
		{"backwards", "a: -\nNOW a\n+00:01 a\n+00:00 a", "backwards"},
		{"relative first", "+00:10 a\na: -", "before any absolute"},
		{"unclosed block", "b: {\n +0:00 a", "not closed"},
		{"absolute in block", "b: {\n 1:00 a", "must be relative"},
		{"dup set", "a: -\na: -", "duplicate"},
	}
	for _, tc := range cases {
		_, err := seq.Parse(tc.src)
		if err == nil {
			t.Errorf("%s: expected error containing %q", tc.name, tc.wantErr)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: error %q does not contain %q", tc.name, err.Error(), tc.wantErr)
		}
	}
}

func TestParseFractionalSpecs(t *testing.T) {
	// Shape from prog-drop-old-demo.sbg: fractional carriers, beats, amps.
	s, err := seq.Parse("ts: 204.444+8.57244/1.0 400/1.5\nNOW ts")
	if err != nil {
		t.Fatal(err)
	}
	sp := s.Sets["ts"].Specs
	if sp[0].Carrier != 204.444 || sp[0].Beat != 8.57244 || sp[0].Amp != 1.0 {
		t.Errorf("fractional spec = %+v", sp[0])
	}
	if sp[1].Beat != 0 || sp[1].Carrier != 400 || sp[1].Amp != 1.5 {
		t.Errorf("sine spec = %+v", sp[1])
	}
}

func TestParseBedColors(t *testing.T) {
	s, err := seq.Parse("beds: white/15 pink/20 brown/30 mix/40\nNOW beds")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]seq.ToneKind{"white/15": seq.White, "pink/20": seq.Pink,
		"brown/30": seq.Brown, "mix/40": seq.Mix}
	for _, sp := range s.Sets["beds"].Specs {
		if want[sp.Raw] != sp.Kind {
			t.Errorf("%s parsed as kind %v", sp.Raw, sp.Kind)
		}
	}
}
