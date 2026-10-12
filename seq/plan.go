package seq

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/desireco/entrain/internal/dsp"
)

// DefaultCrossfade is sbagen's default: fades happen within 30 s either
// side of a changeover (a 60 s window centered on it).
const DefaultCrossfade = 30 * time.Second

// pinkCalibration scales the pink bed's unit-RMS synth output down to
// sbagen's loudness convention, measured from the vendored binary:
// pink/100 renders at ≈0.155 RMS. Tones need no calibration (amp/100 is
// the sine's peak in both engines).
const pinkCalibration = 0.155

// BP is one envelope breakpoint: the component's amplitude at time T.
// Between breakpoints the amplitude is linearly interpolated.
type BP struct {
	T   time.Duration
	Amp float64
}

// VoicePlan is one continuous audio component threaded through the whole
// render: the tone-set entry that defines it, and its amplitude envelope.
// Components keep their identity (and phase) across changeovers as long as
// the same frequencies appear in consecutive tone-sets — only the
// amplitude glides.
type VoicePlan struct {
	Key     string
	Spec    ToneSpec
	Env     []BP      // sorted by T
	Bed     []float64 // Mix only: loop-prepared interleaved bed audio
	BedRate int       // Mix only: the bed's sample rate
}

// AmpAt evaluates the envelope at t (render amplitude, 0 outside).
func (v *VoicePlan) AmpAt(t time.Duration) float64 {
	bps := v.Env
	if len(bps) == 0 || t < bps[0].T {
		return 0
	}
	for i := 1; i < len(bps); i++ {
		if t < bps[i].T {
			a, b := bps[i-1], bps[i]
			frac := float64(t-a.T) / float64(b.T-a.T)
			return a.Amp + (b.Amp-a.Amp)*frac
		}
	}
	return bps[len(bps)-1].Amp
}

// Plan is a sequence expanded into a concrete render timeline.
type Plan struct {
	Seq        *Sequence
	Start, End time.Duration
	Crossfade  time.Duration
	OpenEnded  bool    // single event, no -L: plannable, not renderable
	Events     []Event // clipped to the render span, for display
	Voices     []VoicePlan
	Warnings   []Warning

	bed     []float64 // the -m soundtrack, loop-prepared
	bedRate int
}

// PlanOpts configures planning. Zero values select the defaults; a zero
// LengthLimit means "render the sequence's natural span".
type PlanOpts struct {
	Crossfade   time.Duration // default DefaultCrossfade (sbagen: 30 s per side)
	LengthLimit time.Duration // cut the render at Start+limit (the -L option)
}

// Plan expands a parsed sequence into renderable voice envelopes.
//
// Semantics (sbagen defaults, per SBAGEN.txt §4.1.3): each changeover at
// time T fades the outgoing tone-set out over the 30 s before T and the
// incoming tone-set in over the 30 s after T — sequential half-fades, not
// an overlapping crossfade. Components whose frequencies continue from one
// set to the next never fade: their amplitude glides across the window.
// The render spans the first event to the last (implicit -S -E).
func BuildPlan(s *Sequence, opts PlanOpts) (*Plan, error) {
	if s.Opts.Predef != "" {
		return nil, fmt.Errorf("seq: -p predefined sequences are not supported (%q)", s.Opts.Predef)
	}
	if len(s.Events) == 0 {
		return nil, fmt.Errorf("seq: sequence has no time-sequence lines")
	}

	plan := &Plan{Seq: s, Start: s.Events[0].When, Crossfade: opts.Crossfade}
	plan.Warnings = append(plan.Warnings, s.Warnings...) // parser lints flow to the CLI
	if plan.Crossfade <= 0 {
		plan.Crossfade = DefaultCrossfade
		if s.Opts.FadeMS != "" {
			if ms, err := parseMillis(s.Opts.FadeMS); err == nil && ms > 0 {
				plan.Crossfade = ms
			}
		}
	}

	// De-duplicate events sharing a time (the later one wins) and clip to
	// the render span.
	events := dedupeEvents(s.Events, &plan.Warnings)
	plan.End = events[len(events)-1].When
	if opts.LengthLimit > 0 {
		limited := plan.Start + opts.LengthLimit
		if plan.End == plan.Start || limited < plan.End {
			plan.End = limited
		}
	}
	// A single event (or a zero-length span) is open-ended: sbagen would
	// play it until stopped. The plan still describes it; only rendering
	// requires a length limit.
	if plan.End <= plan.Start {
		plan.OpenEnded = true
		plan.End = plan.Start
	}
	var clipped []Event
	for _, ev := range events {
		if ev.When <= plan.End {
			clipped = append(clipped, ev)
		}
	}
	plan.Events = clipped
	if len(clipped) < 2 {
		// one segment ending in silence (either at End or, when
		// open-ended, at the same instant — display only)
		clipped = append(clipped, Event{When: plan.End, Set: ""})
	}

	// A mix/amp voice needs the -m bed; load it before envelope building
	// so its component can carry the audio.
	if usesMix(s) {
		bed, bedRate, err := loadBed(s)
		if err != nil {
			return nil, err
		}
		plan.bed, plan.bedRate = bed, bedRate
	}

	buildEnvelopes(plan, clipped)
	if len(plan.Voices) == 0 {
		plan.Warnings = append(plan.Warnings, Warning{Msg: "no renderable voices in this sequence (output is silence)"})
	}
	return plan, nil
}

