package synth

import (
	"math"
	"math/rand/v2"
)

// NoiseColor selects the noise generator's spectral character.
type NoiseColor uint8

const (
	// White is flat-spectrum noise.
	White NoiseColor = iota
	// Pink falls at −3 dB/octave (power ∝ 1/f).
	Pink
	// Brown falls at −6 dB/octave (power ∝ 1/f²), high-passed at ~20 Hz
	// to bound DC buildup.
	Brown
)

// Noise generates stereo noise whose left and right channels come from fully
// independent streams (decorrelated — no mono image). All colors are
// normalized to unit RMS per channel at amplitude 1, so `pink/20` and
// `brown/20` bed levels are interchangeable in loudness.
//
// With the same seed the output is deterministic; use different seeds for
// independent instances.
type Noise struct {
	l, r *noiseGen
}

// NewNoise creates a stereo noise source of the given color. The seed
// deterministically derives the two per-channel streams.
func NewNoise(sr float64, color NoiseColor, seed uint64) *Noise {
	n := &Noise{
		l: newNoiseGen(sr, color, seed),
		r: newNoiseGen(sr, color, seed^0x5DEECE66D),
	}
	n.warmup()
	return n
}

// Next returns one sample per channel.
func (n *Noise) Next() (l, r float64) {
	return n.l.next(), n.r.next()
}

// warmup settles the pink/brown filters so the first rendered sample is
// already steady-state (no level bump at t=0).
func (n *Noise) warmup() {
	const warmupSamples = 1 << 14
	for range warmupSamples {
		n.l.next()
		n.r.next()
	}
}

type noiseGen struct {
	sr    float64
	color NoiseColor
	rng   *rand.Rand

	// Pink filter state (Paul Kellet's 3-pole economy filter, public
	// music-dsp technique).
	b0, b1, b2 float64

	// Brown leaky-integrator state and output normalization.
	last, a, scale float64
}

// pinkNorm normalizes the Kellet filter to unit RMS for uniform-white input
// in [−1,1] (σ²=1/3). Each stage yᵢ = aᵢyᵢ'+cᵢw has Var = cᵢ²σ²/(1−aᵢ²) and
// pairwise Cov = cᵢcⱼσ²/(1−aᵢaⱼ); summing variances and covariances for the
// published coefficients gives Var = 8.3397·σ², i.e. RMS 1.6672 for σ=1/√3.
const pinkNorm = 1 / 1.6672

// brownCorner is the −3 dB corner, in Hz, of the brown-noise leaky
// integrator.
const brownCorner = 20.0

func newNoiseGen(sr float64, color NoiseColor, seed uint64) *noiseGen {
	g := &noiseGen{
		sr:    sr,
		color: color,
		rng:   rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15)),
	}
	if color == Brown {
		// One-pole leaky integrator: y[n] = a·y[n−1] + w[n]. Above the
		// corner its spectrum is 1/f in amplitude (−6 dB/oct in power).
		g.a = 1 - 2*math.Pi*brownCorner/sr
		// Steady-state Var = σ_w²/(1−a²); scale output to unit variance
		// for σ_w² = 1/3.
		g.scale = math.Sqrt(3 * (1 - g.a*g.a))
	}
	return g
}

// next returns one unit-RMS sample. White: uniform ±√3. Pink: Kellet filter
// × pinkNorm. Brown: leaky integrator × scale.
func (g *noiseGen) next() float64 {
	w := 2*g.rng.Float64() - 1
	switch g.color {
	case White:
		return w * math.Sqrt(3)
	case Pink:
		g.b0 = 0.99765*g.b0 + w*0.0990460
		g.b1 = 0.96300*g.b1 + w*0.2965164
		g.b2 = 0.57000*g.b2 + w*1.0526913
		return (g.b0 + g.b1 + g.b2 + w*0.1848) * pinkNorm
	case Brown:
		g.last = g.a*g.last + w
		return g.last * g.scale
	}
	panic("synth: unknown noise color")
}
