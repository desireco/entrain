package codec

import (
	"fmt"
	"io"

	"github.com/mewkiz/flac"
	"github.com/mewkiz/flac/frame"
	"github.com/mewkiz/flac/meta"
)

// WriteFlac16 encodes interleaved stereo float64 samples (nominal ±1) as
// 16-bit FLAC (lossless). Samples pass the same clip → TPDF-dither →
// quantize stage as WriteWav16, so the two formats carry bit-identical
// audio; output is deterministic for identical input.
//
// Note: the encoder patches the StreamInfo block (MD5, sample count) on
// close and closes w if it implements io.Closer — callers should treat w
// as consumed afterwards.
func WriteFlac16(w io.WriteSeeker, sampleRate int, x []float64) error {
	if len(x)%2 != 0 {
		return fmt.Errorf("codec: interleaved stereo buffer has odd length %d", len(x))
	}
	q := newQuantizer()
	enc, err := flac.NewEncoder(w, &meta.StreamInfo{
		SampleRate:    uint32(sampleRate),
		NChannels:     2,
		BitsPerSample: 16,
	})
	if err != nil {
		return fmt.Errorf("codec: flac init: %w", err)
	}

	const blockSize = 4096
	n := len(x) / 2
	for off := 0; off < n; off += blockSize {
		m := min(blockSize, n-off)
		l := make([]int32, m)
		r := make([]int32, m)
		for i := range m {
			l[i] = int32(q.int16(x[2*(off+i)]))
			r[i] = int32(q.int16(x[2*(off+i)+1]))
		}
		f := &frame.Frame{
			Header: frame.Header{
				HasFixedBlockSize: true,
				BlockSize:         uint16(m),
				SampleRate:        uint32(sampleRate),
				Channels:          frame.ChannelsLR,
				BitsPerSample:     16,
			},
			Subframes: []*frame.Subframe{
				{SubHeader: frame.SubHeader{Pred: frame.PredVerbatim}, NSamples: m, Samples: l},
				{SubHeader: frame.SubHeader{Pred: frame.PredVerbatim}, NSamples: m, Samples: r},
			},
		}
		if err := enc.WriteFrame(f); err != nil {
			return fmt.Errorf("codec: flac frame: %w", err)
		}
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("codec: flac close: %w", err)
	}
	return nil
}
