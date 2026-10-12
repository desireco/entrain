// Package noise generates standalone noise audio: the basic palette
// (white/pink/brown, shared with the synth engine) plus arbitrary-slope
// 1/f^α material via FFT spectral shaping. It backs the noisemaker
// utility (#15) — sophisticated beds that entrain consumes through its
// mix path, per the noise-split boundary in issue #13.
package noise

import (
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/desireco/entrain/internal/dsp"
	"github.com/desireco/entrain/synth"
)

// Basic generates `frames` frames of stereo noise at unit RMS per
// channel, decorrelated. color: "white", "pink", "brown".
func Basic(color string, sr int, seed uint64, frames int) ([]float64, error) {
	var c synth.NoiseColor
	switch color {
	case "white":
		c = synth.White
	case "pink":
		c = synth.Pink
	case "brown":
		c = synth.Brown
	default:
		return nil, fmt.Errorf("noise: unknown color %q (white, pink, brown, or alpha)", color)
	}
	return synth.Render(synth.NewNoise(float64(sr), c, seed), frames), nil
}

// chunkLen is the FFT chunk for spectral shaping; fade is the equal-power
// joint between consecutive chunks.
const (
	chunkLen = 1 << 18
)

// Alpha generates `frames` frames of stereo noise with power spectral
// density ∝ 1/f^alpha (α in [0, 2]: 0 = white, 1 = pink, 2 = brown),
// unit RMS per channel, decorrelated. Each channel is an independent
// white stream shaped bin-by-bin (gain ∝ f^(−α/2), DC zeroed); chunks
// crossfade at their joints with equal-power curves, so the slope holds
// across chunk boundaries.
func Alpha(alpha, sr float64, seed uint64, frames int) []float64 {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 2 {
		alpha = 2
	}
	fade := int(sr) / 40 // 25 ms joints
	if fade > chunkLen/8 {
		fade = chunkLen / 8
	}
	stride := chunkLen - fade

	l := alphaChannel(alpha, sr, seed, frames, fade, stride)
	r := alphaChannel(alpha, sr, seed^0x5DEECE66D, frames, fade, stride)

	out := make([]float64, 2*frames)
	for i := range frames {
		out[2*i], out[2*i+1] = l[i], r[i]
	}
	return out
}

func alphaChannel(alpha, sr float64, seed uint64, frames, fade, stride int) []float64 {
	rng := rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))
	nChunks := (frames + stride - 1) / stride
	if nChunks == 0 {
		nChunks = 1
	}

	out := make([]float64, 0, frames+nChunks*chunkLen)
	for c := range nChunks {
		// White chunk → spectrum → shape → back.
		re := make([]float64, chunkLen)
		im := make([]float64, chunkLen)
		for i := range re {
			re[i] = 2*rng.Float64() - 1
		}
		dsp.FFT(re, im)
		for k := 1; k < chunkLen/2; k++ {
			f := float64(k) * sr / float64(chunkLen)
			g := math.Pow(f, -alpha/2) // |H| so power ∝ f^-alpha
			re[k], im[k] = re[k]*g, im[k]*g
			re[chunkLen-k], im[chunkLen-k] = re[chunkLen-k]*g, im[chunkLen-k]*g
		}
		re[0], im[0] = 0, 0 // no DC
		dsp.IFFT(re, im)

		if c == 0 {
			out = append(out, re[:min(chunkLen, frames)]...)
			continue
		}
		// Crossfade this chunk's head over the previous chunk's tail.
		start := len(out) - fade
		for i := range fade {
			w := math.Sqrt((float64(i) + 0.5) / float64(fade))
			out[start+i] = out[start+i]*(1-w) + re[i]*w
		}
		out = append(out, re[fade:]...)
	}
	out = out[:frames]

	// Unit RMS, zero mean (finite windows of long-correlation noise
	// wander; exact DC is a quality-bar item).
	var s float64
	for _, v := range out {
		s += v * v
	}
	if s > 0 {
		m := dsp.Mean(out)
		g := 1 / math.Sqrt(s/float64(len(out))-m*m)
		for i := range out {
			out[i] = (out[i] - m) * g
		}
	}
	return out
}
