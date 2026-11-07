//go:build js && wasm

// Browser worker bridge to the native soundscape generators. One pull writes a
// planar stereo Float32 block, suitable for Web Audio without codec gain or dither.
package main

import (
	"encoding/binary"
	"math"
	"syscall/js"

	"github.com/desireco/entrain/synth"
)

const frames = 16384

func main() {
	voices := make(map[string]synth.StereoVoice)
	reset := js.FuncOf(func(_ js.Value, a []js.Value) any {
		name := a[0].String()
		key := a[1].String()
		voice, ok := synth.NewSeededProcedural(name, 44100, int64(a[2].Int()))
		if ok {
			voices[key] = voice
		}
		return ok
	})
	var pcm [frames * 2 * 4]byte
	pull := js.FuncOf(func(_ js.Value, a []js.Value) any {
		voice := voices[a[0].String()]
		for i := range frames {
			l, r := voice.Next()
			binary.LittleEndian.PutUint32(pcm[i*4:], math.Float32bits(float32(l)))
			binary.LittleEndian.PutUint32(pcm[(i+frames)*4:], math.Float32bits(float32(r)))
		}
		return js.CopyBytesToJS(a[1], pcm[:])
	})
	drop := js.FuncOf(func(_ js.Value, a []js.Value) any { delete(voices, a[0].String()); return nil })
	// During a cold seek, advance the identical voice without allocating or
	// transporting PCM that will never be heard. The worker bounds each call.
	skip := js.FuncOf(func(_ js.Value, a []js.Value) any {
		voice := voices[a[0].String()]
		for range a[1].Int() * frames {
			voice.Next()
		}
		return nil
	})
	js.Global().Set("soundscapeSkip", skip)
	js.Global().Set("soundscapeParseSession", js.FuncOf(parseSession))
	js.Global().Set("soundscapeDrop", drop)
	js.Global().Set("soundscapeReset", reset)
	js.Global().Set("soundscapePull", pull)
	select {}
}