// usesMix reports whether any tone-set references a mix voice.
func usesMix(s *Sequence) bool {
	for _, name := range s.SetOrder {
		for _, sp := range s.Sets[name].Specs {
			if sp.Kind == Mix {
				return true
			}
		}
	}
	return false
}

// loadBed reads the -m soundtrack (16/24-bit PCM WAV, stereo). Seamless
// looping is the renderer's job (bedPlayer crossfades the wrap point) —
// an improvement over the vendored fork, which stops when the file ends.
func loadBed(s *Sequence) ([]float64, int, error) {
	if s.Opts.Mix == "" {
		return nil, 0, fmt.Errorf("seq: mix/amp voices used but no -m file given")
	}
	candidates := []string{s.Opts.Mix}
	if s.Dir != "" && !filepath.IsAbs(s.Opts.Mix) {
		candidates = append(candidates, filepath.Join(s.Dir, s.Opts.Mix))
	}
	var raw []byte
	var path string
	for _, c := range candidates {
		b, err := os.ReadFile(c)
		if err == nil {
			raw, path = b, c
			break
		}
	}
	if raw == nil {
		return nil, 0, fmt.Errorf("seq: -m bed %s not found (looked in %v)", s.Opts.Mix, candidates)
	}
	sr, x, _, err := dsp.ReadWav(raw)
	if err != nil {
		return nil, 0, fmt.Errorf("seq: -m bed %s: %w", path, err)
	}

	return x, sr, nil
}

// dedupeEvents keeps the last event at each identical time.
func dedupeEvents(events []Event, warns *[]Warning) []Event {
	var out []Event
	for _, ev := range events {
		if len(out) > 0 && out[len(out)-1].When == ev.When {
			*warns = append(*warns, Warning{Line: ev.Line,
				Msg: fmt.Sprintf("two events at %s; using the later one", FormatDur(ev.When))})
			out[len(out)-1] = ev
			continue
		}
		out = append(out, ev)
	}
	return out
}

