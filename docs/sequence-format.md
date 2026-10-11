# Sequence Format Spec (draft v0)

Status: **draft, deferred** — .sbg read-compatibility is the v0 frontend (see migration-plan Direction); this TOML schema is the native authoring format, planned post-v0.1. Companion: [migration-plan.md](migration-plan.md).

Goal: a modern, typed replacement for SBaGen's `.sbg` format. TOML, human-writable, no reimplementation of `.sbg` syntax or semantics.

## File

- Extension: `.toml` (no custom extension for v0).
- Encoding: UTF-8. Comments via `#` (native TOML).

## Top-level keys

| Key | Type | Default | Description |
|---|---|---|---|
| `title` | string | `""` | Human label; becomes metadata/tags in outputs where supported. |
| `sample_rate` | int | `44100` | `44100` or `48000`. |
| `output` | string | `"wav"` | `wav` \| `flac` \| `mp3`. CLI `-o` overrides. |
| `fade_default` | duration | `"10s"` | Default crossfade for entries without explicit `fade`. |

## Timeline entries: `[[voice]]`

Entries play **in listed order, sequentially**. Each entry is one voice. Fields:

| Field | Type | Required | Applies to | Description |
|---|---|---|---|---|
| `type` | string | yes | all | `"binaural"` or `"pink"` (v0). |
| `duration` | duration | yes | all | Length of the entry. |
| `amplitude` | float 0..1 | no (default `0.5`) | all | Peak level of this entry. |
| `fade` | duration | no (`fade_default`) | all | Crossfade at both boundaries: fades in from previous entry, fades out into next. |
| `carrier` | float Hz | yes | `binaural` | Carrier frequency (e.g. `200`). Both channels centered on it. |
| `beat` | float Hz | yes | `binaural` | L/R frequency difference (e.g. `10` → L=205, R=195). |
| `pan` | float −1..1 | no (default `0`) | all | Reserved (v0: ignored). |

Duration syntax: `"25m"`, `"90s"`, `"1h30m"`, plain seconds number allowed (`duration = 300`).

## Example

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

[[voice]]
type = "binaural"
carrier = 150
beat = 4
duration = "30m"
amplitude = 0.7
```

First entry fades in from silence; last fades out to silence.

## Semantics notes

- Rendering is float64 end-to-end; entries are summed during crossfade overlap; final peak scaling/clipping policy: hard-clip at 1.0 after dither (tbd — may become peak-normalize option).
- Multiple simultaneous voices in v0: only via crossfade overlap. True parallel scheduling (`at = "10m"`) is planned for v1.

## Open questions (to resolve during Phase 2)

- Chords (multiple simultaneous binaural pairs) — separate `[[voice]]` with `at`, or a `[[chord]]` type?
- Noise voice schema: generalize to `type = "noise"` + `color = "pink"|"brown"|"white"`, or keep per-color types?
- Bilateral/EMDR voice fields: `rate`, `tone`, `shape` (beep/click/noise-burst), pan curve — first post-v0.1; requires parallel scheduling for mixes.
- Parallel scheduling (`at`): promoted to first post-v0.1 work — EMDR + binaural + noise mixes need it.
- Frequency slides/drift (`slide_to`, random wobble) — entry fields or separate automation section?
- Amplitude automation over an entry (ramps beyond fades).
- MP3 metadata mapping (`title`, future `album`/`genre`).
- Peak handling: normalize vs clip.
