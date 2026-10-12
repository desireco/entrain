// Package dsp holds small signal-analysis helpers: exact-bin Goertzel DFT
// probes, level/correlation statistics, and a minimal PCM WAV reader. The
// synth and codec tests use these to verify rendered audio against math
// expectations; the planned Beatscope analyzer (issue #4) will build on the
// same primitives.
package dsp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Goertzel returns the magnitude of the DFT of x at frequency f Hz, where x
// is sampled at sr Hz. For an N-sample pure sine of amplitude A at exactly
// bin frequency f = k·sr/N, the result is A·N/2. Frequencies between bins
// leak energy; callers wanting exact amplitudes pick f and len(x) so that
// f·len(x)/sr is an integer.
func Goertzel(x []float64, sr, f float64) float64 {
	n := float64(len(x))
	k := f * n / sr
	w := 2 * math.Pi * k / n
	coeff := 2 * math.Cos(w)
	var s1, s2 float64
	for _, v := range x {
		s0 := v + coeff*s1 - s2
		s2 = s1
		s1 = s0
	}
	real := s1 - s2*math.Cos(w)
	imag := s2 * math.Sin(w)
	return math.Hypot(real, imag)
}

// Mean returns the arithmetic mean of x.
func Mean(x []float64) float64 {
	var s float64
	for _, v := range x {
		s += v
	}
	return s / float64(len(x))
}

// RMS returns the root-mean-square level of x.
func RMS(x []float64) float64 {
	var s float64
	for _, v := range x {
		s += v * v
	}
	return math.Sqrt(s / float64(len(x)))
}

// Corr returns the Pearson correlation coefficient of a and b (len(a) ==
// len(b)). Near zero means the signals are uncorrelated.
func Corr(a, b []float64) float64 {
	ma, mb := Mean(a), Mean(b)
	var num, da, db float64
	for i := range a {
		da_ := a[i] - ma
		db_ := b[i] - mb
		num += da_ * db_
		da += da_ * da_
		db += db_ * db_
	}
	return num / math.Sqrt(da*db)
}

// Deinterleave splits an interleaved stereo buffer [l₀,r₀,l₁,r₁,…] into its
// channels. Panics on odd length.
func Deinterleave(x []float64) (l, r []float64) {
	if len(x)%2 != 0 {
		panic("dsp: odd-length stereo buffer")
	}
	n := len(x) / 2
	l, r = make([]float64, n), make([]float64, n)
	for i := range n {
		l[i], r[i] = x[2*i], x[2*i+1]
	}
	return l, r
}

// ReadWav16 parses a 16-bit PCM WAV file into (sample rate, interleaved
// float samples scaled to ±1). Supports plain RIFF/WAVE with fmt and data
// ReadWav16 parses a 16-bit PCM WAV file into (sample rate, interleaved
// float samples scaled to ±1). See ReadWav for the general form.
func ReadWav16(b []byte) (sr int, x []float64, err error) {
	sr, x, bits, err := readWav(b, 16)
	_ = bits
	return sr, x, err
}

// ReadWav parses a 16- or 24-bit PCM WAV file into (sample rate,
// interleaved float samples scaled to ±1, bit depth).
func ReadWav(b []byte) (sr int, x []float64, bits int, err error) {
	return readWav(b, 16, 24)
}

func readWav(b []byte, allowedBits ...int) (sr int, x []float64, bits int, err error) {
	if len(b) < 44 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return 0, nil, 0, errors.New("dsp: not a RIFF/WAVE file")
	}
	var data []byte
	bits = -1
	channels, sampleRate := 0, 0
	pos := 12
	for pos+8 <= len(b) {
		id := string(b[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(b[pos+4 : pos+8]))
		body := b[pos+8 : min(pos+8+size, len(b))]
		switch id {
		case "fmt ":
			if len(body) < 16 {
				return 0, nil, 0, errors.New("dsp: short fmt chunk")
			}
			format := binary.LittleEndian.Uint16(body[0:2])
			if format != 1 {
				return 0, nil, 0, fmt.Errorf("dsp: unsupported WAV format %d (want PCM)", format)
			}
			channels = int(binary.LittleEndian.Uint16(body[2:4]))
			sampleRate = int(binary.LittleEndian.Uint32(body[4:8]))
			bits = int(binary.LittleEndian.Uint16(body[14:16]))
		case "data":
			data = body
		}
		pos += 8 + size + size%2 // chunks are word-aligned
	}
	okBits := false
	for _, a := range allowedBits {
		if bits == a {
			okBits = true
		}
	}
	if channels != 2 || !okBits || data == nil {
		return 0, nil, 0, fmt.Errorf("dsp: not a supported PCM WAV (channels=%d bits=%d data=%v)", channels, bits, data != nil)
	}

	switch bits {
	case 16:
		x = make([]float64, len(data)/2)
		for i := range x {
			x[i] = float64(int16(binary.LittleEndian.Uint16(data[2*i:2*i+2]))) / 32768
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
	return sampleRate, x, bits, nil
}
