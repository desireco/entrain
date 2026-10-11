package synth

import (
	"math"
	"time"
)

// Gain wraps a voice with a time-varying amplitude. Amplitude changes are
// click-free by construction: the wrapped voice keeps running (its phase is
// untouched), only the output level moves.
//
// A glide is a linear ramp scheduled with GlideTo; Set jumps immediately
// (fine at segment boundaries where the level is zero).
type Gain struct {
	v  StereoVoice
	sr float64

	from, to float64 // glide endpoints
	pos, dur int64   // glide position and length in samples; dur 0 = static
}

// NewGain wraps v with an initial amplitude.
func NewGain(sr float64, v StereoVoice, amp float64) *Gain {
	return &Gain{v: v, sr: sr, to: amp}
}

// Set jumps to amp immediately.
func (g *Gain) Set(amp float64) {
	g.from, g.to, g.pos, g.dur = amp, amp, 0, 0
}

// GlideTo schedules a linear ramp from the current amplitude to amp over d.
// A non-positive duration behaves like Set.
func (g *Gain) GlideTo(amp float64, d time.Duration) {
	n := int64(d.Seconds() * g.sr)
	if n <= 0 {
		g.Set(amp)
		return
	}
	g.from, g.pos, g.dur = g.Amplitude(), 0, n
	g.to = amp
}

// Amplitude returns the amplitude the next sample will be scaled by.
func (g *Gain) Amplitude() float64 {
	if g.dur == 0 || g.pos >= g.dur {
		return g.to
	}
	return g.from + (g.to-g.from)*float64(g.pos)/float64(g.dur)
}

// Next advances the ramp (if any) and returns one scaled sample frame.
func (g *Gain) Next() (l, r float64) {
	amp := g.Amplitude()
	if g.pos < g.dur {
		g.pos++
	}
	l, r = g.v.Next()
	return l * amp, r * amp
}

// Crossfade is a one-shot equal-power fade from voice a to voice b: the old
// voice's gain follows cos(θ) and the new voice's sin(θ) for θ from 0 to π/2,
// keeping total power roughly constant while both voices sound. After the
// fade completes it delegates to b alone.
//
// Equal-power assumes the two voices are uncorrelated (e.g. two noise beds,
// or tones at different frequencies). For same-frequency continuity across
// timeline entries, Phase 2 instead keeps one voice alive and glides its
// Gain — never this combiner.
type Crossfade struct {
	a, b StereoVoice
	i, n int64
}

// NewCrossfade fades from a to b over dur.
func NewCrossfade(sr float64, a, b StereoVoice, dur time.Duration) *Crossfade {
	return &Crossfade{a: a, b: b, n: int64(dur.Seconds() * sr)}
}

// Next returns one sample frame of the fade, or of b once it is over.
func (c *Crossfade) Next() (l, r float64) {
	if c.n <= 0 || c.i >= c.n {
		return c.b.Next()
	}
	th := math.Pi / 2 * float64(c.i+1) / float64(c.n)
	al, ar := c.a.Next()
	bl, br := c.b.Next()
	c.i++
	return al*math.Cos(th) + bl*math.Sin(th),
		ar*math.Cos(th) + br*math.Sin(th)
}
