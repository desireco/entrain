package synth

import "math"

// Osc is a phase-accumulating sine oscillator. Phase advances by freq/sr per
// sample and wraps in [0,1), so frequency may change at any time (including
// mid-render) without a phase jump — the property Phase 2 relies on for tone
// continuity across crossfaded timeline entries.
type Osc struct {
	sr    float64
	phase float64 // cycles, always in [0,1)
	freq  float64
}

// NewOsc creates an oscillator at freq Hz starting at phase 0.
func NewOsc(sr, freq float64) *Osc {
	return &Osc{sr: sr, freq: freq}
}

// SetFreq retunes the oscillator, keeping the accumulated phase intact.
func (o *Osc) SetFreq(freq float64) { o.freq = freq }

// Freq returns the current frequency in Hz.
func (o *Osc) Freq() float64 { return o.freq }

// Phase returns the accumulated phase in cycles, [0,1).
func (o *Osc) Phase() float64 { return o.phase }

// Next returns one sample of sin(2π·phase) and advances the phase.
func (o *Osc) Next() float64 {
	s := math.Sin(2 * math.Pi * o.phase)
	o.phase += o.freq / o.sr
	o.phase -= math.Floor(o.phase)
	return s
}

// Tone is a plain sine at identical level in both ears — SBaGen's
// `carrier/amp` voice with no binaural offset.
type Tone struct{ osc *Osc }

// NewTone creates a mono-perceived tone at freq Hz.
func NewTone(sr, freq float64) *Tone {
	return &Tone{osc: NewOsc(sr, freq)}
}

// SetFreq retunes the tone, phase-continuously.
func (t *Tone) SetFreq(freq float64) { t.osc.SetFreq(freq) }

// Next returns the same sample on both channels.
func (t *Tone) Next() (l, r float64) {
	s := t.osc.Next()
	return s, s
}

// Binaural is a detuned carrier pair: left ear gets carrier−beat/2 and right
// ear carrier+beat/2 (Entrain convention; swapping ears flips the beat's
// perceived direction, not its rate). Both member oscillators retune
// phase-continuously via Set.
type Binaural struct {
	lo, hi *Osc
}

// NewBinaural creates a voice with the given carrier and beat rate in Hz.
func NewBinaural(sr, carrier, beat float64) *Binaural {
	return &Binaural{
		lo: NewOsc(sr, carrier-beat/2),
		hi: NewOsc(sr, carrier+beat/2),
	}
}

// Set retunes carrier and beat without resetting either ear's phase.
func (b *Binaural) Set(carrier, beat float64) {
	b.lo.SetFreq(carrier - beat/2)
	b.hi.SetFreq(carrier + beat/2)
}

// Next returns one sample per ear.
func (b *Binaural) Next() (l, r float64) {
	return b.lo.Next(), b.hi.Next()
}
