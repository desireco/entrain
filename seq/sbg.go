// Package seq reads SBaGen `.sbg` sequence files and expands them into a
// renderable timeline. This is the v0 frontend (issue #9): the owner's
// existing sessions and the SBaGen example sequences keep working, while
// TOML "trains" remain the planned native authoring format for post-v0.1.
//
// The reader is clean-room: derived from SBaGen's published documentation
// (SBAGEN.txt §4) and example files as behavioral reference — never from
// the C source. Entrain never writes `.sbg`.
package seq

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ToneKind identifies the voice type of a tone-set entry.
type ToneKind uint8

const (
	Silence  ToneKind = iota // channel unused ('-')
	White                    // white-noise bed ('white/<amp>')
	Pink                     // pink-noise bed ('pink/<amp>')
	Brown                    // brown-noise bed ('brown/<amp>')
	Mix                      // soundtrack mix input ('mix/<amp>') — read, not yet rendered
	Binaural                 // '<carrier>[±<beat>]/<amp>' (beat 0 = plain sine)
	Bell                     // 'bell+<freq>/<amp>' — read, not yet rendered
	Spin                     // 'spin:<width>+<beat>/<amp>' — read, not yet rendered
	Wave                     // 'wave<n>:…' — read, not yet rendered
	Flip                     // 'flip:<carrier>±<beat>/<amp>' — entrain extension: hemisphere-flip binaural
	EMDR                     // 'emdr:<freq>/<amp>' — entrain extension: bilateral tone bursts
	Doppler                  // 'doppler:<carrier>±<beat>/<amp>' — entrain extension: cyclic pitch swing
)

// ToneSpec is one entry of a tone-set.
type ToneSpec struct {
	Kind    ToneKind
	Carrier float64 // Binaural; Bell: strike frequency; Spin: delay width in µs
	Beat    float64 // Binaural/Spin
	Sign    int     // Binaural/Flip/Doppler: +1 = '+' (left ear carries the higher tone), −1 = '-', 0 = plain sine
	Rate    float64 // Flip/EMDR/Doppler: modulation rate in Hz (kind default applied by the parser when absent)
	Depth   float64 // Doppler: peak detune in cents
	Amp     float64 // 0–100 scale
	Raw     string  // original spec text, for plan display
}

// ToneSet is a named collection of tone specs (a `<name>: <spec>…` line).
type ToneSet struct {
	Name  string
	Specs []ToneSpec
	Line  int
}

// Event is one resolved timeline entry: tone-set `Set` becomes active at
// `When`, relative to the render start (the first event's time — the
// implicit `-S`).
type Event struct {
	When            time.Duration
	Set             string
	FadeIn, FadeOut byte // fade chars: in '<','-','=' ; out '>','-','=' — default '-','-'
	Slide           bool // '->' present
	Fade            time.Duration // entrain extension: changeover length at this boundary (-1 = default)
	Curve           byte          // entrain extension: 'i' = ease-in, 'o' = ease-out, 0 = linear
	Line            int
}

// Options holds the command-line options embedded at the top of the file.
type Options struct {
	StartEnd bool   // -S -E seen (already the default render behavior)
	Quiet    bool   // -Q
	Mix      string // -m file
	Length   string // -L time, raw text (e.g. "0:30")
	FadeMS   string // -F ms, raw text (fade/crossfade length override)
	Clock    string // -T time, raw text (live-mode start; offline renders anchor at the first event)
	Predef   string // -p <spec…> (unsupported)
}

// Warning is a lint finding: the sequence parses and plans, but something
// deserves the author's attention.
type Warning struct {
	Line int // 0 = whole file
	Msg  string
}

// Sequence is a parsed .sbg file.
type Sequence struct {
	Dir         string   // session file's directory (ParseFile only); -m beds resolve against it
	Description []string // '##' lines, printed on run like sbagen does
	Opts        Options
	Sets        map[string]ToneSet
	SetOrder    []string // definition order, for stable display
	Events      []Event  // expanded, time-ordered timeline
	Warnings    []Warning
}

// day is the modulus for clock times: sequences may cross midnight
// (22:00 → 06:00); each wrap adds one day.
var day = 24 * time.Hour

