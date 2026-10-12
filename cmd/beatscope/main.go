// Command beatscope reports the binaural beats present in an audio file
// (WAV or FLAC): per pair, the carrier, beat rate, level, and a
// confidence score. It verifies what a renderer — entrain's or anyone
// else's — actually put into the file.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/desireco/entrain/beatscope"
	"github.com/mewkiz/flac"
)

var version = "0.0.0-dev"

const usage = `beatscope — detect binaural beats in audio (EntrainSpace)

Usage:

  beatscope <file.wav|file.flac> [--json] [--min-confidence 0.0]

Reads 16/24-bit PCM WAV (RIFF) and FLAC. MP3 and OGG inputs are not
decoded yet — convert to WAV or FLAC first.

Output (human):

  file.wav: 44100 Hz, 30.0s, stereo
    carrier   beat   level  confidence
    180.00   40.00   1.000  0.96
    320.00   24.00   0.833  0.95

--json emits the same data as a machine-readable array.`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	jsonOut, minConf := false, 0.0
	var files []string
	for _, a := range args {
		switch {
		case a == "--json" || a == "-j":
			jsonOut = true
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, usage)
			return 0
		case a == "--version":
			fmt.Fprintln(stdout, "beatscope", version)
			return 0
		case strings.HasPrefix(a, "--min-confidence="):
			if _, err := fmt.Sscanf(strings.TrimPrefix(a, "--min-confidence="), "%g", &minConf); err != nil {
				fmt.Fprintf(stderr, "beatscope: bad --min-confidence value\n")
				return 2
			}
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "beatscope: unknown flag %q\n\n%s\n", a, usage)
			return 2
		default:
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	status := 0
	for _, path := range files {
		if code := analyzeOne(path, jsonOut, minConf, stdout, stderr); code != 0 {
			status = code
		}
	}
	return status
}

func analyzeOne(path string, jsonOut bool, minConf float64, stdout, stderr io.Writer) int {
	sr, x, err := decode(path)
	if err != nil {
		fmt.Fprintf(stderr, "beatscope: %v\n", err)
		return 1
	}
	beats := beatscope.Analyze(x, sr)
	filtered := beats[:0]
	for _, b := range beats {
		if b.Confidence >= minConf {
			filtered = append(filtered, b)
		}
	}

	if jsonOut {
		type out struct {
			File       string           `json:"file"`
			SampleRate int              `json:"sample_rate"`
			Seconds    float64          `json:"seconds"`
			Beats      []beatscope.Beat `json:"beats"`
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(out{
			File:       path,
			SampleRate: sr,
			Seconds:    float64(len(x) / 2 / sr),
			Beats:      filtered,
		})
		return 0
	}

	fmt.Fprintf(stdout, "%s: %d Hz, %.1fs, stereo — %d beat(s) detected\n",
		path, sr, float64(len(x)/2/sr), len(filtered))
	if len(filtered) == 0 {
		if len(beats) > 0 {
			fmt.Fprintf(stdout, "  (all below --min-confidence=%g)\n", minConf)
		} else {
			fmt.Fprintf(stdout, "  no binaural beats\n")
		}
		return 0
	}
	fmt.Fprintln(stdout, "  carrier   beat   level  confidence")
	for _, b := range filtered {
		fmt.Fprintf(stdout, "  %7.2f %6.2f  %6.3f  %.2f\n", b.Carrier, b.Beat, b.Level, b.Confidence)
	}
	return 0
}

// decode reads a WAV or FLAC file into interleaved float samples.
func decode(path string) (sr int, x []float64, err error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".wav":
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return 0, nil, rerr
		}
		return decodeWav(b)
	case ".flac":
		return decodeFlac(path)
	default:
		return 0, nil, fmt.Errorf("%s: unsupported input (want .wav or .flac; convert MP3/OGG first)", path)
	}
}

// decodeWav parses RIFF/WAVE PCM (16 or 24-bit).
func decodeWav(b []byte) (sr int, x []float64, err error) {
	if len(b) < 44 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return 0, nil, fmt.Errorf("not a RIFF/WAVE file")
	}
	var data []byte
	channels, sampleRate, bits := 0, 0, 0
	pos := 12
	for pos+8 <= len(b) {
		id := string(b[pos : pos+4])
		size := int(b[pos+4]) | int(b[pos+5])<<8 | int(b[pos+6])<<16 | int(b[pos+7])<<24
		body := b[pos+8 : min(pos+8+size, len(b))]
		switch id {
		case "fmt ":
			if len(body) < 16 {
				return 0, nil, fmt.Errorf("short fmt chunk")
			}
			if int(body[0])|int(body[1])<<8 != 1 {
				return 0, nil, fmt.Errorf("unsupported WAV format (want PCM)")
			}
			channels = int(body[2]) | int(body[3])<<8
			sampleRate = int(body[4]) | int(body[5])<<8 | int(body[6])<<16 | int(body[7])<<24
			bits = int(body[14]) | int(body[15])<<8
		case "data":
			data = body
		}
		pos += 8 + size + size%2
	}
	if channels != 2 || data == nil || (bits != 16 && bits != 24) {
		return 0, nil, fmt.Errorf("want stereo 16/24-bit PCM WAV (got %d ch, %d bits)", channels, bits)
	}

	switch bits {
	case 16:
		x = make([]float64, len(data)/2)
		for i := range x {
			x[i] = float64(int16(int(data[2*i])|int(data[2*i+1])<<8)) / 32768
		}
	case 24:
		x = make([]float64, len(data)/3)
		for i := range x {
			v := int32(data[3*i]) | int32(data[3*i+1])<<8 | int32(data[3*i+2])<<16
			if v&0x800000 != 0 {
				v |= ^int32(0xFFFFFF) // sign-extend
			}
			x[i] = float64(v) / 8388608
		}
	}
	return sampleRate, x, nil
}

// decodeFlac decodes via mewkiz/flac (the Phase 3 dependency).
func decodeFlac(path string) (sr int, x []float64, err error) {
	stream, err := flac.ParseFile(path)
	if err != nil {
		return 0, nil, err
	}
	defer stream.Close()
	sr = int(stream.Info.SampleRate)
	bps := int(stream.Info.BitsPerSample)
	if stream.Info.NChannels != 2 || (bps != 16 && bps != 24) {
		return 0, nil, fmt.Errorf("want stereo 16/24-bit FLAC (got %d ch, %d bits)", stream.Info.NChannels, bps)
	}
	full := int64(1)<<bps - 1
	for {
		fr, ferr := stream.ParseNext()
		if ferr != nil {
			if ferr.Error() == "EOF" {
				break
			}
			return 0, nil, ferr
		}
		for i := range fr.Subframes[0].Samples {
			x = append(x,
				float64(fr.Subframes[0].Samples[i])/float64(full),
				float64(fr.Subframes[1].Samples[i])/float64(full))
		}
	}
	return sr, x, nil
}
