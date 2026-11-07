package codec

import (
	"fmt"
	"io"
	"math"
)

// StereoPCM supplies Frames pairs of samples in playback order. Encoders
// consume bounded blocks, so output duration does not determine memory use.
type StereoPCM struct {
	Frames     int64
	StartFrame int64 // original frame position, for dither on exact excerpts
	Next       func() (float64, float64)
}

func fromSamples(x []float64) StereoPCM {
	i := 0
	return StereoPCM{Frames: int64(len(x) / 2), Next: func() (float64, float64) {
		l, r := x[i], x[i+1]
		i += 2
		return l, r
	}}
}

// WritePCM encodes a stereo stream using the same encoders as sequence renders.
// The FLAC encoder closes w when it implements io.Closer. Metadata applies
// where the container supports tags (MP3's ID3v2); other formats ignore it.
func WritePCM(w io.WriteSeeker, format string, rate, bits, bitrate int, pcm StereoPCM, meta Metadata) error {
	if pcm.Frames < 0 || pcm.StartFrame < 0 || pcm.Next == nil {
		return fmt.Errorf("invalid stereo PCM source")
	}
	switch format {
	case "wav":
		// RIFF's chunk sizes are unsigned 32-bit fields.
		if (bits == 16 || bits == 24) && pcm.Frames > int64(math.MaxUint32-64)/int64(2*bits/8) {
			return fmt.Errorf("WAV exceeds the RIFF size limit; choose FLAC or MP3")
		}
		return writeWav(w, rate, bits, pcm)
	case "flac":
		if pcm.Frames >= 1<<36 {
			return fmt.Errorf("FLAC exceeds the 36-bit sample count limit")
		}
		return writeFlac(w, rate, bits, pcm)
	case "mp3":
		return writeMp3(w, rate, bitrate, pcm, meta)
	default:
		return fmt.Errorf("unknown output format %q", format)
	}
}
