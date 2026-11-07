// Command entrain renders binaural-beat and noise sessions to audio files.
//
// Native .entrain and .toml sessions render Studio soundscapes. Legacy
// .sbg files use the clean-room SBaGen reader; a bare name still resolves
// to <name>.sbg in the working directory.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/desireco/entrain/codec"
	"github.com/desireco/entrain/internal/buildversion"
	"github.com/desireco/entrain/seq"
)

var version string

const usage = `entrain — binaural beat + noise session renderer (EntrainSpace)

Usage:

  entrain render file.entrain      render a native Studio soundscape (TOML)
  entrain mix pink|white|rain|wind|surf|rain-loop [--cue bell|tone|noise] [--duration 6m] [-o file]
  entrain mix rain --save-session file.entrain   save editable source without rendering
  entrain <session> [flags]        render <name>.sbg (shorthand)
  entrain render <session> [flags] render a sequence file
  entrain version

Native sessions use .entrain or .toml. Legacy .sbg paths and bare names resolve as before.

Flags:

  -f format    output format: wav (default), flac, or mp3
  -b bits      bit depth for wav/flac: 16 (default) or 24 (lossless masters)
  -q kbps      MP3 bitrate, MPEG-1 Layer III 32–320 (default 320)
  -o file      output path (default: <session>.<format>)
  -L length    limit output length (native: 30s or h:mm[:ss]; legacy: h:mm[:ss])
  --start time native exact excerpt start (e.g. 4m20s); -L sets its length
  -r rate      sample rate, 44100 or 48000 (default 44100)
  --dry-run    print the render plan (timeline + lint warnings) and exit

Roadmap: docs/migration-plan.md. Reads .sbg per SBaGen's published format;
never writes it.`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point; it returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(stdout, "entrain", buildversion.Resolve(version))
		return 0
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, usage)
		return 0
	case "render":
		return cmdRender(args[1:], stdout, stderr)
	case "mix":
		// Preserve the original shorthand for an existing session named mix.sbg.
		if len(args) == 1 || strings.HasPrefix(args[1], "-") && args[1] != "--help" && args[1] != "-h" {
			if _, err := os.Stat("mix.sbg"); err == nil {
				return cmdRender(args, stdout, stderr)
			}
		}
		return cmdMix(args[1:], stdout, stderr)
	default:
		return cmdRender(args, stdout, stderr)
	}
}

