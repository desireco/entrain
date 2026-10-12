package codec

import (
	"math"
	"math/rand/v2"
)

// quantizer is the shared clip → dither → quantize stage every encoder
// passes through on the way to fixed-point output. Dither is triangular
// (TPDF) at 2 LSB peak-to-peak — the standard choice: it decorrelates
// quantization error from the signal, so quiet content fades into noise
// instead of distorting. The fixed seed makes renders byte-for-byte
// reproducible.
type quantizer struct {
	rng  *rand.Rand
	bits int
	full float64
}

func newQuantizer(bits int) *quantizer {
	return &quantizer{
		rng:  rand.New(rand.NewPCG(0x456E747261696E, 1)), // "Entrain"
		bits: bits,
		full: math.Pow(2, float64(bits-1)) - 1, // signed full scale: 32767 / 8388607
	}
}

// sample quantizes one float sample (nominal ±1) to the configured depth.
func (q *quantizer) sample(x float64) int32 {
	d := (q.rng.Float64() + q.rng.Float64() - 1) / q.full
	v := math.Round((x + d) * q.full)
	if v > q.full {
		v = q.full
	}
	if v < -q.full-1 {
		v = -q.full - 1
	}
	return int32(v)
}
