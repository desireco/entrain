package codec

import (
	"fmt"
	"io"

	"github.com/go-audio/audio"
	wav "github.com/go-audio/wav"
)

// WriteWav16 encodes interleaved stereo float64 samples (nominal ±1) as a
// 16-bit PCM WAV stream written to w (seekable: the RIFF sizes are patched
// on close). Each sample passes the shared clip → TPDF-dither → quantize
// stage; output is deterministic for identical input.
func WriteWav16(w io.WriteSeeker, sampleRate int, x []float64) error {
	if len(x)%2 != 0 {
		return fmt.Errorf("codec: interleaved stereo buffer has odd length %d", len(x))
	}
	q := newQuantizer()
	data := make([]int, len(x))
	for i, s := range x {
		data[i] = int(q.int16(s))
	}
	enc := wav.NewEncoder(w, sampleRate, 16, 2, 1) // 1 = PCM
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
