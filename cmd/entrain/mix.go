package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/desireco/entrain/codec"
	"github.com/desireco/entrain/internal/train"
	"github.com/desireco/entrain/site/audio"
	"github.com/desireco/entrain/synth"
)

const mixUsage = `Usage: entrain mix pink|white|rain|wind|surf|rain-loop [flags]

Render 44.1 kHz stereo. Rain, wind and surf evolve continuously; rain-loop uses the v0.2.0 master.
  --cue none|bell|tone|noise  optional alternating cue (default none)
  --cue-volume percent        cue amplitude, 0–200 (default 100; 0 mutes)
  --cue-interval-ms ms        milliseconds between L/R onsets, 350–2000 (default 625)
  --seed integer              deterministic rain/wind/surf variation, 0–2147483647 (default 0)
  --duration 6m               examples: 20s, 6m, 1h30m
  -o file                     default <background>[-<cue>].<format>
  -f wav|flac|mp3             defaults to the -o extension, then WAV
  -b 16|24                    WAV/FLAC sample depth (default 16)
  -q kbps                     MP3 bitrate (default 320)
  --save-session path         create a new .entrain/.toml source only; render it later
  --dry-run                   describe the requested action without writing a file

For a Studio download, use: entrain render session.entrain -o session.flac`

type mixOutput struct {
	Path, Format  string
	Bits, Bitrate int
	BitrateSet    bool // only a supplied -q overrides a native session default
	DryRun        bool
	LengthLimit   *time.Duration
	Start         *time.Duration
}

func cmdMix(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, mixUsage)
		return 2
	}
	if args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(stdout, mixUsage)
		return 0
	}
	fs := flag.NewFlagSet("mix", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, mixUsage) }
	cfg := mixOutput{}
	var savePath string
	fs.Func("save-session", "create a native source file instead of rendering audio", func(path string) error {
		if path == "" {
			return errors.New("a source filename is required")
		}
		savePath = path
		return nil
	})
	fs.StringVar(&cfg.Path, "o", "", "output path")
	fs.StringVar(&cfg.Format, "f", "", "output format")
	fs.IntVar(&cfg.Bits, "b", 16, "WAV/FLAC bit depth")
	fs.IntVar(&cfg.Bitrate, "q", 320, "MP3 bitrate")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "print a mix description")
	cue := fs.String("cue", "none", "optional bilateral cue")
	volume := fs.Float64("cue-volume", train.DefaultCueVolume, "cue amplitude percentage")
	interval := fs.Int("cue-interval-ms", train.DefaultCueIntervalMS, "time between cue onsets in milliseconds")
	seed := fs.Int64("seed", 0, "procedural background seed (0–2147483647)")
	duration := fs.Duration("duration", 6*time.Minute, "duration")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, mixUsage)
		return 2
	}
	s := train.Soundscape{Background: args[0], Seed: *seed, Cue: *cue, Duration: *duration,
		CueVolumePercent: *volume, CueIntervalMS: *interval}
	if savePath != "" {
		return cmdSaveSession(s, cfg, savePath, stdout, stderr)
	}
	stem := s.Background
	if s.Cue != "none" {
		stem += "-" + s.Cue
	}
	return renderSoundscape(train.Session{Soundscape: s, Output: "wav"}, cfg, stem, "", stdout, stderr)
}

func isNative(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".entrain" || ext == ".toml"
}

func cmdTrain(path, limit string, rate int, output mixOutput, stdout, stderr io.Writer) int {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(stderr, "entrain:", err)
		return 1
	}
	defer f.Close()
	s, err := train.Read(f)
	if err != nil {
		fmt.Fprintf(stderr, "entrain: %s: %v\n", path, err)
		return 1
	}
	if output.Bits == 0 {
		output.Bits = s.BitDepth
	}
	if !output.BitrateSet {
		output.Bitrate = s.MP3Bitrate
	}
	if rate != audio.Rate {
		fmt.Fprintf(stderr, "entrain: soundscape renders at %d Hz\n", audio.Rate)
		return 2
	}
	if limit != "" {
		length, err := parseNativeTime(limit)
		if err != nil {
			fmt.Fprintln(stderr, "entrain:", err)
			return 2
		}
		output.LengthLimit = &length
	}
	return renderSoundscape(s, output, strings.TrimSuffix(path, filepath.Ext(path)), path, stdout, stderr)
}