// Parse reads a .sbg sequence. Syntax errors return an error; anything
// merely unsupported (bell/spin/wave voices, -p options, explicit
// fade/slide specs) parses into the sequence with a Warning instead.
// Use ParseFile when the session references a -m soundtrack, so
// relative paths resolve.
func Parse(src string) (*Sequence, error) {
	p := &parser{seq: &Sequence{Sets: map[string]ToneSet{}}}
	if err := p.run(src); err != nil {
		return nil, fmt.Errorf("seq: %w", err)
	}
	return p.seq, nil
}

// ParseFile reads a .sbg sequence from disk, recording its directory for
// resolving a -m soundtrack bed.
func ParseFile(path string) (*Sequence, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("seq: %w", err)
	}
	s, err := Parse(string(b))
	if err != nil {
		return nil, fmt.Errorf("seq: %s: %w", path, err)
	}
	s.Dir = filepath.Dir(path)
	return s, nil
}

type parser struct {
	seq     *Sequence
	line    int
	pending []Event // top-level timeline entries, pre block-expansion
	blocks  map[string][]Event

	// absolute-time anchors: t0 = the first anchor seen; resolved top-level
	// times are offsets from it. NOW resolves to t0 (an offline render
	// starts at the first event).
	t0Set   bool
	t0      time.Duration
	lastAbs time.Duration // day-time of the last absolute anchor seen
	lastRes time.Duration // last resolved top-level event time
}

func (p *parser) run(src string) error {
	p.blocks = map[string][]Event{}
	var block *blockCtx

	for _, rawLine := range strings.Split(src, "\n") {
		p.line++
		line := strings.TrimSpace(rawLine)

		// '#' starts a comment anywhere in a line (SBAGEN.txt §4.1);
		// full-line '##' comments form the printed description.
		if i := strings.IndexByte(line, '#'); i >= 0 {
			if i == 0 && strings.HasPrefix(line, "##") {
				p.seq.Description = append(p.seq.Description, strings.TrimPrefix(line, "##"))
			}
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}

		if block != nil {
			switch line {
			case "}":
				p.blocks[block.name] = block.lines
				block = nil
			case "::": // optional block terminator; ignored
			default:
				ev, err := p.timelineLine(line, true)
				if err != nil {
					return err
				}
				block.lines = append(block.lines, ev)
			}
			continue
		}

		if strings.HasPrefix(line, "-") {
			p.optionLine(line)
			continue
		}

		// `name:` prefix → tone-set or block definition; time-specs never
		// end their first field with ':'.
		if f := strings.Fields(line); len(f) >= 1 && strings.HasSuffix(f[0], ":") {
			name := strings.TrimSuffix(f[0], ":")
			if !nameRe.MatchString(name) {
				return fmt.Errorf("line %d: bad name %q", p.line, name)
			}
			if len(f) == 2 && f[1] == "{" {
				block = &blockCtx{name: name}
				continue
			}
			ts, err := p.toneSetLine(name, line[len(f[0]):])
			if err != nil {
				return err
			}
			if _, dup := p.seq.Sets[ts.Name]; dup {
				return fmt.Errorf("line %d: duplicate tone-set %q", p.line, ts.Name)
			}
			p.seq.Sets[ts.Name] = ts
			p.seq.SetOrder = append(p.seq.SetOrder, ts.Name)
			continue
		}

		ev, err := p.timelineLine(line, false)
		if err != nil {
			return err
		}
		p.pending = append(p.pending, ev)
	}

	if block != nil {
		return fmt.Errorf("line %d: block %q not closed with '}'", p.line, block.name)
	}

	for _, ev := range p.pending {
		if err := p.expand(&p.seq.Events, ev, 0); err != nil {
			return err
		}
	}
	for i, ev := range p.seq.Events {
		if i > 0 && ev.When < p.seq.Events[i-1].When {
			return fmt.Errorf("line %d: time-spec goes backwards (events must be in order)", ev.Line)
		}
	}
	return nil
}

type blockCtx struct {
	name  string
	lines []Event
}

