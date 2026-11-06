package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/desireco/entrain/internal/dsp"
	"github.com/desireco/entrain/internal/train"
)

func TestSaveSessionRoundTrip(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("elsewhere", 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, format, cue, volume string
		bits, bitrate             string
	}{
		{"default", "wav", "none", "100", "16", "320"},
		{"muted", "flac", "bell", "0", "24", "320"},
		{"custom", "mp3", "noise", "37.5", "16", "192"},
	} {
		args := []string{"mix", "rain", "--duration", "1.375s"}
		if tc.name != "default" {
			args = append(args, "--cue", tc.cue, "--cue-volume", tc.volume, "--cue-interval-ms", "733", "-f", tc.format, "-b", tc.bits, "-q", tc.bitrate)
		}
		name := filepath.Join("elsewhere", tc.name+".entrain")
		code, _, stderr := runCLI(t, append(append([]string{}, args...), "--save-session", name)...)
		if code != 0 {
			t.Fatalf("%s export: %s", tc.name, stderr)
		}
		if paths, err := filepath.Glob("rain*"); err != nil || len(paths) > 0 {
			t.Fatalf("source-only export rendered audio: %v", paths)
		}
		f, err := os.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		s, err := train.Read(f)
		f.Close()
		if err != nil || s.Background != "rain" || s.Cue != tc.cue || s.Duration != 1375*time.Millisecond || s.Output != tc.format {
			t.Fatalf("bad saved model: %+v, %v", s, err)
		}
		if tc.name == "muted" && (s.CueVolumePercent != 0 || s.CueIntervalMS != 733) {
			t.Fatal("lost mute or spacing")
		}
		code, _, stderr = runCLI(t, "render", name)
		if code != 0 {
			t.Fatal(stderr)
		}
		outname := "direct." + tc.format
		code, _, stderr = runCLI(t, append(append([]string{}, args...), "-o", outname)...)
		if code != 0 {
			t.Fatal(stderr)
		}
		a, err := os.ReadFile(filepath.Join("elsewhere", tc.name+"."+tc.format))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(outname)
		if err != nil || !bytes.Equal(a, b) {
			t.Fatalf("%s: direct and saved renders differ", tc.name)
		}
	}
	code, _, stderr := runCLI(t, "render", "elsewhere/muted.entrain", "-b", "16", "-o", "override.wav")
	if code != 0 {
		t.Fatal(stderr)
	}
	x, err := os.ReadFile("override.wav")
	if err != nil {
		t.Fatal(err)
	}
	_, _, depth, err := dsp.ReadWav(x)
	if err != nil || depth != 16 {
		t.Fatalf("CLI did not override bit_depth: %d, %v", depth, err)
	}
	code, _, _ = runCLI(t, "render", "elsewhere/custom.entrain", "-q", "0", "--dry-run")
	if code == 0 {
		t.Fatal("explicit invalid bitrate unexpectedly defaulted")
	}
}

func TestSaveSessionNeverOverwrites(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("existing.entrain", []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link("existing.entrain", "hard.entrain"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("existing.entrain", "linked.entrain"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing.entrain", "dangling.entrain"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"existing.entrain", "hard.entrain", "linked.entrain", "dangling.entrain"} {
		for _, dry := range []bool{false, true} {
			args := []string{"mix", "white", "--save-session", name}
			if dry {
				args = append(args, "--dry-run")
			}
			code, _, err := runCLI(t, args...)
			if code == 0 || !strings.Contains(err, "already exists") {
				t.Fatalf("%v: %v", args, err)
			}
		}
	}
	if b, err := os.ReadFile("existing.entrain"); err != nil || string(b) != "keep" {
		t.Fatal("original changed")
	}
	if _, err := os.Stat("missing.entrain"); !os.IsNotExist(err) {
		t.Fatal("wrote through dangling symlink")
	}
	start := make(chan struct{})
	results := make(chan error, 8)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- writeNewSession("raced.entrain", bytes.Repeat([]byte{byte('a' + i)}, 8000))
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	wins := 0
	for err := range results {
		if err == nil {
			wins++
		}
	}
	b, err := os.ReadFile("raced.entrain")
	if err != nil || wins != 1 || len(b) != 8000 || !bytes.Equal(b, bytes.Repeat(b[:1], 8000)) {
		t.Fatalf("race: %d winners, %d bytes, %v", wins, len(b), err)
	}
	if files, _ := filepath.Glob(".entrain-source-*"); len(files) > 0 {
		t.Fatalf("left staging files: %v", files)
	}
}

func TestSaveSessionValidationAndDryRun(t *testing.T) {
	t.Chdir(t.TempDir())
	code, out, err := runCLI(t, "mix", "pink", "--dry-run", "--save-session", "new.toml")
	if code != 0 || !strings.Contains(out, "would save new.toml") || !strings.Contains(out, "[[voice]]") {
		t.Fatal(out, err)
	}
	for _, args := range [][]string{
		{"mix", "pink", "--save-session", ""},
		{"mix", "pink", "--save-session", "new.sbg"},
		{"mix", "pink", "--save-session", "new.toml", "-o", "audio.wav"},
		{"mix", "pink", "--save-session", "new.toml", "-q", "7"},
		{"mix", "pink", "--save-session", "new.toml", "-b", "48"},
		{"mix", "unknown", "--save-session", "new.toml"},
	} {
		code, _, _ = runCLI(t, args...)
		if code == 0 {
			t.Fatalf("accepted %v", args)
		}
	}
	if paths, _ := os.ReadDir("."); len(paths) > 0 {
		t.Fatalf("dry or invalid export created %v", paths)
	}
}
