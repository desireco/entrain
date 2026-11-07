package synth

import (
	"math"
	"sort"
	"testing"
)

func TestWindStreamsDistinctGusts(t *testing.T) {
	const rate = 44100
	wind := NewWind(rate)
	replay := NewWind(rate)
	var windows []float64
	var product, a2, b2, sum float64
	ref := make([]float64, rate)
	for i := range 75 * rate {
		l, r := wind.Next()
		if i < 2*rate {
			a, b := replay.Next()
			if a != l || b != r {
				t.Fatalf("wind differs at %d", i)
			}
		}
		if math.IsNaN(l+r) || math.Abs(l) > 0.35 || math.Abs(r) > 0.35 {
			t.Fatalf("invalid frame %d", i)
		}
		sum += l*l + r*r
		if i%rate == rate-1 {
			windows = append(windows, sum/(2*rate))
			sum = 0
		}
		if i >= rate && i < 2*rate {
			ref[i-rate] = l
		}
		if i >= 61*rate && i < 62*rate {
			a := ref[i-61*rate]
			product += a * l
			a2 += a * a
			b2 += l * l
		}
	}
	sort.Float64s(windows)
	if spread := 10 * math.Log10(windows[67]/windows[7]); spread < 2 || spread > 12 {
		t.Fatalf("wind level change = %g dB", spread)
	}
	if correlation := product / math.Sqrt(a2*b2); math.Abs(correlation) > .15 {
		t.Fatalf("minute correlation = %g", correlation)
	}
}

func TestProceduralFramesAllocateNothing(t *testing.T) {
	for _, kind := range []string{"rain", "wind", "surf"} {
		v, ok := NewProcedural(kind, 44100)
		if !ok {
			t.Fatal(kind)
		}
		if got := testing.AllocsPerRun(1000, func() { v.Next() }); got != 0 {
			t.Errorf("%s allocated %g times per stereo frame", kind, got)
		}
	}
}
