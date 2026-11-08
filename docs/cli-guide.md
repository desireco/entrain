# Entrain CLI guide

Entrain renders a stereo audio file you can play in your usual player.
Use `mix` for a one-off render or save a native `.entrain` file from
the terminal or Studio to reproduce it later.

Install v0.3.0 to render the current [Studio](https://entrain.cc/studio/)
files, including evolving rain, wind, surf, seeds and timed cues.
Download a [platform archive and checksum](https://github.com/desireco/entrain/releases/tag/v0.3.0),
or install the renderer with Go:

```sh
go install github.com/desireco/entrain/cmd/entrain@v0.3.0
entrain version
```

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
when working from the checkout. The local executable includes the procedural
generators, mastered beds and isolated cues. It can render offline
from any working directory.

To serve matching downloads and browser audio from a source checkout, run
`scripts/build-studio-cli.sh` and `scripts/build-studio-soundscapes.sh` from
the repository root. Serve `site/` on localhost, for example with
`python3 -m http.server 8872 --bind 127.0.0.1 --directory site`.

## Make a short preview

Choose `pink` for a warm, shifting noise bed, `white` for steady filtered
air, or `rain` for synthetic rain. This command writes just the 20-second
rain background. Open `rain-solo.wav` in a player to hear it.

```sh
entrain mix rain --duration 20s -o rain-solo.wav
```

### Reproduce a procedural variation

Set `--seed` to select a different performance by the same rain, wind, or
surf generator. For example:

```sh
entrain mix surf --seed 413 --cue bell --duration 30s --save-session surf-413.entrain
entrain render surf-413.entrain -o surf-413.wav
```

Use a whole number from 0 to 2147483647. Omitting the seed or choosing 0
preserves the original sound. With the same build, seed and settings, the
performance repeats sample-for-sample. The cue timing and level remain
independent of the seed. A positive seed is unsupported on mastered loops
(`pink`, `white`, `rain-loop`). Saved sources keep a positive seed
in their `[[voice]]` entry.

### Rain, wind and surf

For thirty minutes of rain alone, render the included native session or use
the equivalent mix command. Both generate the stereo bed from the beginning,
with the same result for the same settings in this build.

```sh
entrain render examples/evolving-rain.entrain -o evolving-rain.flac
entrain mix rain --duration 30m -f flac -b 24 -o evolving-rain.flac
```

The v0.3.0 CLI and the public Studio continue generating
rain, wind and surf past each minute boundary. Wind builds from gentle gusts;
surf moves through swells, breaking waves and foam. The same cue choices
and timing controls work over all three backgrounds.

```sh
entrain mix wind --duration 20m -o wind.flac
entrain mix surf --cue bell --cue-volume 55 --cue-interval-ms 900 --duration 12m -o surf-bells.flac
```

For a longer mix with a cue-free opening, try the included
[wind and bells](../examples/wind-bells.entrain) and
[surf and bells](../examples/surf-bells.entrain) sessions. Each runs
for twelve minutes and introduces the bells at 18 seconds.

The Studio plays rain, wind and surf for the selected file duration. Its
pink and white previews stop after one minute or at the end of a shorter file.

Use the v0.3.0 CLI to render the current Studio files. The earlier v0.2.0 CLI uses a mastered
60-second rain loop and predates wind and surf. An existing file with `background = "rain"`
uses evolving rain when rendered with the new CLI. To reproduce its
v0.2.0 bed with this build, change just the background to `"rain-loop"`
or use `entrain mix rain-loop`. The v0.2.0 CLI uses `rain` for that loop.
Cue settings and timed cue sections work with either bed.

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
including long sessions. In v0.3.0 rain, wind and surf run for the
whole session. Pink, white and `rain-loop` use 60-second mastered loops.
Cue timing runs independently of each background. For the evolving six-minute takes, see
the [full-length background recipes](../examples/noise-backgrounds/README.md).

## Cut an exact listening excerpt

To compare the entrance of the bells against the full surf session, start at
12 seconds and keep 30 seconds. You hear six seconds of surf alone before the
first bell starts to rise. The quiet opening of the complete session is
available separately.

```sh
entrain render examples/surf-bells.entrain --start 12s -L 30s --dry-run
entrain render examples/surf-bells.entrain --start 12s -L 30s -o surf-entrance.wav
```

`--start` selects a position on the original native timeline. A WAV or FLAC
at the master’s bit depth contains the same decoded samples as that span of
the full render, including noise state, alternating sides, fades and dither.
Use `--start 0` for an exact opening. Without `--start`, `-L` still makes a
short standalone rendition with its own ending taper.

Use durations such as `4m20s`, fractional seconds such as `1.375s`, or clocks
such as `0:04:20`. A two-field clock means hours and minutes. Omit `-L` to
continue to the end. A length extending past the session ends there. The
range includes its start and excludes its end, rounding each boundary up to
a sample and stopping at the full render's actual frame count. Dry-run shows
the source frames before audio is generated. The output defaults to
`<session>.excerpt.<format>` so a short audition has its own filename.

A late excerpt advances the generator from the beginning to recover its
state, so seeking deeper takes longer. Memory stays bounded and only the
selected frames are encoded. Cutting mid-wave can give a sharp edge when
played by itself. MP3 encodes an independent excerpt and has codec delay
and padding. Use a lossless format for sample comparisons. This option
currently supports native `.entrain` and `.toml` files.

## Author cue sections and audition an excerpt

In Studio, choose **Timed sections** under Alternating cue. Add sections in
time order. Set each section’s start, length, cue shape, level, interval and
cue fades. The sections may touch; overlap and fades longer than a section
are flagged before preview or export. Gaps play only the background.

Select **Preview entrance** to cue a window from five seconds before the entrance through 25 seconds after it (within the file), or choose **Preview an excerpt**
and enter a start and length. Press Play to hear that range in its session
position. Copy session and Download always retain the full session; use the
separate CLI excerpt command to render just the range you auditioned.

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
