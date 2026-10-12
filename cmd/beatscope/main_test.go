package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/desireco/entrain/codec"
	"github.com/desireco/entrain/synth"
)

// writeFixture renders 8 s of the focus gamma pair through the engine and
// encodes it in the given format.
func writeFixture(t *testing.T, format string) string {
	t.Helper()
	const sr = 44100
	mix := synth.NewMixer(
		synth.NewGain(sr, synth.NewNoise(sr, synth.Pink, 9), 0.2*0.155),
		synth.NewGain(sr, synth.NewBinaural(sr, 180, -40), 0.30),
		synth.NewGain(sr, synth.NewBinaural(sr, 320, -24), 0.25),
	)
	buf := synth.Render(mix, 8*sr)

	path := filepath.Join(t.TempDir(), "focus."+format)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	switch format {
	case "wav":
		err = codec.WriteWav16(f, sr, buf)
	default:
		err = codec.WriteFlac16(f, sr, buf)
	}
	if err != nil {
		t.Fatal(err)
	}
	if format == "wav" {
		f.Close()
	}
	return path
}

func TestAnalyzeWav(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{writeFixture(t, "wav")}, &out, &errb); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "180.00") || !strings.Contains(s, "40.00") {
		t.Errorf("gamma pair missing from report:\n%s", s)
	}
	if !strings.Contains(s, "320.00") || !strings.Contains(s, "24.00") {
		t.Errorf("beta pair missing from report:\n%s", s)
	}
}

func TestAnalyzeFlac(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{writeFixture(t, "flac")}, &out, &errb); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "180.00") {
		t.Errorf("gamma pair missing from FLAC report:\n%s", out.String())
	}
}

func TestJSONOutput(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--json", writeFixture(t, "wav")}, &out, &errb); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
	var doc struct {
		SampleRate int `json:"sample_rate"`
		Beats      []struct {
			Carrier    float64 `json:"Carrier"`
			Beat       float64 `json:"Beat"`
			Confidence float64 `json:"Confidence"`
		} `json:"beats"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out.String())
	}
	if doc.SampleRate != 44100 || len(doc.Beats) < 2 {
		t.Fatalf("json = sr %d, %d beats", doc.SampleRate, len(doc.Beats))
	}
	if doc.Beats[0].Carrier < 179 || doc.Beats[0].Carrier > 181 {
		t.Errorf("json carrier[0] = %v", doc.Beats[0].Carrier)
	}
}

func TestMinConfidenceFilter(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"--min-confidence=1.01", writeFixture(t, "wav")}, &out, &errb)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "below --min-confidence") {
		t.Errorf("filter note missing:\n%s", out.String())
	}
}

func TestUnsupportedInput(t *testing.T) {
	var out, errb bytes.Buffer
	p := filepath.Join(t.TempDir(), "x.mp3")
	os.WriteFile(p, []byte("id3"), 0o644)
	if code := run([]string{p}, &out, &errb); code != 1 {
		t.Fatalf("code=%d, want 1", code)
	}
	if !strings.Contains(errb.String(), "unsupported input") {
		t.Errorf("stderr = %q", errb.String())
	}
}
