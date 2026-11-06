package synth

import (
	"math"
	"math/rand/v2"
)

// Routing voices: per-sample left/right weight control over persistent
// oscillators. FlipBinaural swings a binaural pair between the ears with
// equal-power pendulum weights; EMDR fires short tone bursts alternating
// hard left / hard right. Both keep a continuous playhead across level changes.

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

// EMDRShape selects the sound of one bilateral cue.
type EMDRShape string

const (
	EMDRTone  EMDRShape = "tone"
	EMDRBell  EMDRShape = "bell"
	EMDRNoise EMDRShape = "noise"
)

// EMDR plays one cue per half-cycle, alternating hard left then right.
// All timbres share that clock. The inactive channel is exactly zero.
type EMDR struct {
	sr, rate float64
	samples  int64
	burst    func(phase, half float64) float64
}

// emdrBurstFrac is the tone burst's share of its half-cycle (0.25 → a
// 125 ms tap at the default 1 Hz).
const emdrBurstFrac = 0.25

// NewEMDR creates the original raised-cosine tone burst.
func NewEMDR(sr, freq, rate float64) *EMDR {
	return NewEMDRWithShape(sr, freq, rate, EMDRTone)
}

// NewEMDRWithShape creates a bilateral voice. For a bell, freq sets the
// fundamental; for a noise burst it controls brightness. Rate is full
// L→R→L cycles per second. The default shape is the original tone.
func NewEMDRWithShape(sr, freq, rate float64, shape EMDRShape) *EMDR {
	e := &EMDR{sr: sr, rate: rate}
	switch shape {
	case EMDRBell:
		e.burst = bellBurst(sr, freq)
	case EMDRNoise:
		e.burst = noiseBurst(sr, freq, rate)
	default:
		osc := NewOsc(sr, freq)
		e.burst = func(ph, half float64) float64 {
			var env float64
			if ph < emdrBurstFrac {
				env = 0.5 * (1 - math.Cos(2*math.Pi*ph/emdrBurstFrac))
			}
			return osc.Next() * env
		}
	}
	return e
}

// A short mallet strike excites a fundamental and two quieter inharmonic
// partials. Drop partials above Nyquist instead of folding them into the bell.
func bellBurst(sr, freq float64) func(float64, float64) float64 {
	type mode struct{ hz, weight float64 }
	var partials []mode
	for _, m := range []mode{{freq, 0.78}, {2.63 * freq, 0.17}, {4.15 * freq, 0.05}} {
		if m.hz > 0 && m.hz < sr/2 {
			partials = append(partials, m)
		}
	}
	return func(ph, half float64) float64 {
		t := ph * half
		duration := min(0.34, 0.72*half)
		if t >= duration {
			return 0
		}
		attack := min(t/min(0.016, duration/4), 1)
		release := min((duration-t)/min(0.028, duration/4), 1)
		env := (0.5 - 0.5*math.Cos(math.Pi*attack)) * (0.5 - 0.5*math.Cos(math.Pi*release))
		env *= math.Exp(-3 * t / duration)
		var sample float64
		for _, partial := range partials {
			sample += partial.weight * math.Sin(2*math.Pi*partial.hz*t)
		}
		return env * sample
	}
}

// Interpolate between seeded random amplitudes for a band-softened puff.
// Smoothstep joins have zero slope; the raised-cosine gate also starts and
// ends at zero. Interpolation stays in [-1,1] at any brightness.
func noiseBurst(sr, freq, rate float64) func(float64, float64) float64 {
	rng := rand.New(rand.NewPCG(math.Float64bits(freq), math.Float64bits(rate)))
	var from, to, pos float64
	step := min(1, 2*freq/sr)
	return func(ph, half float64) float64 {
		pos += step
		if pos >= 1 {
			pos--
			from, to = to, 2*rng.Float64()-1
		}
		t := ph * half
		duration := min(0.17, 0.4*half)
		if t >= duration {
			return 0
		}
		mix := pos * pos * (3 - 2*pos)
		env := 0.5 - 0.5*math.Cos(2*math.Pi*t/duration)
		return (from + (to-from)*mix) * env
	}
}

// Next returns one sample frame of the alternating burst train.
func (e *EMDR) Next() (l, r float64) {
	t := float64(e.samples) / e.sr
	e.samples++
	half := 1 / (2 * e.rate)
	ph := math.Mod(t, half) / half
	s := e.burst(ph, half)
	if int(t/half)%2 == 0 {
		return s, 0
	}
	return 0, s
}

// DopplerBinaural is a binaural pair whose two tones glide up and down
// together on a slow sinusoid — the way a passing vehicle's pitch moves.
// Both ears detune by the same factor, so the beat rate itself stays
// constant while the carrier sweeps ±depth cents around its center. This
// is entrain's native doppler (the the reference library effect, cleaned up), not an
// sbagen construct.
type DopplerBinaural struct {
	sr          float64
	a, b        *Osc
	left, right float64 // base per-ear frequencies
	rate, depth float64 // full cycles/s; peak detune in cents
	samples     int64
}

// NewDopplerBinaural creates a doppler pair: left/right are the per-ear
// frequencies (EarFreqs wiring), rate the swing in full cycles per second,
// depth the peak detune in cents.
func NewDopplerBinaural(sr, left, right, rate, depth float64) *DopplerBinaural {
	return &DopplerBinaural{
		sr: sr, a: NewOsc(sr, left), b: NewOsc(sr, right),
		left: left, right: right, rate: rate, depth: depth,
	}
}

// Next returns one sample frame, advancing the swing.
func (d *DopplerBinaural) Next() (l, r float64) {
	t := float64(d.samples) / d.sr
	d.samples++
	f := math.Pow(2, d.depth/1200*math.Sin(2*math.Pi*d.rate*t))
	d.a.SetFreq(d.left * f)
	d.b.SetFreq(d.right * f)
	return d.a.Next(), d.b.Next()
}
