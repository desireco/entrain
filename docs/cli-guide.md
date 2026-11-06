# Entrain CLI guide

Entrain renders a stereo audio file you can play in your usual player.
Use `mix` for a one-off render or save a native `.entrain` file from
the terminal or Studio to reproduce it later.

## Run this checkout

From the repository root, install the CLI and check its built-in help.

```sh
go install ./cmd/entrain
entrain mix --help
```

Go puts the executable in `GOBIN`, or the `bin` directory under your Go
workspace when `GOBIN` is unset. If that directory is outside your shell's
`PATH`, run the local source directly.

```sh
go run ./cmd/entrain mix --help
```

Substitute `go run ./cmd/entrain` for `entrain` in the commands below
when working from the checkout. An installed executable includes all
three mastered backgrounds and the isolated cues. It can render offline
from any working directory.

## Make a short preview

Choose `pink` for a warm, shifting noise bed, `white` for steady filtered
air, or `rain` for synthetic rain. This command writes just the 20-second
rain background. Open `rain-solo.wav` in a player to hear it.

```sh
entrain mix rain --duration 20s -o rain-solo.wav
```

To hear a cue over the same bed, choose `bell`, `tone`, or `noise`. Here
the bell alternates between the ears every 850 milliseconds at 65% of
its mastered amplitude. The background stays at its original level.

```sh
entrain mix rain --cue bell --cue-volume 65 --cue-interval-ms 850 --duration 20s -o rain-bell-preview.wav
```

| Option | Meaning |
| --- | --- |
| `--duration` | Total session length. Examples include `20s`, `6m`, and `1h30m`. The default is six minutes. |
| `--cue` | Choose `tone`, `bell`, `noise`, or `none`. Omitting it plays the background by itself. |
| `--cue-volume` | Cue amplitude relative to its mastered level. Accepts 0–200%, including fractional values. The default is 100%. At 50% its amplitude halves. At 0% only the bed plays. |
| `--cue-interval-ms` | Time between successive cue onsets, in whole milliseconds from 350 to 2000. The default is 625. At 850, a left cue starts at 0 s, a right cue at 0.85 s, and the next left cue at 1.70 s. Changing the spacing preserves pitch and decay. |

Use each cue's default level and timing by omitting both controls. Device
monitoring volume in Studio is separate from the exported cue volume.

## Save a mix from the terminal

Use `--save-session` to create editable source before rendering. This
example writes just `rain-study.entrain`. The file contains the selected
background, exact duration, cue settings and 24-bit FLAC output preference.

```sh
entrain mix rain --cue bell --cue-volume 65 --cue-interval-ms 850 --duration 20s -f flac -b 24 --save-session rain-study.entrain
entrain render rain-study.entrain --dry-run
entrain render rain-study.entrain
```

The final command writes `rain-study.flac` next to the source. You can move
the source to another machine running this CLI and issue the same render
command there. A muted cue with `--cue-volume 0` retains its timbre and
interval for editing. The CLI also records the encoder defaults, so an
MP3 saved with `-f mp3 -q 192` renders at 192 kbps by default. Override
the saved settings on an individual render with `-f`, `-b`, or `-q`.

Choose a new `.entrain` or `.toml` path each time you save. Existing files
are protected. Use `--dry-run --save-session new.entrain` to see the exact
TOML without writing it. The `-o` flag selects an audio destination
during rendering, as in `entrain render rain-study.entrain -o preview.wav`.

## Inspect and render a Studio file

Save the `.entrain` file from Studio. Use its actual filename in place of
`examples/rain-bell.entrain` below. The example is also in the repository.

```sh
entrain render examples/rain-bell.entrain --dry-run
entrain render examples/rain-bell.entrain -L 0:00:20 -o rain-bell-preview.wav
entrain render examples/rain-bell.entrain -o rain-bell.flac
```

The first command lists the title, background, cue settings, duration,
sample rate and destination. The second renders the opening 20 seconds
with an ending fade. The third renders the full session. Move the source
to another directory or computer with the same CLI build to reproduce it.

Choose a WAV destination for editing, a FLAC destination for lossless
storage, or an MP3 destination for a smaller listening copy. WAV and FLAC
preserve the requested whole-frame duration. MP3 adds encoder-frame
padding. The default WAV/FLAC depth is 16 bits; use `-b 24` for 24 bits.
An explicit `-f` must agree with a recognized output suffix. For example:

```sh
entrain render examples/rain-bell.entrain -b 24 -o rain-bell-24.flac
entrain render examples/rain-bell.entrain -q 192 -o rain-bell.mp3
```

All soundscape renders are 44.1 kHz stereo. The CLI streams the output,
including long sessions. It repeats a seamless 60-second section of the
selected six-minute background master and sequences sixteen alternating
recorded strikes independently. For the evolving six-minute takes, see
the [full-length background recipes](../examples/noise-backgrounds/README.md).

## Change the session later

Open the `.entrain` file in a text editor and change its `[[voice]]`
settings. Set `cue = "none"` or remove `cue` to play just the background.
Try `cue_interval_ms = 1200` to give each bell more space, or set
`cue_volume_percent = 40` to lower it without changing the bed.
Inspect the edited file using `--dry-run` before rendering.
See the [native format reference](sequence-format.md) for the full schema.

Legacy `.sbg` sessions still use their own voice and timeline syntax.
Render them with the usual command, for example
`entrain render examples/emdr-timbres.sbg -f wav`.