func renderSoundscape(s train.Session, cfg mixOutput, stem, source string, stdout, stderr io.Writer) int {
	if err := s.Validate(); err != nil {
		fmt.Fprintln(stderr, "entrain:", err)
		return 2
	}
	formats := []string{"wav", "flac", "mp3"}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(cfg.Path), "."))
	if cfg.Format == "" {
		cfg.Format = s.Output
		if slices.Contains(formats, ext) {
			cfg.Format = ext
		}
	}
	if !slices.Contains(formats, cfg.Format) || (slices.Contains(formats, ext) && cfg.Format != ext) {
		fmt.Fprintf(stderr, "entrain: select matching output and format (wav, flac, or mp3); got %s and %q\n", cfg.Path, cfg.Format)
		return 2
	}
	if cfg.Bits != 16 && cfg.Bits != 24 {
		fmt.Fprintf(stderr, "entrain: unsupported bit depth %d (16 or 24)\n", cfg.Bits)
		return 2
	}
	if cfg.Path == "" {
		if cfg.Start != nil {
			stem += ".excerpt"
		}
		cfg.Path = stem + "." + cfg.Format
	}
	if source != "" {
		if err := checkOutput(source, cfg.Path); err != nil {
			fmt.Fprintln(stderr, "entrain:", err)
			return 2
		}
	}
	if cfg.Format == "mp3" {
		if err := codec.ValidateMp3(audio.Rate, cfg.Bitrate); err != nil {
			fmt.Fprintln(stderr, "entrain:", err)
			return 1
		}
	}
	start := time.Duration(0)
	duration := s.Duration
	var excerpt train.Excerpt
	if cfg.Start != nil {
		var err error
		excerpt, err = s.Excerpt(*cfg.Start, cfg.LengthLimit)
		if err != nil {
			fmt.Fprintln(stderr, "entrain:", err)
			return 2
		}
		start, duration = excerpt.Start, excerpt.End-excerpt.Start
	} else if cfg.LengthLimit != nil {
		duration = min(duration, *cfg.LengthLimit)
		if duration < (time.Second+audio.Rate-1)/audio.Rate {
			fmt.Fprintln(stderr, "entrain: render length must contain at least one audio frame")
			return 2
		}
	}
	if s.Title != "" {
		fmt.Fprintf(stdout, "%s\n", s.Title)
	}
	cue := s.Cue
	if cue != "none" {
		cue = fmt.Sprintf("%s at %g%%, every %d ms", s.Cue, s.CueVolumePercent, s.CueIntervalMS)
	}
	if len(s.CueSections) > 0 {
		cue = "scheduled"
	}
	background := s.Background
	if synth.IsProcedural(background) {
		background = fmt.Sprintf("%s (seed %d)", background, s.Seed)
	}
	fmt.Fprintf(stdout, "mix: %s | cue %s | %s | %d Hz stereo | %s → %s\n", background, cue, duration, audio.Rate, cfg.Format, cfg.Path)
	if cfg.Start != nil {
		fmt.Fprintf(stdout, "excerpt: %s–%s of %s | source frames [%d, %d) | %d frames\n",
			excerpt.Start, excerpt.End, s.Duration, excerpt.FirstFrame, excerpt.FirstFrame+excerpt.Frames, excerpt.Frames)
	}
	for _, sec := range s.CueSections {
		if sec.Start < start+duration && sec.Start+sec.Duration > start {
			fmt.Fprintf(stdout, "  cue %s: %s–%s | %g%%, every %d ms | fade in %s, out %s\n",
				sec.Cue, sec.Start, sec.Start+sec.Duration, sec.CueVolumePercent, sec.CueIntervalMS, sec.FadeIn, sec.FadeOut)
		}
	}
	if cfg.DryRun {
		return 0
	}
	var pcm codec.StereoPCM
	var err error
	if cfg.Start != nil {
		pcm, err = excerpt.PCM()
	} else {
		pcm, err = s.PCMFor(duration)
	}
	if err == nil {
		err = writeMix(cfg, pcm)
	}
	if err != nil {
		fmt.Fprintln(stderr, "entrain:", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s (%s)\n", cfg.Path, humanBytes(fileSize(cfg.Path)))
	return 0
}

func writeMix(cfg mixOutput, pcm codec.StereoPCM) error {
	f, err := os.CreateTemp(filepath.Dir(cfg.Path), ".entrain-mix-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = codec.WritePCM(f, cfg.Format, audio.Rate, cfg.Bits, cfg.Bitrate, pcm)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
		return closeErr
	}
	return os.Rename(f.Name(), cfg.Path)
}
