// Package train reads the versioned TOML session subset exported by Studio.
package train

import (
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"time"

	"github.com/desireco/entrain/codec"
	"github.com/desireco/entrain/site/audio"
	"github.com/pelletier/go-toml/v2"
)

const (
	DefaultCueVolume     = 100.0
	DefaultCueIntervalMS = 625 // the interval recorded in the mastered cue loops
)

type Soundscape struct {
	Background       string
	Cue              string
	Duration         time.Duration
	CueVolumePercent float64
	CueIntervalMS    int
	CueSections      []CueSection
}

func (s Soundscape) Validate() error {
	if !slices.Contains([]string{"pink", "white", "rain"}, s.Background) {
		return fmt.Errorf("unknown background %q (choose pink, white, or rain)", s.Background)
	}
	if !slices.Contains([]string{"none", "bell", "tone", "noise"}, s.Cue) {
		return fmt.Errorf("unknown cue %q (choose none, bell, tone, or noise)", s.Cue)
	}
	if err := validateCueSettings(s.CueVolumePercent, s.CueIntervalMS); err != nil {
		return err
	}
	if frames(s.Duration) < 1 {
		return fmt.Errorf("duration must cover at least one audio frame")
	}
	return s.validateSections()
}

func frames(d time.Duration) int64 {
	return int64(d/time.Second)*audio.Rate + int64(d%time.Second)*audio.Rate/int64(time.Second)
}

type Session struct {
	Title, Output        string
	BitDepth, MP3Bitrate int
	Soundscape
}

type fileDoc struct {
	Version    int        `toml:"version"`
	Title      string     `toml:"title,omitempty"`
	Output     string     `toml:"output"`
	SampleRate *int       `toml:"sample_rate,omitempty"`
	BitDepth   *int       `toml:"bit_depth,omitempty"`
	MP3Bitrate *int       `toml:"mp3_bitrate,omitempty"`
	Voice      []voiceDoc `toml:"voice"`
}

type voiceDoc struct {
	Type             string        `toml:"type"`
	Background       string        `toml:"background"`
	Cue              *string       `toml:"cue,omitempty"`
	Duration         any           `toml:"duration"`
	CueVolumePercent *float64      `toml:"cue_volume_percent,omitempty"`
	CueIntervalMS    *int          `toml:"cue_interval_ms,omitempty"`
	Sections         []sectionSpec `toml:"cue_section,omitempty"`
}

func Read(r io.Reader) (Session, error) {
	var doc fileDoc
	var result Session
	if err := toml.NewDecoder(r).DisallowUnknownFields().Decode(&doc); err != nil {
		var missing *toml.StrictMissingError
		if errors.As(err, &missing) {
			return result, fmt.Errorf("unknown TOML field:\n%s", missing.String())
		}
		return result, err
	}
	if doc.Version != 1 {
		return result, fmt.Errorf("unsupported or missing version %d (use version = 1)", doc.Version)
	}
	if doc.SampleRate != nil && *doc.SampleRate != audio.Rate {
		return result, fmt.Errorf("soundscape uses sample_rate = %d", audio.Rate)
	}
	if len(doc.Voice) != 1 || doc.Voice[0].Type != "soundscape" {
		return result, fmt.Errorf("version 1 requires one [[voice]] with type = \"soundscape\"")
	}
	if doc.Output == "" {
		doc.Output = "wav"
	}
	v := doc.Voice[0]
	duration, err := parseDuration(v.Duration)
	if err != nil {
		return result, err
	}
	cue := "none"
	if v.Cue != nil {
		cue = *v.Cue
	}
	sound := Soundscape{Background: v.Background, Cue: cue, Duration: duration,
		CueVolumePercent: DefaultCueVolume, CueIntervalMS: DefaultCueIntervalMS}
	if v.CueVolumePercent != nil {
		sound.CueVolumePercent = *v.CueVolumePercent
	}
	if v.CueIntervalMS != nil {
		sound.CueIntervalMS = *v.CueIntervalMS
	}
	for i, spec := range v.Sections {
		section, err := spec.section(sound.CueVolumePercent, sound.CueIntervalMS)
		if err != nil {
			return result, fmt.Errorf("cue_section %d: %w", i+1, err)
		}
		sound.CueSections = append(sound.CueSections, section)
	}
	result = Session{Title: doc.Title, Output: doc.Output, BitDepth: 16, MP3Bitrate: 320, Soundscape: sound}
	if doc.BitDepth != nil {
		result.BitDepth = *doc.BitDepth
	}
	if doc.MP3Bitrate != nil {
		result.MP3Bitrate = *doc.MP3Bitrate
	}
	if err := result.validateEncoding(); err != nil {
		return result, err
	}
	return result, result.Validate()
}

