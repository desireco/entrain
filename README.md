# Entrain

**Entrain** is a clean-room Go rewrite of [SBaGen](https://uazu.net/sbagen/) — the classic
binaural-beat generator — rebuilt from scratch with a modern sequence format, lossless-first
output, and a permissive **MIT license**.

Part of the **EntrainSpace** brand. CLI binary: `entrain`. Sequence files are plain TOML —
informally, *trains*.

Project homepage: **[entrain.cc](https://entrain.cc/)**

## Install the CLI

[v0.2.0 downloads and checksums](https://github.com/desireco/entrain/releases/tag/v0.2.0) are available for Linux and macOS on x86-64 and ARM64, and Windows on x86-64. Each archive includes the three tools, an offline quickstart and example sessions.

With Go, install this version of the renderer directly:

```sh
go install github.com/desireco/entrain/cmd/entrain@v0.2.0
entrain version
```

## What it does

Render a native session, save a mix from the terminal, or schedule cues within a longer background:

```sh
entrain render examples/rain-bell.entrain --dry-run
entrain render examples/rain-bell.entrain -o rain-bell.flac
entrain mix pink --duration 20m -o pink.wav
entrain mix rain --cue bell --duration 20s --save-session my-rain.entrain
entrain render my-rain.entrain -o my-rain.wav
entrain render examples/rain-bell-timeline.entrain --dry-run
entrain mix rain --cue bell --cue-volume 65 --cue-interval-ms 850 --duration 10m -o rain-bell.mp3
```

Choose `pink`, `white`, or `rain`. The background plays alone by default.
Use `--cue bell`, `tone`, or `noise` to add one alternating sound.
Set cue amplitude and time between alternating onsets independently.
The defaults are 100% and 625 ms. See the [CLI guide](docs/cli-guide.md)
for a short listening preview, parameter ranges, and export commands.
Use the [listening guide](docs/listening-guide.md) to compare the sounds.
The default length is six minutes. The installed CLI includes
60-second seamless bed loops and sixteen isolated alternating strikes per cue timbre.
It writes a file offline. The separate six-minute production masters play
the full evolving take. See the [native format](docs/sequence-format.md)
for the editable source and [the recipes](examples/noise-backgrounds/README.md)
to produce the full-length tracks.

From this checkout, substitute `go run ./cmd/entrain` for `entrain` in
these commands. To install this specific version, run
`go install ./cmd/entrain` from the repository root.

Render SBaGen `.sbg` session files to audio — your existing sessions keep working:

```
entrain focus-10 -f mp3            # renders focus-10.sbg → focus-10.mp3 (320 kbps)
entrain focus-10 --dry-run        # show the timeline plan + lint warnings
entrain render sleep.sbg -f flac -b 24 -L 0:30 -o nap.flac
```

Verify what any renderer (this one included) put in a file, and make noise on its own:

```
beatscope session.wav              # carrier / beat / confidence per pair
noisemaker brown 30m -o bed.flac   # standalone noise, entrain-calibrated level
noisemaker alpha 1h --alpha 1.4    # arbitrary 1/f^alpha slopes
```

## Extended .sbg syntax

Entrain's own voices and transitions, layered onto the `.sbg` grammar.
Original sbagen files parse unchanged; files using extensions carry an
`## extended syntax: entrain` marker (entrain warns when it's missing):

```
flip:105+10/20 rate0.05     # hemisphere-flip binaural: the pair swings sides
emdr:600/30 rate1.0         # bilateral tone taps, hard L/R alternation
emdr:420/18 shape=bell      # soft bilateral bell at the default rate
emdr:420/18 shape=noise     # soft bilateral noise bursts
doppler:250+10/20 rate0.1   # cyclic pitch swing; the beat stays constant

00:00:30 alpha fade10 ease-in   # 10 s changeover, quadratic ease-in
-changeover 15                  # session-wide default changeover (seconds or h:mm:ss)
```

The optional `shape=tone|bell|noise` selects a bilateral cue. Omitting
`shape` plays the original tone. The frequency sets the pitch of the tone
or bell and the brightness of the noise burst. Omitting `rate` gives one
full left–right–left cycle per second, or two cues per second. The score
places each cue entirely in one ear. Compare all three at the same pace
with `entrain render examples/emdr-timbres.sbg -f wav`.

See `examples/flip-10.sbg`, `examples/emdr-taps.sbg`,
`examples/emdr-focus.sbg` and `examples/library/` for sessions using
them.

The v0 reader covers the common `.sbg` subset: binaural tones (`carrier±beat/amp`),
plain sines, pink-noise beds, absolute/`NOW`/relative times, nested blocks, and
sbagen's default 30-second crossfades with tone continuity across changeovers
(same-frequency tones glide amplitude instead of restarting). The legacy reader also mixes stereo WAV soundtracks with `-m` and `mix/amp`. Unsupported legacy voices such as spin and wave generate warnings. Native TOML sessions support soundscapes with timed cue windows and fades.

## Why a rewrite?

SBaGen (Jim Peters) is a wonderful piece of 2000s software: powerful, battle-tested — and
GPLv2, with a bespoke `.sbg` format and a single-file C codebase. Entrain keeps the
*functionality* and modernizes everything around it:

- **Clean-room, MIT-licensed.** No SBaGen code or documentation was copied — the
  reimplementation is derived from public DSP technique and behavioral comparison against
  the running binary. MIT means you can embed Entrain anywhere, including commercial use.
- **Reads `.sbg` and native TOML.** Existing SBaGen sessions render as before.
  The CLI saves editable, self-contained `.entrain` soundscape sessions.
- **Modern output.** WAV / FLAC / MP3 from a single static binary — pure Go, zero cgo for
  core formats, float64 precision end-to-end, dithered 16-bit output.
- **Your audio is yours.** Rendered audio carries no license — do what you like with it.

## Status & roadmap

v0.2.0 adds native soundscapes, source export and a cue timeline. See the [release notes](docs/releases/v0.2.0.md) for installation and examples.

| Phase | Scope |
|---|---|
| 0 | Scaffold: Go module, package layout, CI — done |
| 1 | Synth core (binaural, noise palette, envelopes) → WAV — done |
| 2 | `.sbg` reader + timeline engine → `entrain <name>` renders — done |
| 3 | FLAC + MP3 encoders — done (MP3 via pure-Go Shine at up to 320 kbps) |
| 4 | Beatscope analyzer, 24-bit output, cross-engine validation — done ([results](docs/validation.md)) |
| 5 | Native TOML soundscapes, timed cue windows and source export shipped in v0.2.0. Broader voice timelines and additional codecs remain future work. |

See the [CLI roadmap](docs/cli-roadmap.md), [migration plan](docs/migration-plan.md), and native format reference:
[docs/sequence-format.md](docs/sequence-format.md)

## License

[MIT](LICENSE). SBaGen itself is GPLv2 software by Jim Peters; it is not included in this
repository, and this project is not affiliated with or endorsed by it.
