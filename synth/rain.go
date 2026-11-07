package synth

import (
	"math"
	"math/rand/v2"
)

// Rain is a continuous stereo shower based on the production rain recipe.
// Two seeded noise streams feed a warm wash, a finer nearby texture, and an
// upper hiss. Filter state and slow weather changes last for the whole take.
type Rain struct {
	pink, white        *Noise
	left, right        rainChannel
	wash, air, texture rainDrift
	whiteLevel         float64
}

// NewRain creates an independent deterministic stream. The supported session
// rates are 44.1 and 48 kHz; all filters and weather times use the given rate.
func NewRain(sr float64) *Rain { return newRain(sr, 0) }

func newRain(sr float64, seed int64) *Rain {
	ch := rainChannel{
		washHP: rainFilter(sr, 125, true), washLP: rainFilter(sr, 1450, false),
		grainHP: rainFilter(sr, 650, true), grainLP: rainFilter(sr, 2700, false),
		airHP: rainFilter(sr, 1000, true), airLP: rainFilter(sr, 4900, false), airTop: rainFilter(sr, 7400, false),
		pressureLP: rainFilter(sr, 2.9, false), outHP: rainFilter(sr, 50, true), outLP: rainFilter(sr, 6600, false),
	}
	return &Rain{
		pink: NewNoise(sr, Pink, streamSeed(1803, seed)), white: NewNoise(sr, White, streamSeed(4911, seed)),
		left: ch, right: ch,
		wash: newRainDrift(sr, 42, 130, streamSeed(741, seed)), air: newRainDrift(sr, 13, 47, streamSeed(2249, seed)),
		texture: newRainDrift(sr, 19, 83, streamSeed(3921, seed)),
		// Equal white-noise power per Hz when the sample rate changes.
		whiteLevel: 0.12 * math.Sqrt(sr/48000),
	}
}

// Next returns one new stereo frame; it neither loops nor depends on duration.
func (r *Rain) Next() (float64, float64) {
	pl, pr := r.pink.Next()
	wl, wr := r.white.Next()
	washGain := 1.18 + 0.18*r.wash.next()
	airGain := 0.76 + 0.22*r.air.next()
	density := 0.63 + 0.11*r.texture.next()
	lw, ld := r.left.layers(pl*0.12, wl*r.whiteLevel, airGain, density)
	rw, rd := r.right.layers(pr*0.12, wr*r.whiteLevel, airGain, density)
	// Only the warm layer has crossfeed; the air and near texture keep
	// each side's independent noise and pressure fluctuations.
	left := r.left.finish(washGain*(0.84*lw+0.16*rw) + ld)
	right := r.right.finish(washGain*(0.84*rw+0.16*lw) + rd)
	return left, right
}

type rainChannel struct {
	washHP, washLP       rainBiquad
	grainHP, grainLP     rainBiquad
	airHP, airLP, airTop rainBiquad
	pressureLP           rainBiquad
	outHP, outLP         rainBiquad
}

func (c *rainChannel) layers(pink, white, airGain, density float64) (wash, detail float64) {
	wash = c.washLP.next(c.washHP.next(pink))
	grains := c.grainLP.next(c.grainHP.next(pink))
	air := c.airTop.next(c.airLP.next(c.airHP.next(white)))
	pressure := c.pressureLP.next(white)
	return wash, airGain*air + grains*(density+0.24*math.Tanh(560*pressure))
}

// Fixed trim matches the level of the six-minute mastered reference and
// keeps previews independent of the selected render length.
const rainMasterGain = 0.5

func (c *rainChannel) finish(x float64) float64 {
	x = rainMasterGain * c.outLP.next(c.outHP.next(x))
	// Leave headroom for a 200% cue, even on an unusually strong random
	// crest. The knee has unity slope and ordinary samples pass unchanged.
	return softCrest(x, 0.24, 0.35)
}

// softCrest retains unity slope at the knee and bounds the maximum sample.
func softCrest(x, knee, ceiling float64) float64 {
	if over := math.Abs(x) - knee; over > 0 {
		return math.Copysign(knee+over/(1+over/(ceiling-knee)), x)
	}
	return x
}

// Second-order Butterworth sections match the recipe's low/high-pass bands.
// The transposed direct form uses two persistent delay values per section.
type rainBiquad struct {
	b0, b1, b2, a1, a2 float64
	z1, z2             float64
}

func rainFilter(sr, freq float64, highpass bool) rainBiquad {
	// Keep both poles stable if a host requests a lower audio sample rate.
	freq = min(freq, 0.45*sr)
	k := math.Tan(math.Pi * freq / sr)
	norm := 1 / (1 + math.Sqrt2*k + k*k)
	f := rainBiquad{a1: 2 * (k*k - 1) * norm, a2: (1 - math.Sqrt2*k + k*k) * norm}
	if highpass {
		f.b0, f.b1, f.b2 = norm, -2*norm, norm
	} else {
		f.b0 = k * k * norm
		f.b1, f.b2 = 2*f.b0, f.b0
	}
	return f
}

func (f *rainBiquad) next(x float64) float64 {
	y := f.b0*x + f.z1
	f.z1 = f.b1*x - f.a1*y + f.z2
	f.z2 = f.b2*x - f.a2*y
	return y
}

// Drift between independent random levels over intervals of varying length.
// Smoothstep gives zero slope at every join, with no repeating LFO period.
type rainDrift struct {
	rng                   *rand.Rand
	minFrames, spanFrames float64
	from, to, phase, step float64
	remaining             int
}

func newRainDrift(sr, minSec, maxSec float64, seed uint64) rainDrift {
	rng := rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))
	return rainDrift{rng: rng, minFrames: sr * minSec, spanFrames: sr * (maxSec - minSec), to: 2*rng.Float64() - 1}
}

func (d *rainDrift) next() float64 {
	if d.remaining == 0 {
		d.from, d.to = d.to, 2*d.rng.Float64()-1
		d.remaining = int(math.Round(d.minFrames + d.spanFrames*d.rng.Float64()))
		d.phase, d.step = 0, 1/float64(d.remaining)
	}
	t := d.phase
	d.phase += d.step
	d.remaining--
	return d.from + (d.to-d.from)*t*t*(3-2*t)
}
