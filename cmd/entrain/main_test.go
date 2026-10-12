package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/desireco/entrain/internal/dsp"
)

// runCLI runs the command in-process and returns (exit code, stdout, stderr).
func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestVersion(t *testing.T) {
	code, out, _ := runCLI(t, "version")
	if code != 0 || !strings.HasPrefix(out, "entrain ") {
		t.Fatalf("version: code=%d out=%q", code, out)
	}
}

func TestNoArgsShowsUsage(t *testing.T) {
	code, _, errOut := runCLI(t)
	if code != 2 || !strings.Contains(errOut, "Usage:") {
		t.Fatalf("no args: code=%d stderr=%q", code, errOut)
	}
}

func TestDryRunGolden(t *testing.T) {
	code, out, errOut := runCLI(t, "render", "testdata/basic.sbg", "--dry-run")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	want := []string{
		`plan: 0:00:00 → 0:01:00 (0:01:00), 2 event(s), 3 voice(s), crossfade 0:00:30`,
		``,
		`  time      fade  tone-set`,
		`  0:00:00   --    focus        pink/20 180+40/30 320+24/25`,
		`  0:01:00   --    all-off      -`,
	}
	if len(lines) != len(want) {
		t.Fatalf("plan output:\n%s", out)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d:\n  got  %q\n  want %q", i, lines[i], w)
		}
	}
	// The 40 Hz gamma beat lints (informational, to stderr).
	if !strings.Contains(errOut, "beat 40 Hz") {
		t.Errorf("stderr should carry the gamma-beat lint, got %q", errOut)
	}
}

func TestBareNameShorthand(t *testing.T) {
	// `entrain focus-10` resolves focus-10.sbg in the working directory.
	t.Chdir("testdata")
	code, _, errOut := runCLI(t, "focus-10", "--dry-run")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	code, _, _ = runCLI(t, "nosuch", "--dry-run")
	if code == 0 {
		t.Fatal("missing session should fail")
	}
}

func TestRenderWritesWav(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.wav")
	code, stdout, errOut := runCLI(t, "render", "testdata/basic.sbg", "-L", "0:00:05", "-o", out)
	if code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, errOut)
	}
	if !strings.Contains(stdout, "wrote "+out) {
		t.Errorf("summary missing from stdout: %q", stdout)
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	sr, x, err := dsp.ReadWav16(raw)
	if err != nil {
		t.Fatal(err)
	}
	if sr != 44100 || len(x) != 2*5*44100 {
		t.Fatalf("wav: sr=%d frames=%d, want 44100/%d", sr, len(x)/2, 5*44100)
	}
	l, _ := dsp.Deinterleave(x)
	// 5 s render, 1 s fade-in/out halves: full level mid-window; the
	// gamma high tone lives on the left ear (200 Hz).
	if got := dsp.Goertzel(l[44100:4*44100], 44100, 200); got < 0.2*float64(3*44100)/2 {
		t.Errorf("L@200 = %.0f, tone missing", got)
	}
}

func TestRenderFormats(t *testing.T) {
	// Same 5 s render through every format; verify each file's signature.
	dir := t.TempDir()
	for _, tc := range []struct {
		format  string
		bitrate int
		want    string
	}{
		{"wav", 0, "RIFF"},
		{"flac", 0, "fLaC"},
		{"mp3", 320, "\xff\xfb"},
	} {
		out := filepath.Join(dir, "out."+tc.format)
		args := []string{"render", "testdata/basic.sbg", "-L", "0:00:05", "-f", tc.format, "-o", out}
		if tc.bitrate > 0 {
			args = append(args, "-q", strconv.Itoa(tc.bitrate))
		}
		code, stdout, errOut := runCLI(t, args...)
		if code != 0 {
			t.Fatalf("%s: code=%d stdout=%q stderr=%q", tc.format, code, stdout, errOut)
		}
		raw, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stdout, "wrote "+out) || !strings.Contains(stdout, tc.format) {
			t.Errorf("%s: summary %q", tc.format, stdout)
		}
		if !strings.HasPrefix(string(raw), tc.want) {
			t.Errorf("%s: signature %q, want prefix %q", tc.format, raw[:4], tc.want)
		}
	}

	// Default output name follows the format: basic.flac next to the input.
	code, stdout, errOut := runCLI(t, "render", "testdata/basic.sbg", "-L", "0:00:02", "-f", "flac")
	if code != 0 {
		t.Fatalf("default-name: code=%d stderr=%q", code, errOut)
	}
	if !strings.Contains(stdout, "testdata/basic.flac") {
		t.Errorf("default flac name missing: %q", stdout)
	}
	os.Remove("testdata/basic.flac")
}

func TestBadFormatAndBitrate(t *testing.T) {
	code, _, errOut := runCLI(t, "render", "testdata/basic.sbg", "-f", "ogg")
	if code != 2 || !strings.Contains(errOut, "format") {
		t.Fatalf("format: code=%d stderr=%q", code, errOut)
	}
	code, _, errOut = runCLI(t, "render", "testdata/basic.sbg", "-f", "mp3", "-q", "321")
	if code != 1 || !strings.Contains(errOut, "bitrate") {
		t.Fatalf("bitrate: code=%d stderr=%q", code, errOut)
	}
}

func TestLengthFlagErrors(t *testing.T) {
	code, _, errOut := runCLI(t, "render", "testdata/basic.sbg", "-L", "garbage")
	if code != 1 || !strings.Contains(errOut, "bad length") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
}

func TestUnsupportedRate(t *testing.T) {
	code, _, errOut := runCLI(t, "render", "testdata/basic.sbg", "-r", "96000")
	if code != 2 || !strings.Contains(errOut, "sample rate") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
}
