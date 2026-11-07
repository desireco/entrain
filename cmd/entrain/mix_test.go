package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/desireco/entrain/internal/dsp"
)

const nativeSource = `# A train exported from Studio
version = 1
title = "Rain and bell"
output = "flac"

[[voice]]
type = "soundscape"
background = "rain"
duration = "1.375s" # fractional final second
cue = "bell"
`

func TestNativeAndShortcutMatch(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("studio.entrain", []byte(nativeSource), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) []byte {
		t.Helper()
		code, out, stderr := runCLI(t, args...)
		if code != 0 {
			t.Fatalf("%v: exit %d, %s %s", args, code, out, stderr)
		}
		data, err := os.ReadFile(args[len(args)-1])
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	native := run("render", "studio.entrain", "-o", "native.wav")
	short := run("mix", "rain", "--cue", "bell", "--duration", "1.375s", "-o", "short.wav")
	if !bytes.Equal(native, short) {
		t.Fatal("builder and shortcut produced different WAVs")
	}
	sr, audio, err := dsp.ReadWav16(short)
	if err != nil || sr != 44100 || len(audio)/2 != 60637 {
		t.Fatalf("fractional duration: %d frames, rate %d, %v", len(audio)/2, sr, err)
	}
	alone := run("mix", "rain", "--duration", "1.375s", "-o", "solo.wav")
	if bytes.Equal(alone, short) {
		t.Fatal("cue option had no effect")
	}
	none := run("mix", "rain", "--cue", "none", "--duration", "1.375s", "-o", "none.wav")
	if !bytes.Equal(none, alone) {
		t.Fatal("default should be background alone")
	}

	for _, format := range []string{"flac", "mp3"} {
		data := run("studio.entrain", "-o", "test."+format)
		prefix := []byte("fLaC")
		if format == "mp3" {
			prefix = []byte{0xff, 0xfb}
		}
		if !bytes.HasPrefix(stripID3(data), prefix) {
			t.Fatalf("incorrect %s data", format)
		}
	}
}

func TestNativeValidationAndDryRun(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, tc := range []struct{ old, new, message string }{
		{"version = 1", "version = 2", "version"},
		{"cue =", "cue_typo =", "cue_typo"},
		{"cue = \"bell\"", "cue = \"bells\"", "unknown cue"},
		{"duration = \"1.375s\"", "duration = -1", "duration"},
		{"\"soundscape\"", "\"binaural\"", "soundscape"},
		{"background = \"rain\"", "background = \"river\"", "unknown background"},
	} {
		source := strings.Replace(nativeSource, tc.old, tc.new, 1)
		if err := os.WriteFile("in.toml", []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := runCLI(t, "render", "in.toml", "-o", "invalid.wav")
		if code == 0 || !strings.Contains(stderr, tc.message) {
			t.Fatalf("accepted %s or bad error: %s", tc.new, stderr)
		}
	}
	if _, err := os.Stat("invalid.wav"); !os.IsNotExist(err) {
		t.Fatal("invalid session created output")
	}
	source := strings.Replace(nativeSource, `duration = "1.375s"`, "duration = 13", 1)
	if err := os.WriteFile("in.toml", []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := runCLI(t, "in.toml", "--dry-run")
	if code != 0 || !strings.Contains(out, "13s") || !strings.Contains(out, "in.flac") {
		t.Fatalf("dry run %s %s", out, stderr)
	}
	files, err := filepath.Glob("*.flac")
	if err != nil || len(files) != 0 {
		t.Fatal("dry run created a file")
	}
	code, _, _ = runCLI(t, "in.toml", "-o", "in.toml")
	if code == 0 {
		t.Fatal("allowed overwriting the source")
	}
}

func TestMixNamedLegacySession(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("mix.sbg", []byte("a: pink/20\nNOW a\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := runCLI(t, "mix", "--dry-run")
	if code != 0 || !strings.Contains(out, "pink/20") {
		t.Fatalf("legacy shorthand failed: %s %s", out, stderr)
	}
}

func TestCueLevelAndIntervalCLI(t *testing.T) {
	t.Chdir(t.TempDir())
	write := func(name, source string) {
		t.Helper()
		if err := os.WriteFile(name, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	render := func(args ...string) []byte {
		t.Helper()
		code, _, stderr := runCLI(t, args...)
		if code != 0 {
			t.Fatalf("%v: %s", args, stderr)
		}
		data, err := os.ReadFile(args[len(args)-1])
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	write("custom.entrain", nativeSource+"cue_volume_percent = 37.5\ncue_interval_ms = 733\n")
	code, out, stderr := runCLI(t, "render", "custom.entrain", "--dry-run")
	if code != 0 || !strings.Contains(out, "37.5%, every 733 ms") {
		t.Fatalf("inspect: %s %s", out, stderr)
	}
	for _, ext := range []string{"wav", "flac", "mp3"} {
		a := render("render", "custom.entrain", "-o", "native."+ext)
		b := render("mix", "rain", "--cue", "bell", "--cue-volume", "37.5", "--cue-interval-ms", "733", "--duration", "1.375s", "-o", "mix."+ext)
		if !bytes.Equal(stripID3(a), stripID3(b)) {
			t.Fatalf("native and shortcut differ for %s", ext)
		}
	}
	a := render("mix", "rain", "--cue", "bell", "--cue-volume", "0", "--cue-interval-ms", "350", "--duration", "1.375s", "-o", "muted.wav")
	b := render("mix", "rain", "--cue", "none", "--cue-volume", "200", "--cue-interval-ms", "2000", "--duration", "1.375s", "-o", "off.wav")
	if !bytes.Equal(a, b) {
		t.Fatal("zero cue must render exactly the solo bed")
	}
}

func TestRejectBadCueSettings(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, tc := range []struct{ flag, key, val string }{
		{"--cue-volume", "cue_volume_percent", "-0.1"},
		{"--cue-volume", "cue_volume_percent", "201"},
		{"--cue-volume", "cue_volume_percent", "nan"},
		{"--cue-volume", "cue_volume_percent", "inf"},
		{"--cue-interval-ms", "cue_interval_ms", "349"},
		{"--cue-interval-ms", "cue_interval_ms", "2001"},
		{"--cue-interval-ms", "cue_interval_ms", "733.5"},
		{"--cue-interval-ms", "cue_interval_ms", "nan"},
	} {
		code, _, _ := runCLI(t, "mix", "pink", tc.flag, tc.val, "--dry-run")
		if code == 0 {
			t.Errorf("mix accepted %s %s", tc.flag, tc.val)
		}
		if err := os.WriteFile("invalid.entrain", []byte(nativeSource+tc.key+" = "+tc.val+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		code, _, _ = runCLI(t, "invalid.entrain", "--dry-run")
		if code == 0 {
			t.Errorf("TOML accepted %s = %s", tc.key, tc.val)
		}
	}
}
