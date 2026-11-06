package codec

import (
	"fmt"
	"io"

	"github.com/go-audio/audio"
	wav "github.com/go-audio/wav"
)

// WriteWav encodes interleaved stereo float64 samples (nominal ±1) to a
// PCM WAV stream at the given bit depth (16 or 24), written to w
// (seekable: RIFF sizes are patched on close). Each sample passes the
// shared clip → TPDF-dither → quantize stage; output is deterministic
// for identical input.
func WriteWav(w io.WriteSeeker, sampleRate, bits int, x []float64) error {
	if len(x)%2 != 0 {
		return fmt.Errorf("codec: interleaved stereo buffer has odd length %d", len(x))
	}
	return writeWav(w, sampleRate, bits, fromSamples(x))
}

func writeWav(w io.WriteSeeker, sampleRate, bits int, pcm StereoPCM) error {
	if bits != 16 && bits != 24 {
		return fmt.Errorf("codec: wav: unsupported bit depth %d (16 or 24)", bits)
	}
	q := newQuantizer(bits)
	enc := wav.NewEncoder(w, sampleRate, bits, 2, 1)
	const blockFrames = 4096
	data := make([]int, 2*blockFrames)
	remaining := pcm.Frames
	for {
		n := int(min(remaining, blockFrames))
		for i := range n {
			l, r := pcm.Next()
			data[2*i], data[2*i+1] = int(q.sample(l)), int(q.sample(r))
		}
		if err := enc.Write(&audio.IntBuffer{
			Data: data[:2*n], Format: &audio.Format{SampleRate: sampleRate, NumChannels: 2},
		}); err != nil {
			return fmt.Errorf("codec: wav write: %w", err)
		}
		remaining -= int64(n)
		if remaining == 0 {
			break
		}
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("codec: wav close: %w", err)
	}
	return nil
}

// WriteWav16 is WriteWav at the default 16-bit depth.
func WriteWav16(w io.WriteSeeker, sampleRate int, x []float64) error {
	return WriteWav(w, sampleRate, 16, x)
}
