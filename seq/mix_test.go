package seq_test

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/desireco/entrain/codec"
	"github.com/desireco/entrain/internal/dsp"
	"github.com/desireco/entrain/seq"
	"github.com/desireco/entrain/synth"
)

// writeBed renders 2 s of noise at RMS 0.25 to a WAV. White by default
// (uncorrelated, so windowed RMS is stable); tests can pick pink.
func writeBedColor(t *testing.T, dir string, rate int, color synth.NoiseColor) string {
	t.Helper()
	bed := synth.Render(synth.NewGain(float64(rate), synth.NewNoise(float64(rate), color, 77), 0.25), 2*rate)
	path := filepath.Join(dir, "bed.wav")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := codec.WriteWav16(f, rate, bed); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return path
}

func writeBed(t *testing.T, dir string, rate int) string {
	return writeBedColor(t, dir, rate, synth.White)
}

func TestMixBedLoopsSeamlessly(t *testing.T) {
	dir := t.TempDir()
	writeBed(t, dir, 44100)
	// The session lives next to the bed; -m resolves via the session dir.
	session := "-m bed.wav\n\na: mix/100 180+40/30\noff: -\n\nNOW a\n+00:00:06 off\n"
	path := filepath.Join(dir, "s.sbg")
	if err := os.WriteFile(path, []byte(session), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := seq.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := seq.BuildPlan(s, seq.PlanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	const rate = 44100
	x := p.Render(rate)
	l := window(x, rate, 0, 6)

	// The loop period is bedFrames − fade (25 ms), matching bedPlayer.
	period := 2*rate - rate/40
	w := rate / 10 // 100 ms windows
	for k := 1; k*period+3*w < len(l); k++ {
		seam := k * period
		before := dsp.RMS(l[seam-3*w : seam-w])
		straddle := dsp.RMS(l[seam-w/2 : seam+w/2])
		after := dsp.RMS(l[seam+w : seam+3*w])
		if math.Abs(straddle-before) > 0.12*before || math.Abs(straddle-after) > 0.12*after {
			t.Errorf("seam %d: straddle RMS %.4f vs before %.4f / after %.4f",
				k, straddle, before, after)
		}
	}
	// A hard-loop regression (no crossfade) would put a full peak-to-peak
	// jump at the wrap; with the crossfade, seam-region jumps are
	// statistically identical to everywhere else.
	globalMax := 0.0
	for i := 1; i < len(l); i++ {
		if d := math.Abs(l[i] - l[i-1]); d > globalMax {
			globalMax = d
		}
	}
	for k := 1; k*period < 6*rate; k++ {
		seam := k * period
		for i := seam - rate/200; i < seam+rate/200; i++ {
			if d := math.Abs(l[i] - l[i-1]); d > globalMax*1.05 {
				t.Errorf("seam %d: sample jump %.3f exceeds global max %.3f (hard loop?)",
					k, d, globalMax)
			}
		}
	}
	// Bed present throughout at roughly the bed's own level (mix/100).
	if r := dsp.RMS(l[rate : 5*rate]); r < 0.2 || r > 0.35 {
		t.Errorf("mixed level = %.4f, want ≈0.25 (bed RMS + tones)", r)
	}
}

func TestMixAmplitudeScales(t *testing.T) {
	dir := t.TempDir()
	writeBed(t, dir, 44100)
	session := "-m bed.wav\n\na: mix/40\noff: -\n\nNOW a\n+00:00:04 off\n"
	path := filepath.Join(dir, "s.sbg")
	os.WriteFile(path, []byte(session), 0o644)
	s, _ := seq.ParseFile(path)
	// Short fades so the [1s,3s] window sits at the bed's full level.
	p, err := seq.BuildPlan(s, seq.PlanOpts{Crossfade: 200 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	const rate = 44100
	got := dsp.RMS(window(p.Render(rate), rate, 1, 3))
	want := 0.4 * 0.25 // amp/100 × bed RMS
	if got < 0.8*want || got > 1.2*want {
		t.Errorf("mix/40 level = %.4f, want ≈%.4f", got, want)
	}
}

func TestMixErrorPaths(t *testing.T) {
	dir := t.TempDir()
	// mix/amp without -m
	path := filepath.Join(dir, "s1.sbg")
	os.WriteFile(path, []byte("a: mix/50\noff: -\n\nNOW a\n+00:00:04 off\n"), 0o644)
	s, _ := seq.ParseFile(path)
	if _, err := seq.BuildPlan(s, seq.PlanOpts{}); err == nil {
		t.Error("mix without -m should error")
	}
	// -m pointing at a missing file
	path = filepath.Join(dir, "s2.sbg")
	os.WriteFile(path, []byte("-m nosuch.wav\n\na: mix/50\noff: -\n\nNOW a\n+00:00:04 off\n"), 0o644)
	s, _ = seq.ParseFile(path)
	if _, err := seq.BuildPlan(s, seq.PlanOpts{}); err == nil {
		t.Error("missing -m bed should error")
	}
}

func TestMixContinuityGlide(t *testing.T) {
	dir := t.TempDir()
	writeBed(t, dir, 44100)
	session := "-m bed.wav\n\na: mix/80\nb: mix/20\noff: -\n\nNOW a\n+00:00:04 b\n+00:00:08 off\n"
	path := filepath.Join(dir, "s.sbg")
	os.WriteFile(path, []byte(session), 0o644)
	s, _ := seq.ParseFile(path)
	p, err := seq.BuildPlan(s, seq.PlanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	const rate = 44100
	l := window(p.Render(rate), rate, 0, 8)
	hi := dsp.RMS(l[rate : 3*rate])
	mid := dsp.RMS(l[4*rate-rate/10 : 4*rate+rate/10])
	lo := dsp.RMS(l[5*rate : 7*rate])
	if hi < lo*2 || mid < lo || mid > hi {
		t.Errorf("bed level glide: hi=%.4f mid=%.4f lo=%.4f", hi, mid, lo)
	}
}
