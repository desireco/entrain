// Command entrain renders binaural-beat and noise sessions to audio files.
//
// v0 reads SBaGen .sbg sequence files (clean-room, read-only). A bare
// session name resolves to <name>.sbg in the working directory, so
// `entrain focus-10` renders focus-10.sbg. TOML "trains" remain the
// planned native format for post-v0.1.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/desireco/entrain/codec"
	"github.com/desireco/entrain/seq"
)

var version = "0.0.0-dev"

const usage = `entrain — binaural beat + noise session renderer (EntrainSpace)

Usage:

  entrain <session> [flags]        render <name>.sbg (shorthand)
  entrain render <session> [flags] render a sequence file
  entrain version

A session is a .sbg file path or a bare name resolved as <name>.sbg.

Flags:

  --dry-run       print the render plan (timeline + lint warnings) and exit
  -o file         output path (default: <session>.wav)
  -L h:mm[:ss]    limit the output length (overrides a -L in the file)
  -r rate         sample rate, 44100 or 48000 (default 44100)

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
		fmt.Fprintln(stdout, "entrain", version)
		return 0
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, usage)
		return 0
	case "render":
		return cmdRender(args[1:], stdout, stderr)
	default:
		return cmdRender(args, stdout, stderr)
	}
}

func cmdRender(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "print the render plan and exit")
	outPath := fs.String("o", "", "output path (default: <session>.wav)")
	length := fs.String("L", "", "limit output length (h:mm[:ss])")
	rate := fs.Int("r", 44100, "sample rate")
	// Go's flag package stops at the first positional; reorder so flags
	// may appear after the session name (`entrain focus-10 --dry-run`),
	// keeping each value-taking flag glued to its argument.
	var flags, pos []string
	withValue := map[string]bool{"-o": true, "-L": true, "-r": true}
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
	if *rate != 44100 && *rate != 48000 {
		fmt.Fprintf(stderr, "entrain: unsupported sample rate %d (44100 or 48000)\n", *rate)
		return 2
	}

	path := fs.Arg(0)
	if !strings.HasSuffix(path, ".sbg") {
		path += ".sbg"
	}
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "entrain: %v\n", err)
		return 1
	}
	s, err := seq.Parse(string(src))
	if err != nil {
		fmt.Fprintf(stderr, "entrain: %s: %v\n", path, err)
		return 1
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
	if *outPath == "" {
		*outPath = strings.TrimSuffix(path, ".sbg") + ".wav"
	}
	if err := writeWav(*outPath, *rate, buf); err != nil {
		fmt.Fprintf(stderr, "entrain: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s (%s, %d Hz, %s)\n",
		*outPath, seq.FormatDur(plan.End-plan.Start), *rate,
		humanBytes(int64(len(buf))*2+44))
	return 0
}

func writeWav(path string, rate int, buf []float64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := codec.WriteWav16(f, rate, buf); err != nil {
		f.Close()
		return err
	}
	return f.Close()
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
