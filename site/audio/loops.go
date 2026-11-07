// Package audio holds the mastered, seamless loops shared by Studio and the CLI.
package audio

import (
	"embed"
	"fmt"
	"io"

	"github.com/desireco/entrain/internal/dsp"
	"github.com/mewkiz/flac"
)

//go:embed pink.flac white.flac rain.flac cue-*.wav
var assets embed.FS

const Rate = 44100

func Background(name string) ([]float64, error) {
	f, err := assets.Open(name + ".flac")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	stream, err := flac.New(f)
	if err != nil {
		return nil, err
	}
	info := stream.Info
	if info.SampleRate != Rate || info.NChannels != 2 || info.BitsPerSample != 16 || info.NSamples != 60*Rate {
		return nil, fmt.Errorf("invalid stereo loop %s", name)
	}
	pcm := make([]float64, 0, 2*info.NSamples)
	for {
		frame, err := stream.ParseNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		l, r := frame.Subframes[0].Samples, frame.Subframes[1].Samples
		for i := range l {
			pcm = append(pcm, float64(l[i])/32768, float64(r[i])/32768)
		}
	}
	if len(pcm) != 2*60*Rate {
		return nil, fmt.Errorf("incomplete background loop %s", name)
	}
	return pcm, nil
}

func Cue(name string) ([]float64, error) {
	b, err := assets.ReadFile("cue-" + name + ".wav")
	if err != nil {
		return nil, err
	}
	sr, pcm, err := dsp.ReadWav16(b)
	if err != nil {
		return nil, err
	}
	if sr != Rate || len(pcm) != 2*10*Rate {
		return nil, fmt.Errorf("invalid bilateral loop %s", name)
	}
	return pcm, nil
}
