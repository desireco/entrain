// Command noisemaker renders standalone noise audio — the sophisticated
// side of the noise split (#13): white/pink/brown plus arbitrary 1/f^α
// slopes, written as WAV/FLAC/MP3 for standalone listening or as beds
// under entrain sessions via its mix path.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/desireco/entrain/codec"
	"github.com/desireco/entrain/noise"
)

var version = "0.0.0-dev"

const usage = `noisemaker — standalone noise generator (EntrainSpace)

Usage:

  noisemaker <color> <duration> [flags]

Colors:

  white, pink, brown         basic palette (unit conventions shared with entrain)
  alpha --alpha SLOPE         power spectral density ∝ 1/f^SLOPE (0=white … 2=brown)

Duration: Go duration (90s, 30m, 2h) or h:mm[:ss].

Flags:

  -o file       output path (default: <color>-<duration>.<format>)
  -f format     wav (default), flac, or mp3
  -b bits       wav/flac bit depth: 16 (default) or 24
  -q kbps       MP3 bitrate (default 320)
  -r rate       sample rate: 44100 or 48000 (default 44100)
  --level RMS   output RMS (default 0.155 — entrain's bed convention)
  --seed N      deterministic seed (default 1)

Examples:

  noisemaker brown 30m -o bed.flac
  noisemaker alpha 1h --alpha 1.4 --level 0.2 -f mp3`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("noisemaker", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outPath := fs.String("o", "", "output path")
	format := fs.String("f", "wav", "output format")
	bitDepth := fs.Int("b", 16, "bit depth for wav/flac")
	bitrate := fs.Int("q", 320, "MP3 bitrate in kbps")
	rate := fs.Int("r", 44100, "sample rate")
	level := fs.Float64("level", 0.155, "output RMS")
	seed := fs.Uint64("seed", 1, "deterministic seed")
	alpha := fs.Float64("alpha", 1, "1/f^alpha slope (color alpha)")

	var pos []string
	var flags []string
	withValue := map[string]bool{"-o": true, "-f": true, "-b": true, "-q": true, "-r": true, "-level": true, "-seed": true, "-alpha": true}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-") // --flag == -flag for lookup
			if withValue["-"+name] && !strings.Contains(a, "=") && i+1 < len(args) {
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
	if fs.NArg() != 2 {
		if fs.NArg() == 0 && len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
			fmt.Fprintln(stdout, usage)
			return 0
		}
		fmt.Fprintln(stderr, usage)
		return 2
	}
	switch *format {
	case "wav", "flac", "mp3":
	default:
		fmt.Fprintf(stderr, "noisemaker: unknown format %q (wav, flac, or mp3)\n", *format)
		return 2
	}
	if *rate != 44100 && *rate != 48000 {
		fmt.Fprintf(stderr, "noisemaker: unsupported sample rate %d (44100 or 48000)\n", *rate)
		return 2
	}
	if *bitDepth != 16 && *bitDepth != 24 {
		fmt.Fprintf(stderr, "noisemaker: unsupported bit depth %d (16 or 24)\n", *bitDepth)
		return 2
	}

	color := fs.Arg(0)
	dur, err := parseDuration(fs.Arg(1))
	if err != nil {
		fmt.Fprintf(stderr, "noisemaker: %v\n", err)
		return 2
	}
	frames := int(dur * time.Duration(*rate) / time.Second)

	var buf []float64
	switch color {
	case "alpha":
		buf = noise.Alpha(*alpha, float64(*rate), *seed, frames)
	default:
		buf, err = noise.Basic(color, *rate, *seed, frames)
	}
	if err != nil {
		fmt.Fprintf(stderr, "noisemaker: %v\n", err)
		return 1
	}
	for i := range buf {
		buf[i] *= *level
	}

	if *outPath == "" {
		*outPath = fmt.Sprintf("%s-%s.%s", color, fs.Arg(1), *format)
	}
	if err := write(*outPath, *format, *rate, *bitDepth, *bitrate, buf); err != nil {
		fmt.Fprintf(stderr, "noisemaker: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s (%s, %s noise at RMS %.3f, %d Hz, %s)\n",
		*outPath, seqDur(dur), color, *level, *rate, humanBytes(size(*outPath)))
	return 0
}

func write(path, format string, rate, bits, bitrate int, buf []float64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	switch format {
	case "wav":
		err = codec.WriteWav(f, rate, bits, buf)
		if err == nil {
			err = f.Close()
		} else {
			f.Close()
		}
	case "flac":
		err = codec.WriteFlac(f, rate, bits, buf)
	case "mp3":
		err = codec.WriteMp3(f, rate, bitrate, buf)
		if err == nil {
			err = f.Close()
		} else {
			f.Close()
		}
	}
	return err
}

// parseDuration accepts Go durations (90s, 30m, 2h), h:mm[:ss], and
// plain seconds.
func parseDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return d, nil
	}
	if strings.Contains(s, ":") {
		parts := strings.Split(s, ":")
		if len(parts) == 2 || len(parts) == 3 {
			var h, m, sec int
			var err error
			if h, err = strconv.Atoi(parts[0]); err != nil {
				return 0, fmt.Errorf("bad duration %q", s)
			}
			if m, err = strconv.Atoi(parts[1]); err != nil {
				return 0, fmt.Errorf("bad duration %q", s)
			}
			if len(parts) == 3 {
				if sec, err = strconv.Atoi(parts[2]); err != nil {
					return 0, fmt.Errorf("bad duration %q", s)
				}
			}
			return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second, nil
		}
	}
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return time.Duration(n) * time.Second, nil
	}
	return 0, fmt.Errorf("bad duration %q (90s, 30m, 2h, or h:mm[:ss])", s)
}

func seqDur(d time.Duration) string {
	total := int64(d.Seconds())
	return fmt.Sprintf("%d:%02d:%02d", total/3600, total%3600/60, total%60)
}

func size(path string) int64 {
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
