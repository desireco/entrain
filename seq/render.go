package seq

import (
	"hash/fnv"
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
