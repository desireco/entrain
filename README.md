# Entrain

**Entrain** is a clean-room Go rewrite of [SBaGen](https://uazu.net/sbagen/) — the classic
binaural-beat generator — rebuilt from scratch with a modern sequence format, lossless-first
output, and a permissive **MIT license**.

Part of the **EntrainSpace** brand. CLI binary: `entrain`. Sequence files are plain TOML —
informally, *trains*.

## What it does

Render SBaGen `.sbg` session files to audio — your existing sessions keep working:

```
entrain focus-10                  # renders focus-10.sbg → focus-10.wav
entrain focus-10 --dry-run        # show the timeline plan + lint warnings
entrain render sleep.sbg -L 0:30 -o nap.wav
```

The v0 reader covers the common `.sbg` subset: binaural tones (`carrier±beat/amp`),
plain sines, pink-noise beds, absolute/`NOW`/relative times, nested blocks, and
sbagen's default 30-second crossfades with tone continuity across changeovers
(same-frequency tones glide amplitude instead of restarting). Voice types beyond
that (bell, spin, wave, soundtrack mixing) parse with a warning today and land
later; a native TOML format ("trains") follows post-v0.1.

## Why a rewrite?

SBaGen (Jim Peters) is a wonderful piece of 2000s software: powerful, battle-tested — and
GPLv2, with a bespoke `.sbg` format and a single-file C codebase. Entrain keeps the
*functionality* and modernizes everything around it:

- **Clean-room, MIT-licensed.** No SBaGen code or documentation was copied — the
  reimplementation is derived from public DSP technique and behavioral comparison against
  the running binary. MIT means you can embed Entrain anywhere, including commercial use.
- **Reads `.sbg`, authors TOML.** Existing SBaGen sessions render unchanged; a typed,
  human-writable TOML format ("trains") arrives as the native authoring format after v0.1.
- **Modern output.** WAV / FLAC / MP3 from a single static binary — pure Go, zero cgo for
  core formats, float64 precision end-to-end, dithered 16-bit output.
- **Your audio is yours.** Rendered audio carries no license — do what you like with it.

## Status & roadmap

Early days — no usable release yet.

| Phase | Scope |
|---|---|
| 0 | Scaffold: Go module, package layout, CI — done |
| 1 | Synth core (binaural, noise palette, envelopes) → WAV — done |
| 2 | `.sbg` reader + timeline engine → `entrain <name>` renders — done |
| 3 | FLAC + MP3 encoders |
| 4 | CLI polish + behavioral validation against SBaGen |
| 5 | Later: TOML trains, Opus/AAC, live playback, frequency slides, chords |

Details: [docs/migration-plan.md](docs/migration-plan.md) · draft format spec:
[docs/sequence-format.md](docs/sequence-format.md)

## License

[MIT](LICENSE). SBaGen itself is GPLv2 software by Jim Peters; it is not included in this
repository, and this project is not affiliated with or endorsed by it.
