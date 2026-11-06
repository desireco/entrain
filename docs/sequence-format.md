# Native session format

## Implemented soundscape format, version 1

Studio exports UTF-8 TOML with a `.entrain` suffix. The CLI also accepts
`.toml`. The soundscape profile is complete for a single selected
background with either a continuous bilateral cue or timed cue sections.

```toml
version = 1
title = "Soft Rain with Bells"
output = "flac"

[[voice]]
type = "soundscape"
background = "rain"
duration = "6m"
cue = "bell"
cue_volume_percent = 65
cue_interval_ms = 850
```

Render with `entrain render file.entrain -o sound.flac`. The file also
works with `entrain file.entrain`. The CLI `mix` shortcut accepts the
same sound choices. `--dry-run` displays the composition before encoding.
A native file can be rendered in any working directory using this version
of the installed CLI; the sound sources are inside the binary.

- `version = 1` and exactly one `[[voice]]` of type `"soundscape"` are required.
- `background` is `"pink"`, `"white"`, or `"rain"`. `cue` is optional.
  Omitting it or using `"none"` leaves the background alone outside any timed sections. Choose
  `"bell"`, `"tone"`, or `"noise"` for alternating cues.
- Optional `cue_volume_percent` accepts a finite number from 0 to 200.
  It scales the cue amplitude relative to its mastered level; 100 is the
  default and 0 mutes only the cue.
- Optional `cue_interval_ms` accepts a whole number from 350 to 2000.
  It sets the time between one ear’s cue and the next ear’s cue. The
  default is 625 ms. Each strike plays at its original pitch and speed.
  Both controls also validate when the cue is off. The CLI equivalents
  are `--cue-volume` and `--cue-interval-ms`.
- `duration` accepts a positive duration string (`"45s"`, `"1h30m"`)
  or a number of seconds. Duration is rounded down to the nearest whole
  stereo frame; a session must include at least one frame. WAV and FLAC
  preserve the frame count. MP3 carries padding to a codec frame boundary.
- Optional top-level `output` accepts `"wav"` (default), `"flac"`, or
  `"mp3"`. The CLI can select a format via a recognized `-o` suffix or
  `-f`. Optional `title` labels the plan. Optional `sample_rate` is
  44100 for this profile. Optional top-level `bit_depth` sets the default
  for WAV/FLAC (16 or 24, default 16). `mp3_bitrate` sets the default
  MP3 bitrate in kbps (default 320). Explicit CLI `-b` or `-q` settings
  take precedence.
- In a continuous mix, the bed and cue start at frame zero and repeat on independent clocks.
  One continuous, equal stereo gain taper covers the first and last half
  second, shortened for shorter sessions. Monitoring volume stays local
  to the playback device. A Studio mix and its equivalent CLI shortcut
  use the same mastered bed and cue samples.
- Unknown fields, unsupported voice types, and unsupported versions are
  rejected. `.sbg` files continue through the legacy renderer.

## Timed cue sections

Use the CLI to render a continuous background with cues that enter and leave
at specific times. In this six-minute example the rain starts immediately.
The bells begin on the left at 1:00 and build over 12 seconds. Their level
starts falling at 4:30 and reaches silence at 5:00. Rain continues until
6:00. [Complete editable session](../examples/rain-bell-timeline.entrain)

```toml
[[voice]]
type = "soundscape"
background = "rain"
duration = "6m"

[[voice.cue_section]]
start = "1m"
duration = "4m"
cue = "bell"
cue_interval_ms = 850
cue_volume_percent = 65
fade_in = "12s"
fade_out = "30s"
```

- Each `[[voice.cue_section]]` requires an absolute `start`, a positive
  `duration`, and a `cue` of `bell`, `tone`, or `noise`. Times accept duration
  strings or numbers of seconds. List sections in time order, within the
  parent duration. They may touch. A section begins at the first audio frame
  at or after its start and ends just before the first frame at or after its
  end.
- Omit the parent's `cue` or set it to `"none"` when defining sections.
  Each section inherits the parent's cue level and interval unless
  overridden. The first strike in each section starts on the left.
  Later strikes alternate at the section's interval through bed loop
  boundaries. Gaps between sections play only the background.
- `fade_in` and `fade_out` change only the cue gain, with a raised-cosine
  curve. Fade-in defaults to zero. Fade-out defaults to a 10 ms release
  or the remaining section length if shorter. Set either to zero for an
  immediate edge. Their total must fit within the section. A section's
  start, end and fades never reset or attenuate the underlying bed.
- `--dry-run` lists section timings and settings. `-L` renders the requested
  prefix and applies the usual output taper at its endpoint. Scheduled
  fades retain their original timing. Studio exports single continuous
  mixes. To add a schedule, edit the exported TOML and render it in the CLI.

## Proposed broader timeline format

The following sections describe the broader authoring design tracked by
issues #3 and #7. Sequential binaural voices, multi-entry crossfades, and
parallel scheduling are outside the implemented soundscape profile.
See [the CLI guide](cli-guide.md) for tested render commands and
[the migration plan](migration-plan.md) for the engine roadmap.

## File

- Extension: `.entrain` (decided Oct 2026; plain `.toml` also accepted by the reader).
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
