# Local Studio procedural audio

`player.js` is a lazy adapter for the CLI’s rain, wind and surf generators. It
loads the worker, WebAssembly and AudioWorklet on the first procedural play.
Serve the site on localhost or HTTPS. On an open page with the audio assets
loaded, no network is needed to continue or change procedural sounds.

The worker runs the Go `synth` package at **44,100 Hz**, retaining independent
state for each scene and seed. It passes planar Float32 chunks over a direct
message channel to the output worklet. A 44.1 kHz graph receives the original
samples. Other device clocks use cubic interpolation. Source pitch and
timeline speed stay fixed.

## Rebuild

From the Go module root:

```sh
./scripts/build-studio-soundscapes.sh
```

The script builds `web/soundscape-wasm` and copies the matching Go JavaScript
runtime and its license. The checked-in bundle was built with Go 1.27.3.
Using that toolchain reproduces the static files. The CLI and WebAssembly use
the same DSP source and seed derivation.

## Playback contract

```js
const source = await EntrainSoundscapes.prepare(context, {
  kind: 'rain', seed: 0, position: 0, signal, onerror: reportError
});
source.start(gainNode, when);
source.stop(fadeEnd);
```

`position` is elapsed source time in seconds. `when` and `fadeEnd` use the
audio context clock. Omitting either schedules its action now. Prepare is
cancelable. After playback starts, use `stop`; `onended` releases a caller’s
outgoing gain. The caller controls cue and master gains, fades and the selected
session deadline.

Each active source starts with up to 12 chunks of 16,384 stereo frames (about
4.46 seconds). The audio thread asks for the next chunk as it advances.
Stop, cancel or a closed context disconnect its source and close the worker
subscription. The shared worker stays idle between plays. It keeps at most 24
chunks per cached scene (3 MiB of PCM), retaining up to three recently used
scene/seed combinations in addition to simultaneous active streams. A cached
resume is immediate. A cold seek replays the deterministic generator from the
start on the worker, in cancelable batches, so its preparation time grows with
the requested offset. The selected session duration does not allocate an
equivalent audio buffer.

If the producer cannot refill before its buffered audio runs out, the worklet
fades its last output to zero, stops and notifies the transport. Real-time
playback runs on the audio clock without UI refill timers. Short offline
regression renders can use the initial preroll; use the CLI for full offline
exports.

## Silent browser check

Serve the repository root on localhost and open
`scripts/test-soundscape-stream.html`. Press **Run silent stream check**.
The recorder outputs zeros to the audio device. It exercises stereo playback
across one minute, worker refill, cancellation during a cold seek, rapid
source switches, a delayed offline start and an intentionally starved
processor. Add `?seed=8472` to repeat the capture for that variation, or
`?seed=8472&rate=48000` to exercise the alternate device clock.

## Native session editing

`await EntrainSoundscapes.parseSession(text, {signal})` uses the same strict
Go TOML decoder and soundscape validation as `entrain render`. It returns the
normalized preview settings, including timed cue sections, or rejects with
the native diagnostic. A separate lazy worker handles parsing so an invalid
edit or canceled request leaves active audio alone. The editable source text
stays with the editor, preserving its comments and formatting for download.
An already-open editor can apply further edits offline after its first parse.
The shared audio/parser WebAssembly bundle is approximately 4.1 MiB.
Open `scripts/test-studio-parser.html` on a repository-local server to check
valid, invalid and canceled parses alongside a silent audio render.
