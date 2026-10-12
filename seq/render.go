package seq

import (
	"hash/fnv"
	"math"
	"time"

	"github.com/desireco/entrain/synth"
)

// Render renders the plan to interleaved stereo float64 at the given
// sample rate. Every component becomes one persistent synth voice: its
// oscillator phase and noise stream run uninterrupted for the whole
// render (sbagen's tone-continuity behavior), with only the envelope
// amplitude changing over time. Pink beds are decorrelated stereo (a
// deliberate improvement over sbagen's mono bed); their seed derives
// from the component key, so renders are deterministic.
func (p *Plan) Render(rate int) []float64 {
	n := int(int64(p.End-p.Start) * int64(rate) / int64(time.Second))
	out := make([]float64, 2*n)
	if n == 0 {
		return out
	}
	sr := float64(rate)

	voices := make([]renderVoice, 0, len(p.Voices))
	for i := range p.Voices {
		vp := &p.Voices[i]
		var v synth.StereoVoice
		switch vp.Spec.Kind {
		case White, Pink, Brown:
			v = synth.NewNoise(sr, synth.NoiseColor(vp.Spec.Kind-1), keySeed(vp.Key))
		case Binaural:
			l, r := EarFreqs(vp.Spec)
			v = synth.NewBinaural(sr, (l+r)/2, r-l)
		case Mix:
			v = newBedPlayer(vp.Bed, vp.BedRate, rate)
		default:
			continue
		}
		rv := renderVoice{voice: v}
		for _, bp := range vp.Env {
			rv.frames = append(rv.frames, int64(bp.T)*int64(rate)/int64(time.Second))
			rv.amps = append(rv.amps, bp.Amp)
		}
		voices = append(voices, rv)
	}

	for f := range n {
		for i := range voices {
			v := &voices[i]
			l, r := v.voice.Next()
			out[2*f] += v.amp(int64(f)) * l
			out[2*f+1] += v.amp(int64(f)) * r
		}
	}
	return out
}

// renderVoice is one synth voice plus its envelope, with breakpoint times
// pre-converted to frame indices. The cursor advances monotonically
// during a render (frames only move forward).
type renderVoice struct {
	voice  synth.StereoVoice
	frames []int64
	amps   []float64
	idx    int // breakpoints [0, idx) are behind the playhead
}

// amp evaluates the envelope at frame f, linearly interpolating between
// breakpoints.
func (v *renderVoice) amp(f int64) float64 {
	for v.idx+1 < len(v.frames) && v.frames[v.idx+1] <= f {
		v.idx++
	}
	switch {
	case len(v.frames) == 0 || f < v.frames[0]:
		return 0
	case v.idx >= len(v.amps)-1:
		return v.amps[len(v.amps)-1]
	}
	t0, t1 := v.frames[v.idx], v.frames[v.idx+1]
	if t1 == t0 {
		return v.amps[v.idx+1]
	}
	frac := float64(f-t0) / float64(t1-t0)
	return v.amps[v.idx] + (v.amps[v.idx+1]-v.amps[v.idx])*frac
}

func keySeed(key string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	return h.Sum64()
}

// bedPlayer plays a bed under the render, looping forever with a seamless
// wrap: the loop period is len−fade frames, and during the first `fade`
// frames of each cycle the previous cycle's tail crossfades out under the
// head with equal-power curves (the tail and head are uncorrelated
// material from the same bed). If the bed's rate differs from the render
// rate it is linearly resampled once up front (fine for noise/music beds).
type bedPlayer struct {
	x     []float64 // interleaved
	fade  int       // wrap crossfade length in frames
	i     int       // current frame
	first bool      // first pass: no tail to crossfade yet
}

func newBedPlayer(bed []float64, bedRate, renderRate int) *bedPlayer {
	if len(bed) == 0 {
		return &bedPlayer{}
	}
	x := bed
	if bedRate != renderRate && bedRate > 0 {
		n := len(bed) * renderRate / bedRate
		rs := make([]float64, n&^1) // even length (stereo)
		for i := range rs {
			pos := float64(i) * float64(bedRate) / float64(renderRate)
			j := int(pos)
			frac := pos - float64(j)
			if j+1 >= len(bed) {
				rs[i] = bed[j%len(bed)]
				continue
			}
			rs[i] = bed[j]*(1-frac) + bed[j+1]*frac
		}
		x = rs
	}
	frames := len(x) / 2
	fade := renderRate / 40 // 25 ms
	if fade > frames/4 {
		fade = frames / 4
	}
	if frames < 4 {
		fade = 0
	}
	return &bedPlayer{x: x, fade: fade, first: true}
}

// period is the loop length in frames: the bed minus the crossfade tail,
// which is spent blending into the next cycle's head.
func (b *bedPlayer) period() int { return len(b.x)/2 - b.fade }

// Next returns the next bed frame, crossfading the wrap point.
func (b *bedPlayer) Next() (l, r float64) {
	frames := len(b.x) / 2
	if frames < 2 {
		return 0, 0
	}
	f := b.i
	if f >= frames {
		f = 0 // safety; period() keeps us inside
	}
	l, r = b.x[2*f], b.x[2*f+1]
	if !b.first && b.fade > 0 && f < b.fade {
		// Blend the previous cycle's tail under this cycle's head with
		// equal-power curves; the tail frame is the bed material that
		// would have continued past the wrap (position period()+f).
		w := math.Sqrt((float64(f) + 0.5) / float64(b.fade))
		tw := math.Sqrt(1 - w*w)
		tl, tr := b.x[2*(b.period()+f)], b.x[2*(b.period()+f)+1]
		l = l*w + tl*tw
		r = r*w + tr*tw
	}
	b.i = f + 1
	if b.i >= b.period() {
		b.i = 0
		b.first = false
	}
	return l, r
}
