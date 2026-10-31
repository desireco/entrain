package synth

import "math"

// Routing voices: per-sample left/right weight control over persistent
// oscillators. FlipBinaural swings a binaural pair between the ears with
// equal-power pendulum weights; EMDR fires short tone bursts alternating
// hard left / hard right. Both keep oscillator phase continuous for their
// whole life — only the routing moves (plus EMDR's burst envelope), so
// renders stay deterministic and glide-friendly.

// FlipBinaural is a binaural pair whose two tones swing between the ears on
// a slow sinusoidal pendulum: source a starts in the left ear and source b
// in the right, and over one full cycle (1/rate seconds) they trade places
// and trade back. Weights follow an equal-power pan law — at the extremes
// the mix is ordinary binaural, mid-swing each ear carries both tones at
// −3 dB with constant per-ear power. The brief in-ear interference at the
// beat rate during the swap is part of the character, not a defect.
//
// `flip:105+10/25` → NewFlipBinaural(sr, 110, 100, 0.05): the beat percept
// fades and re-forms mirrored every half-cycle.
type FlipBinaural struct {
	sr      float64
	a, b    *Osc // a = the left ear's tone at swing phase 0
	rate    float64
	samples int64
}

// NewFlipBinaural creates a swinging pair: left and right are the per-ear
// frequencies (EarFreqs wiring, so the beat sign decides who starts where),
// rate is full L→R→L cycles per second.
func NewFlipBinaural(sr, left, right, rate float64) *FlipBinaural {
	return &FlipBinaural{
		sr:   sr,
		a:    NewOsc(sr, left),
		b:    NewOsc(sr, right),
		rate: rate,
	}
}

// Next returns one sample frame, advancing the swing.
func (f *FlipBinaural) Next() (l, r float64) {
	t := float64(f.samples) / f.sr
	f.samples++
	// Pendulum phase 0→1→0 with zero velocity at the extremes.
	ph := 0.5 * (1 - math.Cos(2*math.Pi*f.rate*t))
	th := ph * (math.Pi / 2)
	wl := math.Cos(th) // a's weight on the left = b's weight on the right
	wr := math.Sin(th)
	sa, sb := f.a.Next(), f.b.Next()
	return sa*wl + sb*wr, sa*wr + sb*wl
}

// EMDR is a bilateral stimulus voice: short soft-attack tone bursts
// alternate hard left / hard right, one burst per half-cycle (rate Hz = full
// L/R cycles per second; EMDR pacing is typically 0.5–1.5). The inactive
// channel is exactly zero. Burst length is a fixed fraction of the
// half-cycle with a raised-cosine shape, so bursts read as gentle taps
// rather than clicks.
type EMDR struct {
	sr      float64
	osc     *Osc
	rate    float64
	samples int64
}

// emdrBurstFrac is the burst's share of its half-cycle (0.25 → a 125 ms tap
// at the default 1 Hz).
const emdrBurstFrac = 0.25

// NewEMDR creates a bilateral voice: freq is the burst tone in Hz, rate the
// full L→R alternation rate in Hz.
func NewEMDR(sr, freq, rate float64) *EMDR {
	return &EMDR{sr: sr, osc: NewOsc(sr, freq), rate: rate}
}

// Next returns one sample frame of the alternating burst train.
func (e *EMDR) Next() (l, r float64) {
	t := float64(e.samples) / e.sr
	e.samples++
	half := 1 / (2 * e.rate)
	ph := math.Mod(t, half) / half // position within the current half-cycle
	var env float64
	if ph < emdrBurstFrac {
		// Raised cosine hump: smooth attack and decay, zero at both ends.
		env = 0.5 * (1 - math.Cos(2*math.Pi*ph/emdrBurstFrac))
	}
	s := e.osc.Next() * env
	if int(t/half)%2 == 0 {
		return s, 0
	}
	return 0, s
}
