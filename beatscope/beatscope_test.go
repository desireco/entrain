package beatscope_test

import (
	"math"
	"testing"

	"github.com/desireco/entrain/beatscope"
	"github.com/desireco/entrain/synth"
)

const sr = 44100

// render renders seconds of a stereo voice through the synth core.
func render(v synth.StereoVoice, seconds int) []float64 {
	return synth.Render(v, sr*seconds)
}

func TestDetectsFocusPair(t *testing.T) {
	// The focus session's gamma pair: 180 Hz carrier, 40 Hz beat, over a
	// pink bed (which must not register as a beat).
	mix := synth.NewMixer(
		synth.NewGain(sr, synth.NewNoise(sr, synth.Pink, 7), 0.2*0.155),
		synth.NewGain(sr, synth.NewBinaural(sr, 180, -40), 0.30), // L=200, R=160
	)
	beats := beatscope.Analyze(render(mix, 10), sr)
	if len(beats) == 0 {
		t.Fatal("no beats detected")
	}
	if math.Abs(beats[0].Carrier-180) > 0.5 {
		t.Errorf("carrier = %.2f Hz, want 180", beats[0].Carrier)
	}
	if math.Abs(beats[0].Beat-40) > 0.5 {
		t.Errorf("beat = %.2f Hz, want 40", beats[0].Beat)
	}
	if beats[0].Confidence < 0.6 {
		t.Errorf("confidence = %.2f, want ≥0.6 for a clean pair", beats[0].Confidence)
	}
}

func TestDetectsBothFocusPairs(t *testing.T) {
	mix := synth.NewMixer(
		synth.NewGain(sr, synth.NewBinaural(sr, 180, -40), 0.30),
		synth.NewGain(sr, synth.NewBinaural(sr, 320, -24), 0.25),
	)
	beats := beatscope.Analyze(render(mix, 10), sr)
	if len(beats) < 2 {
		t.Fatalf("detected %d beats, want 2: %+v", len(beats), beats)
	}
	found := map[float64]bool{}
	for _, b := range beats {
		found[math.Round(b.Carrier/10)*10] = true
	}
	if !found[180] || !found[320] {
		t.Fatalf("carriers = %+v, want 180 and 320", beats)
	}
}

func TestNoBeatsInMonoNoise(t *testing.T) {
	// Decorrelated noise on its own has no stable L/R peak pairing.
	mix := synth.NewGain(sr, synth.NewNoise(sr, synth.Pink, 3), 0.3*0.155)
	if beats := beatscope.Analyze(render(mix, 10), sr); len(beats) > 0 {
		// Low-confidence false positives are tolerable; loud confident ones are not.
		for _, b := range beats {
			if b.Confidence > 0.5 {
				t.Fatalf("noise produced a confident beat: %+v", b)
			}
		}
	}
}

func TestNoBeatsInMonoTone(t *testing.T) {
	// Identical tone both ears (no binaural offset) must not pair with itself.
	mix := synth.NewGain(sr, synth.NewTone(sr, 300), 0.4)
	for _, b := range beatscope.Analyze(render(mix, 10), sr) {
		if b.Beat < 1 {
			t.Fatalf("mono tone paired with itself: %+v", b)
		}
	}
}

func TestShortInputRejected(t *testing.T) {
	if beats := beatscope.Analyze(make([]float64, 2*sr), sr); beats != nil {
		t.Fatal("under one window + hop of input should return nil")
	}
}
