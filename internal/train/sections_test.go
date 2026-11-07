package train

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/desireco/entrain/site/audio"
)

const sceneSource = `version = 1
[[voice]]
type = "soundscape"
background = "rain"
duration = "65s"
cue_volume_percent = 60
cue_interval_ms = 733
`

func TestCueSectionsValidation(t *testing.T) {
	const valid = `
[[voice.cue_section]]
start = 0
duration = "4s"
cue = "bell"
`
	for _, tc := range []struct{ name, source string }{
		{"unspecified start", strings.Replace(valid, "start = 0", "", 1)},
		{"unspecified cue", strings.Replace(valid, `cue = "bell"`, "", 1)},
		{"negative start", strings.Replace(valid, "start = 0", `start = "-1s"`, 1)},
		{"outside session", strings.Replace(valid, "start = 0", "start = 64", 1)},
		{"overlap", valid + strings.Replace(valid, "start = 0", "start = 3", 1)},
		{"out of order", strings.Replace(valid, "start = 0", "start = 5", 1) + valid},
		{"fade longer than section", valid + "fade_in = 3\nfade_out = 2\n"},
		{"negative fade", valid + "fade_out = \"-0.1s\"\n"},
		{"bad timbre", strings.Replace(valid, `"bell"`, `"whistle"`, 1)},
		{"too short", strings.Replace(valid, `"4s"`, `"1ns"`, 1)},
		{"bad spacing", valid + "cue_interval_ms = 15\n"},
		{"bad gain", valid + "cue_volume_percent = -1\n"},
		{"unknown key", valid + "bad_key = 2\n"},
		{"continuous cue conflict", "cue = \"tone\"\n" + valid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Read(strings.NewReader(sceneSource + tc.source)); err == nil {
				t.Fatalf("unexpectedly accepted %s", tc.source)
			}
		})
	}
	s, err := Read(strings.NewReader(sceneSource + valid + strings.Replace(valid, "start = 0", "start = 4", 1) + "cue_volume_percent = 0\n"))
	if err != nil || len(s.CueSections) != 2 {
		t.Fatalf("touching sections: %v", err)
	}
	if a := s.CueSections[0]; a.CueIntervalMS != 733 || a.CueVolumePercent != 60 || a.FadeOut != 10*time.Millisecond {
		t.Fatalf("unexpected defaults: %+v", a)
	}
	if s.CueSections[1].CueVolumePercent != 0 {
		t.Fatal("explicit zero did not mute the second section")
	}
}

// Compare with the same evolving rain rendered alone and independently
// addressed cue assets. The bell spans the one-minute mark.
func TestSectionsKeepBedAndCueClocks(t *testing.T) {
	const score = `
[[voice.cue_section]]
cue = "tone"
start = "3s"
duration = "1s"
fade_out = "0s"

[[voice.cue_section]]
cue = "bell"
start = "58.10101s"
duration = "5.11s"
fade_in = "310ms"
fade_out = "320ms"
cue_volume_percent = 75
`
	s, err := Read(strings.NewReader(sceneSource + score))
	if err != nil {
		t.Fatal(err)
	}
	pcm, err := s.PCM()
	if err != nil {
		t.Fatal(err)
	}
	alone := s.Soundscape
	alone.CueSections = nil
	bed, err := alone.PCM()
	if err != nil {
		t.Fatal(err)
	}
	bell, err := audio.Cue("bell")
	if err != nil {
		t.Fatal(err)
	}
	tone, err := audio.Cue("tone")
	if err != nil {
		t.Fatal(err)
	}
	first := int64(math.Ceil(58.10101 * audio.Rate))
	last := int64(math.Ceil(63.21101 * audio.Rate))
	for i := range pcm.Frames {
		l, r := pcm.Next()
		wantL, wantR := bed.Next()
		if i < audio.Rate/2 || i >= pcm.Frames-audio.Rate/2 {
			continue
		}
		pos, gain, strikes := int64(-1), 0.0, bell
		switch {
		case i >= 3*audio.Rate && i < 4*audio.Rate:
			pos, gain, strikes = i-3*audio.Rate, 0.6, tone
		case i >= first && i < last:
			pos, gain = i-first, 0.75
			progress := min(1.0, float64(pos)/(.310*audio.Rate), float64(last-1-i)/(.320*audio.Rate))
			gain *= 0.5 - 0.5*math.Cos(math.Pi*progress)
		}
		if pos >= 0 {
			n := pos * 1000 / (733 * audio.Rate)
			offset := pos - int64(math.Ceil(float64(n)*.733*audio.Rate))
			slot := n % 16
			src := int64(math.Ceil(float64(slot)*.625*audio.Rate)) + offset
			if src < int64(math.Ceil(float64(slot+1)*.625*audio.Rate)) {
				wantL += strikes[2*src] * gain
				wantR += strikes[2*src+1] * gain
			}
		}
		if math.Abs(l-wantL) > 1e-12 || math.Abs(r-wantR) > 1e-12 {
			t.Fatalf("frame %d got %g,%g want %g,%g", i, l, r, wantL, wantR)
		}
	}
}

func TestTimelinePreviewKeepsAuthoredCues(t *testing.T) {
	s, err := Read(strings.NewReader(sceneSource + `
[[voice.cue_section]]
start = 3
duration = 8
cue = "bell"
fade_out = 3
`))
	if err != nil {
		t.Fatal(err)
	}
	full, err := s.PCM()
	if err != nil {
		t.Fatal(err)
	}
	short, err := s.PCMFor(6 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for i := range short.Frames {
		sl, sr := short.Next()
		fl, fr := full.Next()
		if i < short.Frames-audio.Rate/2 && (sl != fl || sr != fr) {
			t.Fatalf("preview changed cue at %d", i)
		}
		if i == short.Frames-1 && (sl != 0 || sr != 0) {
			t.Fatal("preview ended without fade")
		}
	}
	before, err := s.PCMFor(time.Second)
	if err != nil || before.Frames != audio.Rate {
		t.Fatalf("preview before section: %v", err)
	}
}
