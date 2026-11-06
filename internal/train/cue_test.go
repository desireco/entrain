package train

import (
	"math"
	"testing"

	"github.com/desireco/entrain/site/audio"
)

func TestCueDefaultIsOriginalLoop(t *testing.T) {
	x, err := audio.Cue("noise")
	if err != nil {
		t.Fatal(err)
	}
	player := cuePlayer{samples: x, gain: 1, intervalMS: 625}
	for i := range int64(21 * audio.Rate) {
		l, r := player.next(i)
		j := int(2*i) % len(x)
		if l != x[j] || r != x[j+1] {
			t.Fatalf("default differs at frame %d", i)
		}
	}
}

func TestCustomCueSpacingPreservesEachStrike(t *testing.T) {
	x, err := audio.Cue("noise")
	if err != nil {
		t.Fatal(err)
	}
	for _, ms := range []int{350, 733, 2000} {
		player := cuePlayer{samples: x, gain: 0.375, intervalMS: ms}
		// Test across the 10s cue and 60s bed boundaries. The clock follows
		// elapsed time while the sixteen original strikes retain their pitch.
		for i := range int64(63 * audio.Rate) {
			l, r := player.next(i)
			strike := i * 1000 / (int64(ms) * audio.Rate)
			onsetFrame := float64(strike) * float64(ms) * audio.Rate / 1000
			offset := i - int64(math.Ceil(onsetFrame))
			slot := strike % 16
			sourceStart := int64(math.Ceil(float64(slot) * 0.625 * audio.Rate))
			sourceEnd := int64(math.Ceil(float64(slot+1) * 0.625 * audio.Rate))
			var wantL, wantR float64
			if sourceStart+offset < sourceEnd {
				j := 2 * (sourceStart + offset)
				wantL, wantR = x[j]*0.375, x[j+1]*0.375
			}
			if l != wantL || r != wantR {
				t.Fatalf("interval %d ms, frame %d: want (%g, %g), got (%g, %g)", ms, i, wantL, wantR, l, r)
			}
		}
	}
}

func TestPackagedMixHeadroomAtMaximumCueVolume(t *testing.T) {
	peak := func(x []float64) float64 {
		var p float64
		for _, s := range x {
			p = max(p, math.Abs(s))
		}
		return p
	}
	var bedPeak, cuePeak float64
	for _, name := range []string{"pink", "white", "rain"} {
		x, err := audio.Background(name)
		if err != nil {
			t.Fatal(err)
		}
		bedPeak = max(bedPeak, peak(x))
	}
	for _, name := range []string{"tone", "bell", "noise"} {
		x, err := audio.Cue(name)
		if err != nil {
			t.Fatal(err)
		}
		cuePeak = max(cuePeak, peak(x))
	}
	// This bound holds at any timing, even if the individual maxima align.
	if worst := bedPeak + 2*cuePeak; worst >= 1 {
		t.Fatalf("200%% cue can clip: %.4f", worst)
	}
}
