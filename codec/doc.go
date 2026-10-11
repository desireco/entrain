// Package codec will encode rendered float64 audio to WAV, FLAC, and MP3
// (320 kbps), with a shared clip → dither → quantize stage.
// Pure Go, zero cgo for all three. (Phase 3.)
package codec
