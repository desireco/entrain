package synth

// StereoVoice is a source of stereo audio: each Next call returns one sample
// frame (left, right) in float64. Samples are nominally in [−1, +1]; voices
// may briefly exceed it (the encoder clips at quantization), but a mix's
// steady-state sum should stay within ±1.
type StereoVoice interface {
	Next() (l, r float64)
}

// Mixer sums any number of voices sample-by-sample.
type Mixer struct {
	voices []StereoVoice
}

// NewMixer creates a mixer playing the given voices simultaneously.
func NewMixer(voices ...StereoVoice) *Mixer {
	return &Mixer{voices: voices}
}

// Add adds a voice to the mix.
func (m *Mixer) Add(v StereoVoice) {
	m.voices = append(m.voices, v)
}

// Next returns the summed sample frame.
func (m *Mixer) Next() (l, r float64) {
	for _, v := range m.voices {
		vl, vr := v.Next()
		l += vl
		r += vr
	}
	return l, r
}

// Render pulls n sample frames from v and returns them interleaved:
// [l₀, r₀, l₁, r₁, …], length 2n.
func Render(v StereoVoice, n int) []float64 {
	out := make([]float64, 2*n)
	for i := range n {
		out[2*i], out[2*i+1] = v.Next()
	}
	return out
}
