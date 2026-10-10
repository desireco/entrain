# Entrain

**Entrain** is a clean-room Go rewrite of [SBaGen](https://uazu.net/sbagen/) — the classic
binaural-beat generator — rebuilt from scratch with a modern sequence format, lossless-first
output, and a permissive **MIT license**.

Part of the **EntrainSpace** brand. CLI binary: `entrain`. Sequence files are plain TOML —
informally, *trains*.

## What it does

Describe a session as a TOML file — a timeline of binaural tones and pink noise with
durations, amplitudes, and crossfades — and render it to WAV, FLAC, or MP3:

```
entrain render sleep.toml -o sleep.flac
```

Example session:

```toml
title = "Deep Sleep Descent"
sample_rate = 44100
output = "flac"
fade_default = "30s"

[[voice]]
type = "pink"
duration = "1h"
amplitude = 0.3

[[voice]]
type = "binaural"
carrier = 200
beat = 10
duration = "25m"
amplitude = 0.7
fade = "1m"
```

## Why a rewrite?

SBaGen (Jim Peters) is a wonderful piece of 2000s software: powerful, battle-tested — and
GPLv2, with a bespoke `.sbg` format and a single-file C codebase. Entrain keeps the
*functionality* and modernizes everything around it:

- **Clean-room, MIT-licensed.** No SBaGen code or documentation was copied — the
  reimplementation is derived from public DSP technique and behavioral comparison against
  the running binary. MIT means you can embed Entrain anywhere, including commercial use.
- **TOML sequences.** A typed, human-writable format instead of `.sbg` syntax.
- **Modern output.** WAV / FLAC / MP3 from a single static binary — pure Go, zero cgo for
  core formats, float64 precision end-to-end, dithered 16-bit output.
- **Your audio is yours.** Rendered audio carries no license — do what you like with it.

## Status & roadmap

Early days — no usable release yet.

| Phase | Scope |
|---|---|
| 0 | Scaffold: Go module, package layout, CI |
| 1 | Synth core (binaural, pink noise, envelopes) → WAV |
| 2 | TOML sequence format + timeline engine |
| 3 | FLAC + MP3 encoders |
| 4 | CLI polish + behavioral validation against SBaGen |
| 5 | Later: Opus/AAC, live playback, frequency slides, chords |

Details: [docs/migration-plan.md](docs/migration-plan.md) · draft format spec:
[docs/sequence-format.md](docs/sequence-format.md)

## License

[MIT](LICENSE). SBaGen itself is GPLv2 software by Jim Peters; it is not included in this
repository, and this project is not affiliated with or endorsed by it.
