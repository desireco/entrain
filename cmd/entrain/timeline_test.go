package main

import (
	"os"
	"strings"
	"testing"

	"github.com/desireco/entrain/internal/dsp"
)

func TestNativeCueSectionInspectionAndLengthLimit(t *testing.T) {
	t.Chdir(t.TempDir())
	source := `version = 1
[[voice]]
type = "soundscape"
background = "rain"
duration = "4s"
[[voice.cue_section]]
start = "2s"
duration = "1s"
cue = "bell"
fade_in = "50ms"
fade_out = "200ms"
`
	if err := os.WriteFile("scene.entrain", []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := runCLI(t, "render", "scene.entrain", "--dry-run")
	if code != 0 || !strings.Contains(out, "2s–3s") || !strings.Contains(out, "fade in 50ms, out 200ms") {
		t.Fatalf("inspection: %d %s %s", code, out, stderr)
	}
	code, out, stderr = runCLI(t, "render", "scene.entrain", "-L", "0:00:01", "-o", "short.wav")
	if code != 0 || strings.Contains(out, "cue bell:") {
		t.Fatalf("preview before cue: %d %s %s", code, out, stderr)
	}
	wav, err := os.ReadFile("short.wav")
	if err != nil {
		t.Fatal(err)
	}
	rate, samples, err := dsp.ReadWav16(wav)
	if err != nil || rate != 44100 || len(samples) != 2*rate {
		t.Fatalf("preview rate=%d samples=%d err=%v", rate, len(samples), err)
	}
	if err := os.WriteFile("bad.entrain", []byte(strings.Replace(source, `start = "2s"`, `start = "5s"`, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{{"--dry-run"}, {"-L", "0:00:01", "-o", "invalid.wav"}} {
		args := append([]string{"render", "bad.entrain"}, flags...)
		code, _, stderr := runCLI(t, args...)
		if code == 0 || !strings.Contains(stderr, "cue_section 1") {
			t.Fatalf("invalid section accepted: %v %s", args, stderr)
		}
	}
	if _, err := os.Stat("invalid.wav"); !os.IsNotExist(err) {
		t.Fatal("invalid schedule wrote an output")
	}
}
