package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/desireco/entrain/internal/dsp"
)

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestNoisemakerRendersExactDuration(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "a.wav")
	code, stdout, errOut := runCLI(t, "brown", "2s", "-o", out, "--seed", "5")
	if code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, errOut)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	sr, x, _, err := dsp.ReadWav(raw)
	if err != nil {
		t.Fatal(err)
	}
	if sr != 44100 || len(x) != 2*2*44100 {
		t.Fatalf("sr=%d frames=%d, want 44100/%d", sr, len(x)/2, 2*44100)
	}
	l, _ := dsp.Deinterleave(x)
	if r := dsp.RMS(l); r < 0.9*0.155 || r > 1.1*0.155 {
		t.Errorf("RMS = %.4f, want ≈0.155 (default bed level)", r)
	}
}

func TestNoisemakerFormatsAndDurations(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		args   []string
		want   string
		frames int
	}{
		{[]string{"pink", "0:01", "-f", "flac", "-o", filepath.Join(dir, "a.flac")}, "fLaC", 44100},
		{[]string{"alpha", "1s", "--alpha", "1", "-f", "mp3", "-q", "128", "-o", filepath.Join(dir, "a.mp3")}, "\xff\xfb", 44100},
		{[]string{"white", "1500ms", "-o", filepath.Join(dir, "a.wav")}, "RIFF", 66150},
	} {
		code, _, errOut := runCLI(t, tc.args...)
		if code != 0 {
			t.Fatalf("%v: code=%d stderr=%q", tc.args, code, errOut)
		}
		b, err := os.ReadFile(tc.args[len(tc.args)-1])
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(b), tc.want) {
			t.Errorf("%v: signature %q", tc.args, b[:4])
		}
	}
}

func TestNoisemakerDeterministic(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.wav"), filepath.Join(dir, "b.wav")
	runCLI(t, "pink", "1s", "--seed", "9", "-o", a)
	runCLI(t, "pink", "1s", "--seed", "9", "-o", b)
	ba, bb := readFile(t, a), readFile(t, b)
	if !bytes.Equal(ba, bb) {
		t.Error("same seed produced different bytes")
	}
}

func TestNoisemakerErrors(t *testing.T) {
	if code, _, _ := runCLI(t, "violet", "1s"); code != 1 {
		t.Errorf("unknown color: code=%d", code)
	}
	if code, _, _ := runCLI(t, "pink", "nope"); code != 2 {
		t.Errorf("bad duration: code=%d", code)
	}
	if code, _, _ := runCLI(t, "pink"); code != 2 {
		t.Errorf("missing args: code=%d", code)
	}
}

func readFile(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