func parseDuration(v any) (time.Duration, error) {
	var sec float64
	switch value := v.(type) {
	case string:
		d, err := time.ParseDuration(value)
		if err != nil {
			return 0, fmt.Errorf("duration: %w", err)
		}
		return d, nil
	case int64:
		sec = float64(value)
	case float64:
		sec = value
	default:
		return 0, fmt.Errorf("duration must be a string such as \"6m\" or a number in seconds")
	}
	if math.IsNaN(sec) || sec < 0 || sec >= float64(math.MaxInt64)/float64(time.Second) {
		return 0, fmt.Errorf("duration is out of range")
	}
	return time.Duration(sec * float64(time.Second)), nil
}

// PCM repeats the matching Studio loops on their original sample clocks.
// Each endpoint has a half-second cosine taper, shortened on short sessions.
func (s Soundscape) PCM() (codec.StereoPCM, error) {
	return s.PCMFor(s.Duration)
}

// PCMFor renders a prefix with an endpoint taper while retaining the scene's
// original cue windows. Short previews do not move a cue or accelerate its fade.
func (s Soundscape) PCMFor(duration time.Duration) (codec.StereoPCM, error) {
	if err := s.Validate(); err != nil {
		return codec.StereoPCM{}, err
	}
	if duration > s.Duration || frames(duration) < 1 {
		return codec.StereoPCM{}, fmt.Errorf("render duration must contain at least one frame and fit the session")
	}
	bed, err := audio.Background(s.Background)
	if err != nil {
		return codec.StereoPCM{}, err
	}
	cues, err := s.prepareCues(duration)
	if err != nil {
		return codec.StereoPCM{}, err
	}
	frames := frames(duration)
	fade := min(frames/2, audio.Rate/2)
	var i int64
	return codec.StereoPCM{Frames: frames, Next: func() (float64, float64) {
		b := int(2 * i % int64(len(bed)))
		l, r := bed[b], bed[b+1]
		cl, cr := cues.next(i)
		l, r = l+cl, r+cr
		edge := min(i, frames-1-i)
		if edge < fade {
			gain := 0.5 - 0.5*math.Cos(math.Pi*float64(edge)/float64(fade))
			l, r = l*gain, r*gain
		}
		i++
		return l, r
	}}, nil
}

// Each ten-second cue recording holds sixteen alternating strikes. At a new
// cadence only the gap changes: replay each stereo strike at its original
// sample rate, including its recorded character and left/right placement.
type cuePlayer struct {
	samples           []float64
	gain              float64
	intervalMS        int
	strike, nextOnset int64
	pos, end          int
}

// Round up each absolute onset to the first output frame at or after its
// requested time. Independently rounding the interval would drift over time.
func onset(strike int64, millis int) int64 {
	return (strike*int64(millis)*audio.Rate + 999) / 1000
}

func (p *cuePlayer) next(frame int64) (float64, float64) {
	if frame == p.nextOnset {
		slot := p.strike % 16
		p.pos = int(2 * onset(slot, DefaultCueIntervalMS))
		p.end = int(2 * onset(slot+1, DefaultCueIntervalMS))
		p.strike++
		p.nextOnset = onset(p.strike, p.intervalMS)
	}
	if p.pos >= p.end {
		return 0, 0
	}
	l, r := p.samples[p.pos], p.samples[p.pos+1]
	p.pos += 2
	return l * p.gain, r * p.gain
}
