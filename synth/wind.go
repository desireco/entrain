package synth

import (
	"math"
	"math/rand/v2"
)

// Wind is continuous stereo wind made from noise bands, with no pitched
// oscillator. A broad floor persists while gusts introduce brighter air.
type Wind struct {
	pink, white *Noise
	left, right windChannel
	front, lean rainDrift
	gust        windGust
	whiteScale  float64
}

func NewWind(sr float64) *Wind { return newWind(sr, 0) }

func newWind(sr float64, seed int64) *Wind {
	c := windChannel{
		floorHP: rainFilter(sr, 47, true), floorLP: rainFilter(sr, 410, false),
		sweepHP: rainFilter(sr, 240, true), sweepLP: rainFilter(sr, 1700, false),
		airHP: rainFilter(sr, 980, true), airLP: rainFilter(sr, 3400, false),
	}
	return &Wind{
		pink: NewNoise(sr, Pink, streamSeed(7913, seed)), white: NewNoise(sr, White, streamSeed(1109, seed)), left: c, right: c,
		front: newRainDrift(sr, 36, 105, streamSeed(2074, seed)), lean: newRainDrift(sr, 12, 45, streamSeed(6487, seed)),
		gust:       windGust{rng: rand.New(rand.NewPCG(streamSeed(7151, seed), streamSeed(19477, seed))), sr: sr},
		whiteScale: math.Sqrt(sr / 48000),
	}
}

type windChannel struct {
	floorHP, floorLP, sweepHP, sweepLP, airHP, airLP rainBiquad
}

func (c *windChannel) bands(pink, white float64) (floor, sweep, air float64) {
	return c.floorLP.next(c.floorHP.next(pink)), c.sweepLP.next(c.sweepHP.next(pink)), c.airLP.next(c.airHP.next(white))
}

func (w *Wind) Next() (float64, float64) {
	lp, rp := w.pink.Next()
	lw, rw := w.white.Next()
	lf, ls, la := w.left.bands(lp, lw*w.whiteScale)
	rf, rs, ra := w.right.bands(rp, rw*w.whiteScale)
	front, gust, lean := w.front.next(), w.gust.next(), 0.09*w.lean.next()
	low := 0.64 + 0.12*front + 0.25*gust
	mid := 0.18 + 0.65*gust
	high := 0.028 + 0.25*gust*gust
	const gain = 0.157
	left := gain * (low*(0.77*lf+0.23*rf) + (1+lean)*(mid*ls+high*la))
	right := gain * (low*(0.77*rf+0.23*lf) + (1-lean)*(mid*rs+high*ra))
	return softCrest(left, 0.24, 0.35), softCrest(right, 0.24, 0.35)
}

// Each gust has a different strength, rising time and trailing lull. Smooth
// joins keep both level and slope continuous when the next gust begins.
type windGust struct {
	rng                       *rand.Rand
	sr, height                float64
	pos, crest, end, nextGust int
}

func (g *windGust) next() float64 {
	if g.pos >= g.nextGust {
		g.end = int(math.Round(g.sr * (7 + 18*g.rng.Float64())))
		g.crest = int(float64(g.end) * (0.23 + 0.38*g.rng.Float64()))
		g.nextGust = g.end + int(g.sr*(0.8+5*g.rng.Float64()))
		g.height = 0.35 + 0.65*g.rng.Float64()
		g.pos = 0
	}
	i := g.pos
	g.pos++
	if i >= g.end {
		return 0
	}
	var t float64
	if i < g.crest {
		t = float64(i) / float64(g.crest)
	} else {
		t = float64(g.end-i) / float64(g.end-g.crest)
	}
	return g.height * t * t * (3 - 2*t)
}
