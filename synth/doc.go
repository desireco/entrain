// Package synth will contain the synthesis core: phase-accumulating
// oscillators, the binaural voice (L/R = carrier ± beat/2), the
// white/pink/brown noise palette, and amplitude envelopes with crossfades.
// Everything mixes in float64; quantization happens only in codec.
// (Phase 1.)
package synth
