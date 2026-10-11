// Package codec encodes rendered float64 audio to output files: 16-bit
// dithered WAV now; FLAC and MP3 (320 kbps) arrive in Phase 3. All encoders
// share one clip → dither → quantize stage and are pure Go.
package codec
