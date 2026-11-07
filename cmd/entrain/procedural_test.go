package main

import (
	"bytes"
	"os"
	"testing"
)

func TestProceduralChoicesSurviveSaveAndRender(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, kind := range []string{"wind", "surf"} {
		args := []string{"mix", kind, "--cue", "bell", "--cue-volume", "55", "--cue-interval-ms", "750", "--duration", "2.135s"}
		name := kind + ".entrain"
		run := func(args ...string) {
			t.Helper()
			if code, _, err := runCLI(t, args...); code != 0 {
				t.Fatal(err)
			}
		}
		run(append(append([]string(nil), args...), "--save-session", name)...)
		run("render", name, "-o", "saved.wav")
		run(append(append([]string(nil), args...), "-o", "direct.wav")...)
		a, err := os.ReadFile("direct.wav")
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile("saved.wav")
		if err != nil || !bytes.Equal(a, b) {
			t.Fatalf("%s round-trip changed audio: %v", kind, err)
		}
	}
}

func TestSeedCLIAndNative(t *testing.T) {
	t.Chdir(t.TempDir())
	args := []string{"mix", "surf", "--seed", "2147483647", "--cue", "bell", "--duration", "2.25s"}
	code, _, why := runCLI(t, append(append([]string{}, args...), "--save-session", "seeded.entrain")...)
	if code != 0 {
		t.Fatal(why)
	}
	source, err := os.ReadFile("seeded.entrain")
	if err != nil || !bytes.Contains(source, []byte("seed = 2147483647")) {
		t.Fatalf("saved seed: %s, %v", source, err)
	}
	for _, run := range [][]string{append(args, "-o", "direct.wav"), {"render", "seeded.entrain", "-o", "native.wav"}} {
		if code, _, why := runCLI(t, run...); code != 0 {
			t.Fatal(why)
		}
	}
	direct, _ := os.ReadFile("direct.wav")
	native, _ := os.ReadFile("native.wav")
	if !bytes.Equal(direct, native) {
		t.Fatal("seeded native and direct exports differ")
	}
	if code, _, why := runCLI(t, "mix", "surf", "--cue", "bell", "--duration", "2.25s", "-o", "default.wav"); code != 0 {
		t.Fatal(why)
	}
	original, _ := os.ReadFile("default.wav")
	if bytes.Equal(original, direct) {
		t.Fatal("positive seed had no effect")
	}
	for _, name := range []string{"pink", "white", "rain-loop"} {
		if code, _, _ := runCLI(t, "mix", name, "--seed", "1", "--dry-run"); code == 0 {
			t.Fatalf("%s accepted unsupported seed", name)
		}
		if code, _, why := runCLI(t, "mix", name, "--seed", "0", "--dry-run"); code != 0 {
			t.Fatal(why)
		}
	}
	for _, val := range []string{"-1", "2.5", "2147483648"} {
		if code, _, _ := runCLI(t, "mix", "rain", "--seed", val, "--dry-run"); code == 0 {
			t.Errorf("accepted seed %s", val)
		}
		bad := bytes.Replace(source, []byte("seed = 2147483647"), []byte("seed = "+val), 1)
		if err := os.WriteFile("bad.entrain", bad, 0600); err != nil {
			t.Fatal(err)
		}
		if code, _, _ := runCLI(t, "bad.entrain", "--dry-run"); code == 0 {
			t.Errorf("native accepted seed %s", val)
		}
	}
}
