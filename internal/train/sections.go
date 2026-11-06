package train

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/desireco/entrain/site/audio"
)

// CueSection plays one timbre against the uninterrupted session background.
type CueSection struct {
	Start, Duration  time.Duration
	FadeIn, FadeOut  time.Duration
	Cue              string
	CueVolumePercent float64
	CueIntervalMS    int
}

type sectionSpec struct {
	Start    any      `toml:"start"`
	Duration any      `toml:"duration"`
	FadeIn   any      `toml:"fade_in"`
	FadeOut  any      `toml:"fade_out"`
	Cue      string   `toml:"cue"`
	Volume   *float64 `toml:"cue_volume_percent"`
	Interval *int     `toml:"cue_interval_ms"`
}

func (spec sectionSpec) section(volume float64, interval int) (CueSection, error) {
	sec := CueSection{Cue: spec.Cue, CueVolumePercent: volume, CueIntervalMS: interval}
	if spec.Volume != nil {
		sec.CueVolumePercent = *spec.Volume
	}
	if spec.Interval != nil {
		sec.CueIntervalMS = *spec.Interval
	}
	var err error
	if sec.Start, err = parseDuration(spec.Start); err != nil {
		return sec, fmt.Errorf("start: %w", err)
	}
	if sec.Duration, err = parseDuration(spec.Duration); err != nil {
		return sec, err
	}
	if spec.FadeIn != nil {
		if sec.FadeIn, err = parseDuration(spec.FadeIn); err != nil {
			return sec, fmt.Errorf("fade_in: %w", err)
		}
	}
	// A short release avoids cutting a ringing bell at an arbitrary endpoint.
	sec.FadeOut = min(10*time.Millisecond, max(0, sec.Duration-sec.FadeIn))
	if spec.FadeOut != nil {
		if sec.FadeOut, err = parseDuration(spec.FadeOut); err != nil {
			return sec, fmt.Errorf("fade_out: %w", err)
		}
	}
	return sec, nil
}

func validateCueSettings(volume float64, interval int) error {
	if !(volume >= 0 && volume <= 200) {
		return fmt.Errorf("cue_volume_percent must be between 0 and 200")
	}
	if interval < 350 || interval > 2000 {
		return fmt.Errorf("cue_interval_ms must be an integer between 350 and 2000")
	}
	return nil
}

func (s Soundscape) validateSections() error {
	if len(s.CueSections) > 0 && s.Cue != "none" {
		return fmt.Errorf("choose either a continuous cue or cue_section entries")
	}
	var end time.Duration
	for i, sec := range s.CueSections {
		var reason string
		switch {
		case !slices.Contains([]string{"tone", "bell", "noise"}, sec.Cue):
			reason = "cue must be tone, bell, or noise"
		case sec.Start < 0 || sec.Start > s.Duration || sec.Duration > s.Duration-sec.Start || frames(sec.Duration) < 1:
			reason = "start and duration must cover at least one frame inside the session"
		case firstFrame(sec.Start) >= frames(s.Duration):
			reason = "section begins after the last rendered frame"
		case sec.Start < end:
			reason = "sections must be in order and cannot overlap"
		case sec.FadeIn < 0 || sec.FadeOut < 0 || sec.FadeIn > sec.Duration || sec.FadeOut > sec.Duration-sec.FadeIn:
			reason = "fade_in and fade_out must be nonnegative and fit inside the section"
		}
		if reason != "" {
			return fmt.Errorf("cue_section %d: %s", i+1, reason)
		}
		if err := validateCueSettings(sec.CueVolumePercent, sec.CueIntervalMS); err != nil {
			return fmt.Errorf("cue_section %d: %w", i+1, err)
		}
		end = sec.Start + sec.Duration
	}
	return nil
}

type cueWindow struct {
	start, end, fadeIn, fadeOut int64
	player                      cuePlayer
}

type cueTimeline []cueWindow

// First sample at or after the requested instant. Split seconds from the
// fractional part to keep long sessions within int64.
func firstFrame(d time.Duration) int64 {
	return int64(d/time.Second)*audio.Rate + (int64(d%time.Second)*audio.Rate+int64(time.Second)-1)/int64(time.Second)
}

func (s Soundscape) prepareCues(limit time.Duration) (cueTimeline, error) {
	sections := s.CueSections
	if s.Cue != "none" {
		sections = []CueSection{{Duration: s.Duration, Cue: s.Cue, CueVolumePercent: s.CueVolumePercent, CueIntervalMS: s.CueIntervalMS}}
	}
	var timeline cueTimeline
	assets := make(map[string][]float64)
	for _, sec := range sections {
		if sec.Start >= limit || sec.CueVolumePercent == 0 {
			continue
		}
		samples := assets[sec.Cue]
		if samples == nil {
			var err error
			if samples, err = audio.Cue(sec.Cue); err != nil {
				return nil, err
			}
			assets[sec.Cue] = samples
		}
		timeline = append(timeline, cueWindow{
			start: firstFrame(sec.Start), end: firstFrame(sec.Start + sec.Duration),
			fadeIn: firstFrame(sec.FadeIn), fadeOut: firstFrame(sec.FadeOut),
			player: cuePlayer{samples: samples, gain: sec.CueVolumePercent / 100, intervalMS: sec.CueIntervalMS},
		})
	}
	return timeline, nil
}

func (t *cueTimeline) next(frame int64) (float64, float64) {
	for len(*t) > 0 && frame >= (*t)[0].end {
		*t = (*t)[1:]
	}
	if len(*t) == 0 || frame < (*t)[0].start {
		return 0, 0
	}
	w := &(*t)[0]
	elapsed, remaining := frame-w.start, w.end-1-frame
	l, r := w.player.next(elapsed)
	gain := 1.0
	if elapsed < w.fadeIn {
		gain = .5 - .5*math.Cos(math.Pi*float64(elapsed)/float64(w.fadeIn))
	} else if remaining < w.fadeOut {
		gain = .5 - .5*math.Cos(math.Pi*float64(remaining)/float64(w.fadeOut))
	}
	return l * gain, r * gain
}