// expand appends ev to out, expanding block references recursively: a block
// reference contributes its entries at the call-site time plus each entry's
// own relative offset.
func (p *parser) expand(out *[]Event, ev Event, depth int) error {
	if depth > 16 {
		return fmt.Errorf("line %d: block nesting deeper than 16", ev.Line)
	}
	if _, isSet := p.seq.Sets[ev.Set]; isSet {
		*out = append(*out, ev)
		return nil
	}
	if inner, isBlock := p.blocks[ev.Set]; isBlock {
		for _, e := range inner {
			comb := e
			comb.When = ev.When + e.When
			if err := p.expand(out, comb, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("line %d: unknown tone-set or block %q", ev.Line, ev.Set)
}

// timelineLine parses `<time-spec> [<fade>] <name> [->] [fade<dur>]
// [ease-in|ease-out]`. Inside block definitions the time-spec must be a
// pure relative offset. The trailing fade/ease tokens are entrain
// extensions; sbagen-valid lines never carry tokens after the name, so
// they are never consumed from an old file.
func (p *parser) timelineLine(line string, inBlock bool) (Event, error) {
	ev := Event{Line: p.line, FadeIn: '-', FadeOut: '-', Fade: -1}

	fields := strings.Fields(line)
	if fields[len(fields)-1] == "->" {
		ev.Slide = true
		fields = fields[:len(fields)-1]
	}
	for len(fields) > 0 {
		last := fields[len(fields)-1]
		if last == "ease-in" || last == "ease-out" {
			ev.Curve = 'i'
			if last == "ease-out" {
				ev.Curve = 'o'
			}
			fields = fields[:len(fields)-1]
			continue
		}
		if m := fadeAttr.FindStringSubmatch(last); m != nil {
			d, err := parseFadeDur(m[1])
			if err != nil {
				return ev, fmt.Errorf("line %d: bad fade length %q", p.line, last)
			}
			ev.Fade = d
			fields = fields[:len(fields)-1]
			continue
		}
		break
	}
	if len(fields) < 2 || len(fields) > 3 {
		return ev, fmt.Errorf("line %d: expected `<time-spec> [<fade>] <tone-set>`, got %q", p.line, line)
	}

	when, err := p.parseTimeSpec(fields[0], inBlock)
	if err != nil {
		return ev, fmt.Errorf("line %d: %v", p.line, err)
	}
	ev.When = when

	name := fields[len(fields)-1]
	if !nameRe.MatchString(name) {
		return ev, fmt.Errorf("line %d: bad tone-set name %q", p.line, name)
	}
	ev.Set = name

	if len(fields) == 3 {
		f := fields[1]
		if len(f) != 2 || !strings.ContainsAny(f[:1], "<-=") || !strings.ContainsAny(f[1:], ">-=") {
			return ev, fmt.Errorf("line %d: bad fade spec %q", p.line, f)
		}
		ev.FadeIn, ev.FadeOut = f[0], f[1]
		if f != "--" {
			p.warn(p.line, fmt.Sprintf("fade spec %q not supported yet; using default '--' fades", f))
		}
	}
	if ev.Slide {
		p.warn(p.line, "'->' slide periods not supported yet; using default crossfades")
	}
	return ev, nil
}

// parseTimeSpec resolves a time-spec for a top-level entry (anchored
// against t0) or a block entry (pure offset).
func (p *parser) parseTimeSpec(s string, inBlock bool) (time.Duration, error) {
	parts := strings.Split(s, "+")
	rel := parts[0] == ""

	if inBlock {
		if !rel || len(parts) < 2 {
			return 0, fmt.Errorf("block time-specs must be relative (+hh:mm[:ss]), got %q", s)
		}
		return sumOffsets(parts[1:])
	}
	if rel && len(parts) < 2 {
		return 0, fmt.Errorf("empty time-spec %q", s)
	}
	if rel && !p.t0Set {
		return 0, fmt.Errorf("time-spec %q: relative time before any absolute time or NOW", s)
	}

	anchor := p.lastAbs
	if !rel {
		var dayTime time.Duration
		var err error
		switch parts[0] {
		case "NOW":
			dayTime = p.t0 // NOW after an anchor: the render start; as first anchor: 0
		default:
			if dayTime, err = parseClock(parts[0]); err != nil {
				return 0, fmt.Errorf("bad time-spec %q", s)
			}
		}
		if !p.t0Set {
			p.t0Set, p.t0 = true, dayTime
		}
		anchor = dayTime
		p.lastAbs = dayTime
	}

	off, err := sumOffsets(parts[1:])
	if err != nil {
		return 0, err
	}
	res := anchor - p.t0 + off
	if rel {
		if res < p.lastRes {
			return 0, fmt.Errorf("time-spec %q goes backwards (relative times must follow the previous event)", s)
		}
	} else {
		for res < p.lastRes {
			res += day // clock sequences may cross midnight
		}
	}
	p.lastRes = res
	return res, nil
}

// sumOffsets adds chained h:mm[:ss] offset parts (empty list = 0).
func sumOffsets(parts []string) (time.Duration, error) {
	var sum time.Duration
	for _, o := range parts {
		d, err := parseClock(o)
		if err != nil {
			return 0, fmt.Errorf("bad offset +%s", o)
		}
		sum += d
	}
	return sum, nil
}

var clockRe = regexp.MustCompile(`^(\d+):(\d{2})(?::(\d{2}))?$`)

// parseClock parses `h:mm` or `h:mm:ss` into a time-of-day duration.
func parseClock(s string) (time.Duration, error) {
	m := clockRe.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("bad time %q (want h:mm or h:mm:ss)", s)
	}
	h, _ := strconv.Atoi(m[1])
	mm, _ := strconv.Atoi(m[2])
	ss := 0
	if m[3] != "" {
		ss, _ = strconv.Atoi(m[3])
	}
	if mm > 59 || ss > 59 {
		return 0, fmt.Errorf("bad time %q", s)
	}
	return time.Duration(h)*time.Hour + time.Duration(mm)*time.Minute + time.Duration(ss)*time.Second, nil
}

func (p *parser) optionLine(line string) {
	fields := strings.Fields(line)
	for i := 0; i < len(fields); i++ {
		tok := fields[i]
		next := func() string {
			if i+1 < len(fields) {
				i++
				return fields[i]
			}
			return ""
		}
		switch {
		case tok == "-S" || tok == "-E" || tok == "-SE" || tok == "-ES":
			p.seq.Opts.StartEnd = true
		case tok == "-Q":
			p.seq.Opts.Quiet = true
		case tok == "-m":
			p.seq.Opts.Mix = next()
		case tok == "-L":
			p.seq.Opts.Length = next()
		case tok == "-F":
			p.seq.Opts.FadeMS = next()
		case tok == "-T":
			p.seq.Opts.Clock = next()
		case tok == "-p":
			p.seq.Opts.Predef = strings.Join(fields[i+1:], " ")
			p.warn(p.line, "predefined sequences (-p) not supported")
			return
		default:
			// other sbagen flags (-r -b -R -c …) are accepted silently
		}
	}
}

func (p *parser) warn(line int, msg string) {
	p.seq.Warnings = append(p.seq.Warnings, Warning{Line: line, Msg: msg})
}

var (
	nameRe   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
	silence  = regexp.MustCompile(`^-$`)
	bedRe    = regexp.MustCompile(`^(white|pink|brown|mix)/([0-9]*\.?[0-9]+)$`)
	bell     = regexp.MustCompile(`^bell\+([0-9]*\.?[0-9]+)/([0-9]*\.?[0-9]+)$`)
	spin     = regexp.MustCompile(`^spin:([0-9]*\.?[0-9]+)\+([0-9]*\.?[0-9]+)/([0-9]*\.?[0-9]+)$`)
	wave     = regexp.MustCompile(`^wave[0-9]+:.+$`)
	flipRe   = regexp.MustCompile(`^flip:([0-9]*\.?[0-9]+)([+-])([0-9]*\.?[0-9]+)/([0-9]*\.?[0-9]+)$`)
	fadeAttr = regexp.MustCompile(`^fade([0-9:.]+)$`)
	emdrRe   = regexp.MustCompile(`^emdr:([0-9]*\.?[0-9]+)/([0-9]*\.?[0-9]+)$`)
	dopplRe  = regexp.MustCompile(`^doppler:([0-9]*\.?[0-9]+)([+-])([0-9]*\.?[0-9]+)/([0-9]*\.?[0-9]+)$`)
	rateAttr = regexp.MustCompile(`^(rate|depth)([0-9]*\.?[0-9]+)$`)
	binaural = regexp.MustCompile(`^([0-9]*\.?[0-9]+)([+-])([0-9]*\.?[0-9]+)/([0-9]*\.?[0-9]+)$`)
	sine     = regexp.MustCompile(`^([0-9]*\.?[0-9]+)/([0-9]*\.?[0-9]+)$`)
)

// toneSetLine parses the specs following `name:` into a ToneSet, appending
// lint warnings for out-of-range values and unsupported voice kinds.
func (p *parser) toneSetLine(name, rest string) (ToneSet, error) {
	ts := ToneSet{Name: name, Line: p.line}
	fields := strings.Fields(rest)
	for i := 0; i < len(fields); i++ {
		raw := fields[i]
		spec, err := p.parseSpec(raw)
		if err != nil {
			return ts, fmt.Errorf("line %d: %v", p.line, err)
		}
		// Trailing `rate<hz>`/`depth<cents>` attribute tokens attach to the
		// preceding flip/emdr/doppler voice (entrain extension; sbagen-valid
		// syntax is never consumed this way).
		if spec.Kind == Flip || spec.Kind == EMDR || spec.Kind == Doppler {
			sawDepth := false
			for i+1 < len(fields) {
				m := rateAttr.FindStringSubmatch(fields[i+1])
				if m == nil {
					break
				}
				r, err := strconv.ParseFloat(m[2], 64)
				if err != nil || (m[1] == "rate" && r <= 0) || r < 0 {
					return ts, fmt.Errorf("line %d: spec %q: bad %s %q", p.line, raw, m[1], fields[i+1])
				}
				if m[1] == "rate" {
					spec.Rate = r
				} else {
					spec.Depth = r
					sawDepth = true
				}
				i++
			}
			if spec.Rate == 0 {
				spec.Rate = defaultRate(spec.Kind)
			}
			if spec.Kind == Doppler && spec.Depth == 0 && !sawDepth {
				spec.Depth = 15 // cents; a gentle, clearly audible sweep
			}
		}
		ts.Specs = append(ts.Specs, spec)
	}
	if len(ts.Specs) == 0 {
		return ts, fmt.Errorf("line %d: tone-set %s has no specs", p.line, name)
	}
	if len(ts.Specs) > 16 {
		p.warn(p.line, fmt.Sprintf("tone-set %s has %d specs (sbagen channels cap at 16)", name, len(ts.Specs)))
	}
	var sum float64
	for _, sp := range ts.Specs {
		sum += sp.Amp
		if (sp.Kind == Binaural || sp.Kind == Flip || sp.Kind == Doppler) && sp.Beat > 0 {
			if sp.Carrier < 50 || sp.Carrier > 1000 {
				p.warn(p.line, fmt.Sprintf("%s: carrier %.4g Hz outside sbagen's recommended 50–1000 Hz", sp.Raw, sp.Carrier))
			}
			if sp.Beat < 0.5 || sp.Beat > 30 {
				p.warn(p.line, fmt.Sprintf("%s: beat %.4g Hz outside the typical 0.5–30 Hz range", sp.Raw, sp.Beat))
			}
		}
		if sp.Kind == Flip && sp.Rate > 0.5 {
			p.warn(p.line, fmt.Sprintf("%s: flip rate %.4g Hz is above 0.5 — faster swings blur the beat percept", sp.Raw, sp.Rate))
		}
		if sp.Kind == Doppler {
			if sp.Rate > 0.5 {
				p.warn(p.line, fmt.Sprintf("%s: doppler rate %.4g Hz is above 0.5 — vibrato territory, not doppler", sp.Raw, sp.Rate))
			}
			if sp.Depth > 100 {
				p.warn(p.line, fmt.Sprintf("%s: doppler depth %.4g cents is above 100 — a very wide sweep", sp.Raw, sp.Depth))
			}
		}
		if sp.Kind == EMDR && (sp.Rate < 0.1 || sp.Rate > 3) {
			p.warn(p.line, fmt.Sprintf("%s: emdr rate %.4g Hz outside the typical 0.1–3 Hz pacing", sp.Raw, sp.Rate))
		}
	}
	if sum > 100 {
		p.warn(p.line, fmt.Sprintf("tone-set %s: total amplitude %.4g > 100 will clip on output", name, sum))
	}
	return ts, nil
}

func (p *parser) parseSpec(raw string) (ToneSpec, error) {
	spec := ToneSpec{Raw: raw}
	num := func(s string) (float64, error) {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("spec %q: bad number", raw)
		}
		return v, nil
	}
	amp := func(s string) (float64, error) {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("spec %q: bad amplitude", raw)
		}
		return v, nil
	}

	switch {
	case silence.MatchString(raw):
		spec.Kind = Silence
	case strings.HasPrefix(raw, "white/"), strings.HasPrefix(raw, "pink/"),
		strings.HasPrefix(raw, "brown/"), strings.HasPrefix(raw, "mix/"):
		m := bedRe.FindStringSubmatch(raw)
		if m == nil {
			return spec, fmt.Errorf("spec %q: bad noise/mix spec", raw)
		}
		a, err := amp(m[2])
		if err != nil {
			return spec, err
		}
		spec.Amp = a
		switch m[1] {
		case "white":
			spec.Kind = White
		case "brown":
			spec.Kind = Brown
		case "mix":
			spec.Kind = Mix
		default:
			spec.Kind = Pink
		}
	case strings.HasPrefix(raw, "bell+"):
		m := bell.FindStringSubmatch(raw)
		if m == nil {
			return spec, fmt.Errorf("spec %q: bad bell spec", raw)
		}
		f, err := num(m[1])
		if err != nil {
			return spec, err
		}
		a, err := amp(m[2])
		if err != nil {
			return spec, err
		}
		spec.Kind, spec.Carrier, spec.Amp = Bell, f, a
		p.warn(p.line, fmt.Sprintf("%s: bell voice not supported yet; ignored", raw))
	case strings.HasPrefix(raw, "spin:"):
		m := spin.FindStringSubmatch(raw)
		if m == nil {
			return spec, fmt.Errorf("spec %q: bad spin spec", raw)
		}
		w, err := num(m[1])
		if err != nil {
			return spec, err
		}
		b, err := num(m[2])
		if err != nil {
			return spec, err
		}
		a, err := amp(m[3])
		if err != nil {
			return spec, err
		}
		spec.Kind, spec.Carrier, spec.Beat, spec.Amp = Spin, w, b, a
		p.warn(p.line, fmt.Sprintf("%s: spin voice not supported yet; ignored", raw))
	case wave.MatchString(raw):
		spec.Kind = Wave
		p.warn(p.line, fmt.Sprintf("%s: wave voices not supported yet; ignored", raw))
	case strings.HasPrefix(raw, "flip:"):
		m := flipRe.FindStringSubmatch(raw)
		if m == nil {
			return spec, fmt.Errorf("spec %q: bad flip spec (want flip:<carrier>±<beat>/<amp>)", raw)
		}
		c, err := num(m[1])
		if err != nil {
			return spec, err
		}
		b, err := num(m[3])
		if err != nil {
			return spec, err
		}
		a, err := amp(m[4])
		if err != nil {
			return spec, err
		}
		spec.Kind, spec.Carrier, spec.Beat, spec.Amp = Flip, c, b, a
		spec.Sign = 1
		if m[2] == "-" {
			spec.Sign = -1
		}
		return spec, nil
	case strings.HasPrefix(raw, "doppler:"):
		m := dopplRe.FindStringSubmatch(raw)
		if m == nil {
			return spec, fmt.Errorf("spec %q: bad doppler spec (want doppler:<carrier>±<beat>/<amp>)", raw)
		}
		c, err := num(m[1])
		if err != nil {
			return spec, err
		}
		b, err := num(m[3])
		if err != nil {
			return spec, err
		}
		a, err := amp(m[4])
		if err != nil {
			return spec, err
		}
		spec.Kind, spec.Carrier, spec.Beat, spec.Amp = Doppler, c, b, a
		spec.Sign = 1
		if m[2] == "-" {
			spec.Sign = -1
		}
		if spec.Depth == 0 {
			spec.Depth = 15 // cents; a gentle, clearly audible sweep
		}
		return spec, nil
	case strings.HasPrefix(raw, "emdr:"):
		m := emdrRe.FindStringSubmatch(raw)
		if m == nil {
			return spec, fmt.Errorf("spec %q: bad emdr spec (want emdr:<freq>/<amp>)", raw)
		}
		f, err := num(m[1])
		if err != nil {
			return spec, err
		}
		a, err := amp(m[2])
		if err != nil {
			return spec, err
		}
		spec.Kind, spec.Carrier, spec.Amp = EMDR, f, a
		return spec, nil
	default:
		if m := binaural.FindStringSubmatch(raw); m != nil {
			c, err := num(m[1])
			if err != nil {
				return spec, err
			}
			b, err := num(m[3])
			if err != nil {
				return spec, err
			}
			a, err := amp(m[4])
			if err != nil {
				return spec, err
			}
			spec.Kind, spec.Carrier, spec.Beat, spec.Amp = Binaural, c, b, a
			spec.Sign = 1
			if m[2] == "-" {
				spec.Sign = -1
			}
			return spec, nil
		}
		if m := sine.FindStringSubmatch(raw); m != nil {
			c, err := num(m[1])
			if err != nil {
				return spec, err
			}
			a, err := amp(m[2])
			if err != nil {
				return spec, err
			}
			spec.Kind, spec.Carrier, spec.Beat, spec.Amp = Binaural, c, 0, a
			return spec, nil
		}
		return spec, fmt.Errorf("unrecognized tone spec %q", raw)
	}
	return spec, nil
}
