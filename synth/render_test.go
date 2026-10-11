package synth_test

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/desireco/entrain/codec"
	"github.com/desireco/entrain/internal/dsp"
	"github.com/desireco/entrain/synth"
)

// TestFocusSetFullChain is the Phase 1 exit check. It renders the focus
// session's tone-set (`pink/20 180+40/30 320+24/25` from
// sessions/focus-40-24-pink.sbg) through synth → codec → WAV, reads the file
// back, and verifies the spectrum matches the math: left ear carries
// 160 Hz and 308 Hz, right ear 200 Hz and 332 Hz, carriers (180/320) and
// off-frequency bins stay silent.
func TestFocusSetFullChain(t *testing.T) {
	const (
		sr      = 44100
		n       = 88200 // 2 s → integer-Hz DFT bins
		ampPink = 0.20
		ampGam  = 0.30 // 180 Hz carrier, 40 Hz beat
		ampBeta = 0.25 // 320 Hz carrier, 24 Hz beat
	)

	mix := synth.NewMixer(
		synth.NewGain(sr, synth.NewNoise(sr, synth.Pink, 42), ampPink),
		synth.NewGain(sr, synth.NewBinaural(sr, 180, 40), ampGam),
		synth.NewGain(sr, synth.NewBinaural(sr, 320, 24), ampBeta),
	)
	buf := synth.Render(mix, n)

	path := filepath.Join(t.TempDir(), "focus-set.wav")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := codec.WriteWav16(f, sr, buf); err != nil {
		t.Fatal(err)
	}
	f.Close()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	gotSR, x, err := dsp.ReadWav16(raw)
	if err != nil {
		t.Fatalf("decode rendered file: %v", err)
	}
	if gotSR != sr || len(x) != len(buf) {
		t.Fatalf("roundtrip sr=%d len=%d, want %d/%d", gotSR, len(x), sr, len(buf))
	}
	l, r := deinterleave(x)

	half := float64(n) / 2
	// tone magnitude = amp · N/2 (exact bin); the pink bed's per-bin energy
	// is ~2 orders below the tones, so 5%/3% windows are clean.
	for _, tc := range []struct {
		ch   []float64
		f    float64
		want float64 // 0 → expect ~no energy
		note string
	}{
		{l, 160, ampGam, "L gamma-low"},
		{r, 200, ampGam, "R gamma-high"},
		{l, 308, ampBeta, "L beta-low"},
		{r, 332, ampBeta, "R beta-high"},
		{l, 180, 0, "L carrier 180 (absent)"},
		{l, 320, 0, "L carrier 320 (absent)"},
		{r, 180, 0, "R carrier 180 (absent)"},
		{l, 137, 0, "L off-frequency control"},
		{r, 275, 0, "R off-frequency control"},
	} {
		got := dsp.Goertzel(tc.ch, float64(sr), tc.f)
		if tc.want > 0 && math.Abs(got-tc.want*half) > 0.05*tc.want*half {
			t.Errorf("%s: magnitude = %.0f, want %.0f ± 5%%", tc.note, got, tc.want*half)
		}
		if tc.want == 0 && got > 0.03*ampGam*half {
			t.Errorf("%s: magnitude = %.0f, want < 3%% of tone peak", tc.note, got)
		}
	}

	// Headroom sanity: the mix (amps total 0.75 + noise) must not clip.
	for i, s := range x {
		if math.Abs(s) > 1 {
			t.Fatalf("sample %d = %g: mix clipped", i, s)
		}
	}
}

// TestRender48kSmoke runs the same voice at 48 kHz, the second supported
// rate.
func TestRender48kSmoke(t *testing.T) {
	const sr = 48000
	const n = 96000 // 2 s, integer-Hz bins
	l, _ := deinterleave(synth.Render(
		synth.NewGain(sr, synth.NewBinaural(sr, 180, 40), 0.3), n))
	half := float64(n) / 2
	if got := dsp.Goertzel(l, sr, 160); math.Abs(got-0.3*half) > 0.005*0.3*half {
		t.Fatalf("48k L@160 = %.0f, want %.0f", got, 0.3*half)
	}
}
