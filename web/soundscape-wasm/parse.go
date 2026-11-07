//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"
	"time"

	"github.com/desireco/entrain/internal/train"
	"github.com/desireco/entrain/site/audio"
)

// Pass only normalized values across the worker boundary. The editor owns
// the original TOML string, including its comments, spacing and newlines.
func parseSession(_ js.Value, args []js.Value) any {
	s, err := train.Read(strings.NewReader(args[0].String()))
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
	sections := make([]any, len(s.CueSections))
	for i, sec := range s.CueSections {
		sections[i] = map[string]any{
			"start": ms(sec.Start), "length": ms(sec.Duration),
			"cue": sec.Cue, "volume": sec.CueVolumePercent, "interval": sec.CueIntervalMS,
			"fadeIn": ms(sec.FadeIn), "fadeOut": ms(sec.FadeOut),
		}
	}
	return map[string]any{"session": map[string]any{
		"title": s.Title, "output": s.Output, "bitDepth": s.BitDepth, "mp3Bitrate": s.MP3Bitrate,
		"sampleRate": audio.Rate, "background": s.Background, "durationMS": ms(s.Duration),
		"seed": s.Seed, "cue": s.Cue, "volume": s.CueVolumePercent, "interval": s.CueIntervalMS,
		"sections": sections,
	}}
}