// buildEnvelopes threads each distinct component through the clipped
// event list, building its amplitude envelope.
func buildEnvelopes(plan *Plan, events []Event) {
	F := plan.Crossfade

	type comp struct {
		spec ToneSpec
		env  []BP
	}
	comps := map[string]*comp{}
	var order []string

	ampOf := func(ev Event, key string) (float64, bool) {
		ts, ok := plan.Seq.Sets[ev.Set]
		if !ok {
			return 0, false
		}
		for _, sp := range ts.Specs {
			if keyOf(sp) == key {
				return renderAmp(sp), true
			}
		}
		return 0, true
	}

	segStart := func(i int) time.Duration { return events[i].When }
	segLen := func(i int) time.Duration {
		if i+1 < len(events) {
			return events[i+1].When - events[i].When
		}
		return plan.End - events[i].When
	}

	// Collect components and their per-segment amplitudes.
	for i := range events {
		ts, ok := plan.Seq.Sets[events[i].Set]
		if !ok {
			continue
		}
		for _, sp := range ts.Specs {
			if sp.Kind != White && sp.Kind != Pink && sp.Kind != Brown && sp.Kind != Binaural && sp.Kind != Mix {
				continue
			}
			k := keyOf(sp)
			if _, seen := comps[k]; !seen {
				comps[k] = &comp{spec: sp}
				order = append(order, k)
			}
		}
	}

	for _, k := range order {
		c := comps[k]

		// Fade in from silence at the render start (clamped to half the
		// first segment so it cannot collide with the first fade-out).
		a0, _ := ampOf(events[0], k)
		if a0 > 0 {
			fIn := minDur(F, segLen(0)/2)
			c.env = append(c.env, BP{plan.Start, 0}, BP{plan.Start + fIn, a0})
		}

		for i := 0; i+1 < len(events); i++ {
			T := events[i+1].When // changeover time
			a, _ := ampOf(events[i], k)
			b, _ := ampOf(events[i+1], k)
			// Each fade clamps to half its segment, so a segment shorter
			// than 2×F peaks mid-segment instead of the fades colliding.
			fOut := minDur(F, (T-segStart(i))/2)
			fIn := minDur(F, segLen(i+1)/2)
			switch {
			case a > 0 && b > 0: // continuity: amplitude glides across the window
				c.env = append(c.env, BP{T - fOut, a}, BP{T + fIn, b})
			case a > 0: // fades out before the boundary
				c.env = append(c.env, BP{T - fOut, a}, BP{T, 0})
			case b > 0: // fades in after the boundary
				c.env = append(c.env, BP{T, 0}, BP{T + fIn, b})
			}
		}

		c.env = normalizeEnv(c.env, plan.Start, plan.End)
		vp := VoicePlan{Key: k, Spec: c.spec, Env: c.env}
		if c.spec.Kind == Mix {
			vp.Bed, vp.BedRate = plan.bed, plan.bedRate
		}
		plan.Voices = append(plan.Voices, vp)
	}
}

// normalizeEnv sorts breakpoints, drops redundant ones at equal times, and
// clamps envelope times into the render span.
func normalizeEnv(env []BP, start, end time.Duration) []BP {
	sort.SliceStable(env, func(i, j int) bool { return env[i].T < env[j].T })
	out := env[:0]
	for _, bp := range env {
		if bp.T < start {
			bp.T = start
		}
		if bp.T > end {
			bp.T = end
		}
		if len(out) > 0 && out[len(out)-1].T == bp.T {
			out[len(out)-1] = bp // later value wins at identical times
			continue
		}
		out = append(out, bp)
	}
	if len(out) > 0 && out[0].T > start && out[0].Amp != 0 {
		out = append([]BP{{start, 0}}, out...)
	}
	return out
}

// keyOf identifies a component across tone-sets: pink noise, or a
// binaural/sine voice by its per-ear frequencies (so '+'/'-' signs with
// the same carrier and beat are different components, and a beat change
// is a fade, not a glide — frequency slides are the later '=' work).
func keyOf(sp ToneSpec) string {
	switch sp.Kind {
	case White:
		return "white"
	case Pink:
		return "pink"
	case Brown:
		return "brown"
	case Mix:
		return "mix"
	}
	l, r := EarFreqs(sp)
	return fmt.Sprintf("bin:%.6g/%.6g", l, r)
}

// EarFreqs returns the (left, right) tone frequencies of a binaural spec:
// the beat sits equally either side of the carrier, and the sign picks
// which ear carries the higher tone ('+': left).
func EarFreqs(sp ToneSpec) (left, right float64) {
	half := sp.Beat / 2
	if sp.Sign >= 0 {
		return sp.Carrier + half, sp.Carrier - half
	}
	return sp.Carrier - half, sp.Carrier + half
}

// renderAmp converts a spec's 0–100 amplitude to render amplitude: tones
// at amp/100 peak (sbagen convention), pink scaled to sbagen's measured
// bed loudness.
func renderAmp(sp ToneSpec) float64 {
	if sp.Kind == White || sp.Kind == Pink || sp.Kind == Brown {
		return sp.Amp / 100 * pinkCalibration
	}
	return sp.Amp / 100
}

// FormatDur renders a duration as h:mm:ss (the .sbg convention).
func FormatDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int64(math.Round(d.Seconds()))
	return fmt.Sprintf("%d:%02d:%02d", total/3600, total%3600/60, total%60)
}

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func parseMillis(s string) (time.Duration, error) {
	var ms float64
	if _, err := fmt.Sscanf(s, "%g", &ms); err != nil {
		return 0, err
	}
	return time.Duration(ms * float64(time.Millisecond)), nil
}
