package synth

import (
	"math"
	"math/rand/v2"
)

// Surf synthesizes a shoreline from overlapping waves. Each wave has a swell,
// a breaking lip, and a longer retreat; one wave can be washing back while
// another starts offshore. All timing and filter state persist for the take.
type Surf struct {
	sr          float64
	pink, white *Noise
	channels    [2]surfChannel
	rng         *rand.Rand
	waves       [3]shoreWave
	nextSlot    int
	nextWave    int64
	frame       int64
	step        int64
	remaining   int64
	strength    float64
	gains, ramp [2]surfBands
	whiteGain   float64
}

// NewSurf starts an independent repeatable stream at the requested sample rate.
func NewSurf(sr float64) *Surf { return newSurf(sr, 0) }

func newSurf(sr float64, seed int64) *Surf {
	filter := surfChannel{
		deepHP: rainFilter(sr, 53, true), deepLP: rainFilter(sr, 580, false),
		breakHP: rainFilter(sr, 550, true), breakLP: rainFilter(sr, 4400, false),
		foamHP: rainFilter(sr, 230, true), foamLP: rainFilter(sr, 1900, false),
		pressure: rainFilter(sr, 5, false),
	}
	s := &Surf{
		sr: sr, pink: NewNoise(sr, Pink, streamSeed(7727, seed)), white: NewNoise(sr, White, streamSeed(23911, seed)),
		channels: [2]surfChannel{filter, filter}, rng: rand.New(rand.NewPCG(streamSeed(9167, seed), streamSeed(31259, seed))),
		step: max(1, int64(math.Round(sr/200))), strength: 1,
		whiteGain: math.Sqrt(sr / 44100),
	}
	// Open on the approach of a wave. The session envelope supplies the
	// entrance; no prerender or length-dependent initialization is needed.
	s.launch(-int64(1.4 * sr))
	return s
}

type surfChannel struct {
	deepHP, deepLP, breakHP, breakLP, foamHP, foamLP, pressure rainBiquad
}

func (c *surfChannel) layers(pink, white float64) (deep, breaking, foam float64) {
	deep = c.deepLP.next(c.deepHP.next(pink))
	breaking = c.breakLP.next(c.breakHP.next(white))
	foam = c.foamLP.next(c.foamHP.next(pink))
	// Slow, irregular local pressure breaks the broadband crash into
	// soft patches of froth. It never gates the entire stereo scene.
	pressure := c.pressure.next(white)
	bubble := pressure / (0.020 + math.Abs(pressure))
	return deep, breaking * (1 + 0.24*bubble), foam * (1 - 0.29*bubble)
}

type shoreWave struct {
	at                             int64
	approach, rise, breaking, wash float64 // seconds
	power, skew, travel            float64
}

func (s *Surf) launch(at int64) {
	random := s.rng.Float64
	s.strength = 0.65*s.strength + 0.35*(0.65+0.75*random())
	s.waves[s.nextSlot] = shoreWave{
		at: at, approach: 2.0 + 2.1*random(), rise: 0.32 + 0.52*random(),
		breaking: 0.8 + 1.4*random(), wash: 4.0 + 3.8*random(),
		power: s.strength * (0.75 + 0.40*random()),
		skew:  0.08 + 0.20*random(), travel: 2*random() - 1,
	}
	s.nextSlot = (s.nextSlot + 1) % len(s.waves)
	// The longest wave ends before three later arrivals can replace it.
	s.nextWave = at + int64(math.Round(s.sr*(5.8+5.6*random())))
}

type surfBands struct{ deep, breaking, foam float64 }

// ramp01 goes from zero to one with zero slope at both ends. All envelope
// times are in seconds, independent of sample rate and session duration.
func ramp01(t float64) float64 {
	t = max(0, min(1, t))
	return t * t * (3 - 2*t)
}

func (w shoreWave) levels(age, side float64) surfBands {
	t := age - w.travel*side*w.skew
	if w.power == 0 || t <= 0 || t >= w.approach+w.rise+w.wash {
		return surfBands{}
	}
	crest := t - w.approach
	swell := ramp01(t/w.approach) * (1 - ramp01(crest/(w.rise+1.8)))
	lip := ramp01(crest/w.rise) * (1 - ramp01((crest-w.rise)/w.breaking))
	retreat := ramp01((crest-0.25)/(w.rise+0.7)) * (1 - ramp01((crest-w.rise)/w.wash))
	return surfBands{
		deep:     w.power * swell,
		breaking: w.power * lip * (1 + 0.13*side*w.travel),
		foam:     w.power * retreat * (1 - 0.08*side*w.travel),
	}
}

func (s *Surf) updateGains() {
	end := s.frame + s.step
	if s.nextWave <= end {
		s.launch(s.nextWave)
	}
	for ch, side := range [2]float64{-1, 1} {
		// A distant shoreline remains audible between the nearby waves.
		next := surfBands{deep: 0.48, breaking: 0.035, foam: 0.16}
		for _, w := range s.waves {
			part := w.levels(float64(end-w.at)/s.sr, side)
			next.deep += part.deep
			next.breaking += part.breaking
			next.foam += part.foam
		}
		g := s.gains[ch]
		n := float64(s.step)
		s.ramp[ch] = surfBands{(next.deep - g.deep) / n, (next.breaking - g.breaking) / n, (next.foam - g.foam) / n}
	}
	s.remaining = s.step
}

// Next produces one stereo frame. Envelopes use a 200 Hz control rate and
// interpolate between updates; the noise and filters always run at audio rate.
func (s *Surf) Next() (float64, float64) {
	if s.remaining == 0 {
		s.updateGains()
	}
	pl, pr := s.pink.Next()
	wl, wr := s.white.Next()
	dl, bl, fl := s.channels[0].layers(pl, wl*s.whiteGain)
	dr, br, fr := s.channels[1].layers(pr, wr*s.whiteGain)
	for i := range s.gains {
		g, r := &s.gains[i], s.ramp[i]
		g.deep += r.deep
		g.breaking += r.breaking
		g.foam += r.foam
	}
	s.frame++
	s.remaining--
	// The bass is partly shared across ears. The breaking lip and high
	// foam retain their width as a crest moves along the shore.
	l, r := s.gains[0], s.gains[1]
	left := 0.13*l.deep*(0.78*dl+0.22*dr) + 0.16*l.breaking*bl + 0.14*l.foam*fl
	right := 0.13*r.deep*(0.78*dr+0.22*dl) + 0.16*r.breaking*br + 0.14*r.foam*fr
	// A fixed trim keeps the level near the rain master without changing
	// the contrast between the approaching swell and its crest.
	const masterGain = 0.65
	return softCrest(masterGain*left, 0.24, 0.35), softCrest(masterGain*right, 0.24, 0.35)
}
