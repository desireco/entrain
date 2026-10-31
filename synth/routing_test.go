package synth

import (
	"math"
	"testing"

	"github.com/desireco/entrain/internal/dsp"
)

// mono extracts one channel from interleaved stereo.
func mono(x []float64, ch int) []float64 {
	out := make([]float64, 0, len(x)/2)
	for i := ch; i < len(x); i += 2 {
		out = append(out, x[i])
	}
	return out
}

func seg(x []float64, sr, from, to float64) []float64 {
	return x[int(from*sr):int(to*sr)]
}

func TestFlipBinauralSwing(t *testing.T) {
	const sr = 8000
	// 0.5 Hz = one full L→R→L cycle in 2 s; swapped exactly at t = 1 s.
	v := NewFlipBinaural(sr, 110, 100, 0.5)
	n := 2 * sr
	x := make([]float64, 2*n)
	for i := range n {
		x[2*i], x[2*i+1] = v.Next()
	}
	L, R := mono(x, 0), mono(x, 1)

	// Early window: 110 Hz (a) belongs to the left, 100 Hz (b) to the right.
	eL, eR := seg(L, sr, 0, 0.3), seg(R, sr, 0, 0.3)
	if hiL, hiR := dsp.Goertzel(eL, sr, 110), dsp.Goertzel(eR, sr, 110); hiL <= 3*hiR {
		t.Errorf("early window: left 110 Hz magnitude %g should dominate right %g", hiL, hiR)
	}
	if loR, loL := dsp.Goertzel(eR, sr, 100), dsp.Goertzel(eL, sr, 100); loR <= 3*loL {
		t.Errorf("early window: right 100 Hz magnitude %g should dominate left %g", loR, loL)
	}

	// Window centered on the swap: the ears have traded places.
	sL, sR := seg(L, sr, 0.8, 1.2), seg(R, sr, 0.8, 1.2)
	if hiR, hiL := dsp.Goertzel(sR, sr, 110), dsp.Goertzel(sL, sr, 110); hiR <= 5*hiL {
		t.Errorf("swapped window: right 110 Hz magnitude %g should dominate left %g", hiR, hiL)
	}
	if loL, loR := dsp.Goertzel(sL, sr, 100), dsp.Goertzel(sR, sr, 100); loL <= 5*loR {
		t.Errorf("swapped window: left 100 Hz magnitude %g should dominate right %g", loL, loR)
	}
}

func TestEMDRAlternatesSides(t *testing.T) {
	const sr = 8000
	// 2 Hz → half-cycles of 250 ms; bursts fill the first quarter of each.
	v := NewEMDR(sr, 600, 2)
	n := sr
	x := make([]float64, 2*n)
	for i := range n {
		x[2*i], x[2*i+1] = v.Next()
	}
	L, R := mono(x, 0), mono(x, 1)

	// First burst [0, 62.5 ms): left only — right is exactly zero.
	if got := dsp.RMS(seg(R, sr, 0.005, 0.055)); got != 0 {
		t.Errorf("first burst: right RMS = %g, want exactly 0", got)
	}
	if got := dsp.RMS(seg(L, sr, 0.005, 0.055)); got < 0.01 {
		t.Errorf("first burst: left RMS = %g, want a clearly audible burst", got)
	}
	// Second half-cycle [250, 500 ms): right only — left is exactly zero.
	if got := dsp.RMS(seg(L, sr, 0.255, 0.305)); got != 0 {
		t.Errorf("second burst: left RMS = %g, want exactly 0", got)
	}
	if got := dsp.RMS(seg(R, sr, 0.255, 0.305)); got < 0.01 {
		t.Errorf("second burst: right RMS = %g, want a clearly audible burst", got)
	}
	// Burst envelope starts and ends at zero (no clicks at burst edges);
	// the sample just inside the boundary carries only a negligible tail.
	burst := seg(L, sr, 0, 0.0625)
	if math.Abs(burst[0]) > 1e-9 || math.Abs(burst[len(burst)-1]) > 1e-3 {
		t.Errorf("burst edges not click-free: first=%g last=%g", burst[0], burst[len(burst)-1])
	}
}
