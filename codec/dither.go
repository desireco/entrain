package codec

import (
	"math"
	"math/rand/v2"
)

// quantizer is the shared clip → dither → quantize stage every encoder passes
// through on the way to 16-bit. Dither is triangular (TPDF) at 2 LSB
// peak-to-peak — the standard choice: it decorrelates quantization error
// from the signal, so quiet content fades into noise instead of distorting.
// The fixed seed makes renders byte-for-byte reproducible.
type quantizer struct {
	rng *rand.Rand
}

func newQuantizer() *quantizer {
	return &quantizer{rng: rand.New(rand.NewPCG(0x456E747261696E, 1))} // "Entrain"
}

// int16 quantizes one float sample (nominal ±1).
func (q *quantizer) int16(x float64) int16 {
	const full = 32767.0
	d := (q.rng.Float64() + q.rng.Float64() - 1) / full
	v := math.Round((x + d) * full)
	if v > full {
		v = full
	}
	if v < -full-1 {
		v = -full - 1
	}
	return int16(v)
}
