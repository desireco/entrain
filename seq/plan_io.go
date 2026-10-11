package seq

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// WritePlan prints the human-readable render plan: the timeline with each
// event's tone-set. Lint warnings are not included — they flow to stderr
// via Plan.Warnings.
func (p *Plan) WritePlan(w io.Writer) {
	for _, d := range p.Seq.Description {
		fmt.Fprintln(w, strings.TrimRight(d, " "))
	}
	if len(p.Seq.Description) > 0 {
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "plan: %s → %s (%s), %d event(s), %d voice(s), crossfade %s\n\n",
		FormatDur(p.Start), FormatDur(p.End), FormatDur(p.End-p.Start),
		len(p.Events), len(p.Voices), FormatDur(p.Crossfade))

	fmt.Fprintln(w, "  time      fade  tone-set")
	for _, ev := range p.Events {
		ts, ok := p.Seq.Sets[ev.Set]
		specs := ""
		if ok {
			parts := make([]string, len(ts.Specs))
			for i, sp := range ts.Specs {
				parts[i] = sp.Raw
			}
			specs = strings.Join(parts, " ")
		} else if ev.Set == "" {
			specs = "(end of output)"
		} else {
			specs = "(unknown)"
		}
		fmt.Fprintf(w, "  %-9s %c%c    %-12s %s\n", FormatDur(ev.When),
			ev.FadeIn, ev.FadeOut, ev.Set, specs)
	}
}

// ParseLength parses a -L-style length (`h:mm` or `h:mm:ss`).
func ParseLength(s string) (time.Duration, error) {
	d, err := parseClock(s)
	if err != nil {
		return 0, fmt.Errorf("seq: bad length %q (want h:mm or h:mm:ss)", s)
	}
	return d, nil
}