func cmdRender(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "print the render plan and exit")
	format := fs.String("f", "wav", "output format: wav, flac, or mp3")
	bitDepth := fs.Int("b", 16, "bit depth for wav/flac output")
	bitrate := fs.Int("q", 320, "MP3 bitrate in kbps (32-320)")
	outPath := fs.String("o", "", "output path (default: <session>.<format>)")
	length := fs.String("L", "", "output length (native: 30s or h:mm[:ss]; legacy: h:mm[:ss])")
	var start *time.Duration
	fs.Func("start", "source position of exact native excerpt (e.g. 4m20s)", func(text string) error {
		at, err := parseNativeTime(text)
		if err != nil || at < 0 {
			return fmt.Errorf("invalid nonnegative excerpt start %q", text)
		}
		start = &at
		return nil
	})
	rate := fs.Int("r", 44100, "sample rate")
	// Go's flag package stops at the first positional; reorder so flags
	// may appear after the session name (`entrain focus-10 --dry-run`),
	// keeping each value-taking flag glued to its argument.
	var flags, pos []string
	withValue := map[string]bool{"-o": true, "-L": true, "-r": true, "-f": true, "-q": true, "-b": true, "--start": true, "-start": true}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			if withValue[a] && !strings.Contains(a, "=") && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		pos = append(pos, a)
	}
	if err := fs.Parse(append(flags, pos...)); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	switch *format {
	case "wav", "flac", "mp3":
	default:
		fmt.Fprintf(stderr, "entrain: unknown format %q (wav, flac, or mp3)\n", *format)
		return 2
	}
	if *bitDepth != 16 && *bitDepth != 24 {
		fmt.Fprintf(stderr, "entrain: unsupported bit depth %d (16 or 24)\n", *bitDepth)
		return 2
	}
	if *format == "mp3" && *bitDepth != 16 {
		fmt.Fprintf(stderr, "warning: -b %d ignored: mp3 is lossy with a fixed internal depth\n", *bitDepth)
	}
	if *rate != 44100 && *rate != 48000 {
		fmt.Fprintf(stderr, "entrain: unsupported sample rate %d (44100 or 48000)\n", *rate)
		return 2
	}

	path := fs.Arg(0)
	if isNative(path) {
		output := mixOutput{Path: *outPath, DryRun: *dryRun, Start: start}
		emptyLimit := false
		fs.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "L":
				emptyLimit = *length == ""
			case "f":
				output.Format = *format
			case "b":
				output.Bits = *bitDepth
			case "q":
				output.Bitrate = *bitrate
				output.BitrateSet = true
			}
		})
		if emptyLimit {
			fmt.Fprintln(stderr, "entrain: -L requires a positive length")
			return 2
		}
		return cmdTrain(path, *length, *rate, output, stdout, stderr)
	}
	if start != nil {
		fmt.Fprintln(stderr, "entrain: --start requires a native .entrain or .toml session")
		return 2
	}
	if !strings.HasSuffix(path, ".sbg") {
		path += ".sbg"
	}
	s, err := seq.ParseFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "entrain: %v\n", err)
		return 1
	}

	if *outPath == "" {
		*outPath = strings.TrimSuffix(path, ".sbg") + "." + *format
	}
	if err := checkOutput(path, *outPath); err != nil {
		fmt.Fprintf(stderr, "entrain: %v\n", err)
		return 2
	}
	if *format == "mp3" {
		if err := codec.ValidateMp3(*rate, *bitrate); err != nil {
			fmt.Fprintf(stderr, "entrain: %v\n", err)
			return 1
		}
	}

	var limit time.Duration
	if *length != "" {
		limit, err = seq.ParseLength(*length)
	} else if s.Opts.Length != "" {
		limit, err = seq.ParseLength(s.Opts.Length)
	}
	if err != nil {
		fmt.Fprintf(stderr, "entrain: %v\n", err)
		return 1
	}

	plan, err := seq.BuildPlan(s, seq.PlanOpts{LengthLimit: limit})
	if err != nil {
		fmt.Fprintf(stderr, "entrain: %s: %v\n", path, err)
		return 1
	}
	plan.WritePlan(stdout)
	for _, warn := range plan.Warnings {
		if warn.Line > 0 {
			fmt.Fprintf(stderr, "warning: %s:%d: %s\n", path, warn.Line, warn.Msg)
		} else {
			fmt.Fprintf(stderr, "warning: %s: %s\n", path, warn.Msg)
		}
	}
	if *dryRun {
		return 0
	}
	if plan.OpenEnded {
		fmt.Fprintf(stderr, "entrain: %s: open-ended sequence; pass -L to render\n", path)
		return 1
	}

	buf := plan.Render(*rate)
	if err := encode(path, *outPath, *format, *rate, *bitDepth, *bitrate, buf); err != nil {
		fmt.Fprintf(stderr, "entrain: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s (%s, %d Hz, %s, %s)\n",
		*outPath, seq.FormatDur(plan.End-plan.Start), *rate, *format,
		humanBytes(fileSize(*outPath)))
	return 0
}

var errSourceOutput = errors.New("choose an output separate from the source session")

func checkOutput(source, output string) error {
	dst, err := os.Stat(output)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	src, err := os.Stat(source)
	if err != nil {
		return err
	}
	if os.SameFile(src, dst) {
		return errSourceOutput
	}
	return nil
}

// encode writes the rendered buffer to path in the given format.
func encode(source, path, format string, rate, bitDepth, bitrate int, buf []float64) error {
	// Open without truncating. Recheck the chosen file itself before writing
	// in case a symlink or hard link changed while a long plan rendered.
	input, err := os.Stat(source)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	dst, err := f.Stat()
	if err == nil && os.SameFile(input, dst) {
		err = errSourceOutput
	}
	if err == nil && dst.Mode().IsRegular() {
		err = f.Truncate(0)
	}
	if err != nil {
		f.Close()
		return err
	}
	switch format {
	case "wav":
		err = codec.WriteWav(f, rate, bitDepth, buf)
		if err == nil {
			err = f.Close()
		} else {
			f.Close()
		}
	case "flac":
		// WriteFlac's encoder closes the file itself (StreamInfo patch).
		err = codec.WriteFlac(f, rate, bitDepth, buf)
	case "mp3":
		err = codec.WriteMp3(f, rate, bitrate, buf)
		if err == nil {
			err = f.Close()
		} else {
			f.Close()
		}
	default:
		f.Close()
		return fmt.Errorf("unknown format %q", format)
	}
	return err
}

func fileSize(path string) int64 {
	if st, err := os.Stat(path); err == nil {
		return st.Size()
	}
	return 0
}

func humanBytes(n int64) string {
	const KiB, MiB = 1 << 10, 1 << 20
	switch {
	case n >= MiB:
		return fmt.Sprintf("%.1f MiB", float64(n)/MiB)
	case n >= KiB:
		return fmt.Sprintf("%.1f KiB", float64(n)/KiB)
	}
	return fmt.Sprintf("%d B", n)
}
