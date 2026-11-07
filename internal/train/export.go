package train

import (
	"fmt"
	"slices"

	"github.com/desireco/entrain/codec"
	"github.com/desireco/entrain/site/audio"
	"github.com/pelletier/go-toml/v2"
)

func (s Session) validateEncoding() error {
	if !slices.Contains([]string{"wav", "flac", "mp3"}, s.Output) {
		return fmt.Errorf("unsupported output %q (choose wav, flac, or mp3)", s.Output)
	}
	if s.BitDepth != 16 && s.BitDepth != 24 {
		return fmt.Errorf("unsupported bit_depth %d (choose 16 or 24)", s.BitDepth)
	}
	if err := codec.ValidateMp3(audio.Rate, s.MP3Bitrate); err != nil {
		return fmt.Errorf("mp3_bitrate: %w", err)
	}
	return nil
}

// MarshalMix writes a direct CLI mix using the same wire schema as Read.
// Explicit values keep a muted cue distinct from an omitted setting.
func MarshalMix(s Session) ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if err := s.validateEncoding(); err != nil {
		return nil, err
	}
	if len(s.CueSections) != 0 {
		return nil, fmt.Errorf("mix source export expects a single continuous cue")
	}
	sampleRate := audio.Rate
	doc := fileDoc{
		Version: 1, Title: s.Title, Output: s.Output, SampleRate: &sampleRate,
		BitDepth: &s.BitDepth, MP3Bitrate: &s.MP3Bitrate,
		Voice: []voiceDoc{{Type: "soundscape", Background: s.Background, Seed: s.Seed, Duration: s.Duration.String(),
			Cue: &s.Cue, CueVolumePercent: &s.CueVolumePercent, CueIntervalMS: &s.CueIntervalMS}},
	}
	return toml.Marshal(doc)
}
