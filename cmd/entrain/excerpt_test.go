package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/desireco/entrain/internal/dsp"
	"github.com/mewkiz/flac"
)

const excerptSource = `version = 1
[[voice]]
type = "soundscape"
background = "surf"
duration = "4.25s"
[[voice.cue_section]]
start = "0.6s"
duration = "2.5s"
cue = "bell"
cue_interval_ms = 733
cue_volume_percent = 76
fade_in = "0.1s"
fade_out = "0.3s"
`

func exactSamples(t *testing.T, path string) []int32 {
	t.Helper()
	var samples []int32
	if strings.HasSuffix(path, ".wav") {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		_, pcm, bits, err := dsp.ReadWav(b)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range pcm {
			samples = append(samples, int32(v*math.Exp2(float64(bits-1))))
		}
		return samples
	}
	stream, err := flac.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	for {
		f, err := stream.ParseNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		for i, l := range f.Subframes[0].Samples {
			samples = append(samples, l, f.Subframes[1].Samples[i])
		}
	}
	return samples
}

func TestNativeLosslessExcerpts(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("scene.entrain", []byte(excerptSource), 0600); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{"wav", "flac"} {
		for _, bits := range []int{16, 24} {
			t.Run(fmt.Sprintf("%s%d", ext, bits), func(t *testing.T) {
				whole := "full." + ext
				code, _, stderr := runCLI(t, "render", "scene.entrain", "-b", fmt.Sprint(bits), "-o", whole)
				if code != 0 {
					t.Fatal(stderr)
				}
				full := exactSamples(t, whole)
				for _, tc := range []struct {
					start, length string
					first, last   int
				}{
					{"0", "0.7s", 0, 30870},            // actual attack; no new release at cut
					{"0.50001s", "0.3s", 22051, 35281}, // straddles cue entrance
					{"1.44s", "100ms", 63504, 67914},   // mid-strike on the right
					{"3.02s", "250ms", 133182, 144207}, // crosses end of cue section
					{"4s", "60s", 176400, 187425},      // trim at natural end
					{"4s", "", 176400, 187425},         // omitted length, same natural end
				} {
					args := []string{"render", "scene.entrain", "--start", tc.start, "-b", fmt.Sprint(bits), "-f", ext}
					if tc.length != "" {
						args = append(args, "-L", tc.length)
					}
					code, out, stderr := runCLI(t, args...)
					if code != 0 || !strings.Contains(out, fmt.Sprintf("source frames [%d, %d)", tc.first, tc.last)) {
						t.Fatalf("%v %d %s %s", args, code, out, stderr)
					}
					got := exactSamples(t, "scene.excerpt."+ext)
					want := full[2*tc.first : 2*tc.last]
					if len(got) != len(want) {
						t.Fatalf("wrong frame count %d want %d", len(got)/2, len(want)/2)
					}
					for i := range got {
						if got[i] != want[i] {
							t.Fatalf("%s sample %d != master: %d vs %d", tc.start, i, got[i], want[i])
						}
					}
				}
			})
		}
	}
}

func TestInvalidSliceBeforeWriting(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("scene.toml", []byte(excerptSource), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("legacy.sbg", []byte("x: pink/10\nNOW x\n+0:00:02 off\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"scene.toml", "--start", "-1ms"}, {"scene.toml", "--start", "4.25s"}, {"scene.toml", "--start", "5s"},
		{"scene.toml", "--start", "4.24999999s"}, {"scene.toml", "--start", "1.1s", "-L", "0"},
		{"scene.toml", "--start", "1.00000001s", "-L", "1ns"},
		{"scene.toml", "--start", "abc"}, {"scene.toml", "--start", "18446744073709551616:00"},
		{"scene.toml", "--start", "1:70:00"}, {"scene.toml", "--start="}, {"scene.toml", "--start=0", "-L="}, {"legacy.sbg", "--start", "1s"},
	} {
		for _, dry := range []bool{false, true} {
			cmd := append([]string{"render"}, args...)
			cmd = append(cmd, "-o", "previous.wav")
			if dry {
				cmd = append(cmd, "--dry-run")
			}
			if err := os.WriteFile("previous.wav", []byte("previous file"), 0600); err != nil {
				t.Fatal(err)
			}
			code, _, _ := runCLI(t, cmd...)
			if code == 0 {
				t.Fatalf("accepted %v", cmd)
			}
			b, err := os.ReadFile("previous.wav")
			if err != nil || !bytes.Equal(b, []byte("previous file")) {
				t.Fatalf("changed destination for invalid %v", cmd)
			}
		}
	}
	code, out, stderr := runCLI(t, "render", "scene.toml", "--start", "0:00:01", "-L", "0:00:02", "--dry-run")
	if code != 0 || !strings.Contains(out, "source frames [44100, 132300)") {
		t.Fatalf("clock window: %d %s %s", code, out, stderr)
	}
	if _, err := os.Stat("scene.excerpt.wav"); !os.IsNotExist(err) {
		t.Fatalf("dry run created output: %v", err)
	}
}

func TestSeededProceduralExcerptUsesOriginalTake(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, kind := range []string{"rain", "wind", "surf"} {
		source := strings.Replace(excerptSource, `background = "surf"`, fmt.Sprintf("background = %q\nseed = 83027", kind), 1)
		if err := os.WriteFile("seeded.entrain", []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := runCLI(t, "render", "seeded.entrain", "-b", "24", "-o", "full.flac")
		if code != 0 {
			t.Fatal(stderr)
		}
		code, _, stderr = runCLI(t, "render", "seeded.entrain", "-b", "24", "--start", "1.44s", "-L", "100ms", "-o", "portion.wav")
		if code != 0 {
			t.Fatal(stderr)
		}
		full, cut := exactSamples(t, "full.flac"), exactSamples(t, "portion.wav")
		want := full[2*63504 : 2*67914]
		if len(cut) != len(want) {
			t.Fatal("wrong excerpt sample count")
		}
		for i := range want {
			if cut[i] != want[i] {
				t.Fatalf("%s mismatched the seeded master at sample %d", kind, i)
			}
		}
	}
}
