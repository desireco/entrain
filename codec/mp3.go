package codec

import (
	"fmt"
	"io"

	"github.com/braheezy/shine-mp3/pkg/mp3"
)

// mpeg1Layer3Bitrates maps the MPEG-1 Layer III bitrate-table index to
// kbps (ISO/IEC 11172-3 §2.4.2.3); shine-mp3's constructor hardcodes
// 128 kbps and offers no setter, so setBitrate redoes its derived-field
// arithmetic after changing the bitrate.
var mpeg1Layer3Bitrates = [16]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}

// WriteMp3 encodes interleaved stereo float64 samples (nominal ±1) as
// MP3 at the given bitrate (kbps, MPEG-1 Layer III table: 32…320) via
// the pure-Go Shine encoder. Sample rates are the MPEG-1 set
// (32000/44100/48000). Samples pass the shared clip → TPDF-dither →
// quantize stage; output is deterministic for identical input.
func WriteMp3(w io.Writer, sampleRate, bitrate int, x []float64) error {
	if len(x)%2 != 0 {
		return fmt.Errorf("codec: interleaved stereo buffer has odd length %d", len(x))
	}
	switch sampleRate {
	case 32000, 44100, 48000: // MPEG-1 only: the bitrate table below is MPEG-1's
	default:
		return fmt.Errorf("codec: mp3: sample rate %d not supported (32000/44100/48000)", sampleRate)
	}
	if _, err := mp3.CheckConfig(sampleRate, bitrate); err != nil {
		return fmt.Errorf("codec: mp3: sample rate %d / bitrate %d: %w", sampleRate, bitrate, err)
	}
	q := newQuantizer()
	s16 := make([]int16, len(x))
	for i, s := range x {
		s16[i] = q.int16(s)
	}

	enc := mp3.NewEncoder(sampleRate, 2)
	if err := setBitrate(enc, sampleRate, bitrate); err != nil {
		return err
	}

	// Feed the encoder one frame of audio at a time: 1152 samples per
	// channel, interleaved. (Encoder.Write's own chunking splits stereo
	// data incorrectly, dropping every other frame's worth of samples.)
	const samplesPerChannel = 2 * mp3.GRANULE_SIZE // MPEG-1: 2 granules × 576
	for off := 0; off < len(s16); off += 2 * samplesPerChannel {
		end := min(off+2*samplesPerChannel, len(s16))
		data, n := enc.EncodeBufferInterleaved(s16[off:end])
		if _, err := w.Write(data[:n]); err != nil {
			return fmt.Errorf("codec: mp3 write: %w", err)
		}
	}
	return nil
}

// setBitrate reconfigures the encoder's bitrate and the frame-size fields
// derived from it, mirroring shine-mp3's constructor arithmetic.
func setBitrate(enc *mp3.Encoder, sampleRate, bitrate int) error {
	idx := -1
	for i, b := range mpeg1Layer3Bitrates {
		if b == bitrate {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("codec: mp3: unsupported bitrate %d (want MPEG-1 Layer III: 32–320)", bitrate)
	}
	enc.Mpeg.Bitrate = int64(bitrate)
	enc.Mpeg.BitrateIndex = int64(idx)

	avgSlotsPerFrame := (float64(enc.Mpeg.GranulesPerFrame) * mp3.GRANULE_SIZE / float64(sampleRate)) *
		(float64(bitrate) * 1000 / float64(enc.Mpeg.BitsPerSlot))
	enc.Mpeg.WholeSlotsPerFrame = int64(avgSlotsPerFrame)
	enc.Mpeg.FracSlotsPerFrame = avgSlotsPerFrame - float64(enc.Mpeg.WholeSlotsPerFrame)
	enc.Mpeg.SlotLag = -enc.Mpeg.FracSlotsPerFrame
	if enc.Mpeg.FracSlotsPerFrame == 0 {
		enc.Mpeg.Padding = 0
	}
	return nil
}
