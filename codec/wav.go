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
	if bits != 16 && bits != 24 {
		return fmt.Errorf("codec: wav: unsupported bit depth %d (16 or 24)", bits)
	}
	q := newQuantizer(bits)
	data := make([]int, len(x))
	for i, s := range x {
		data[i] = int(q.sample(s))
	}
	enc := wav.NewEncoder(w, sampleRate, bits, 2, 1) // 1 = PCM
	if err := enc.Write(&audio.IntBuffer{
		Data: data,
		Format: &audio.Format{
			SampleRate:  sampleRate,
			NumChannels: 2,
		},
	}); err != nil {
		return fmt.Errorf("codec: wav write: %w", err)
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
