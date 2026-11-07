package synth

import (
	"math"
	"testing"
)

// A breaker changes color as well as level: low water leads, the broadband
// lip rises near shore, and the quieter backwash outlasts both.
func TestSurfWaveApproachBreakRetreat(t *testing.T) {
	w := shoreWave{approach: 3, rise: 0.5, breaking: 1.4, wash: 6, power: 1, skew: 0.2, travel: 0.8}
	approach := w.levels(1.5, 0)
	crest := w.levels(3.5, 0)
	retreat := w.levels(6.5, 0)
	if approach.deep <= 0 || approach.breaking != 0 || approach.foam != 0 {
		t.Fatalf("swell: %+v", approach)
	}
	if crest.breaking < 0.95 || crest.deep <= 0 || crest.foam <= 0 {
		t.Fatalf("crest: %+v", crest)
	}
	if retreat.foam <= 0 || retreat.deep != 0 || retreat.breaking != 0 {
		t.Fatalf("retreat: %+v", retreat)
	}
	if w.levels(12, -1) != (surfBands{}) || w.levels(12, 1) != (surfBands{}) {
		t.Fatal("old wave did not recede")
	}
	if w.levels(3.2, -1).breaking <= w.levels(3.2, 1).breaking {
		t.Fatal("traveling crest arrived in both channels at once")
	}
}

func TestSurfDeterministicContinuousAndBounded(t *testing.T) {
	// Run past both possible 60-second and shorter-bed reset points.
	const rate = 8000
	s, again := NewSurf(rate), NewSurf(rate)
	first := make([]float64, rate)
	var dot, norm, normLater, leftSquares, rightSquares, cross float64
	var quiet, loud, sum, peak float64
	quiet = math.MaxFloat64
	for i := 0; i < 64*rate; i++ {
		l, r := s.Next()
		if i < rate {
			a, b := again.Next()
			if l != a || r != b {
				t.Fatalf("non-deterministic frame %d", i)
			}
		}
		if i >= 2*rate && i < 3*rate {
			first[i-2*rate] = l
		}
		if i >= 62*rate && i < 63*rate {
			a := first[i-62*rate]
			dot += a * l
			norm += a * a
			normLater += l * l
		}
		peak = max(peak, math.Abs(l), math.Abs(r))
		sum += (l*l + r*r) / 2
		leftSquares += l * l
		rightSquares += r * r
		cross += l * r
		if (i+1)%rate == 0 {
			quiet = min(quiet, sum/rate)
			loud = max(loud, sum/rate)
			sum = 0
		}
	}
	repeatCorrelation := dot / math.Sqrt(norm*normLater)
	stereoCorrelation := cross / math.Sqrt(leftSquares*rightSquares)
	if peak >= 0.35 || quiet < 1e-5 || math.IsNaN(stereoCorrelation) || stereoCorrelation >= 0.9 {
		t.Fatalf("invalid stream: peak %g, quiet power %g, stereo corr %g", peak, quiet, stereoCorrelation)
	}
	if loud/quiet < 4 || math.Abs(repeatCorrelation) > 0.2 {
		t.Fatalf("missing wave contrast or unexpected repeat: power ratio %g, lag corr %g", loud/quiet, repeatCorrelation)
	}
}

func TestSurfWaveTimesAcrossSampleRates(t *testing.T) {
	// Advance the event scheduler at both supported rates. Identical seeds
	// should produce the same arriving wave in seconds, including its pan.
	a, b := NewSurf(44100), NewSurf(48000)
	for i := 0; i < 75; i++ {
		if math.Abs(float64(a.nextWave)/a.sr-float64(b.nextWave)/b.sr) > 0.002 {
			t.Fatalf("arrival %d diverged between sample rates", i)
		}
		a.launch(a.nextWave)
		b.launch(b.nextWave)
		ia := (a.nextSlot + len(a.waves) - 1) % len(a.waves)
		ib := (b.nextSlot + len(b.waves) - 1) % len(b.waves)
		if a.waves[ia].power != b.waves[ib].power || a.waves[ia].travel != b.waves[ib].travel {
			t.Fatal("wave shape changed with sampling rate")
		}
	}
}
