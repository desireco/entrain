# Entrain CLI v0.3.0

The archive contains three executables, seven editable examples and the MIT license. Backgrounds, generators and cues are inside the Entrain executable. Rendering works offline without separate audio assets.

You can also install the renderer with Go:

```sh
go install github.com/desireco/entrain/cmd/entrain@v0.3.0
entrain version
```

Add your Go bin directory to PATH, then use `entrain` in place of
`./entrain` in the examples below. The packaged examples are also available
at https://github.com/desireco/entrain/tree/v0.3.0/examples.

## Render a session

Extract the archive and open its directory in a terminal. On Windows PowerShell, replace `./entrain` with `.\entrain.exe`.

```sh
./entrain version
./entrain mix surf --seed 413 --cue bell --cue-volume 65 --cue-interval-ms 850 --duration 20s --save-session my-surf.entrain
./entrain render my-surf.entrain --dry-run
./entrain render my-surf.entrain -o my-surf.wav
```

Open `my-surf.wav` in your player. The bell alternates between ears every 850 ms. Omit the cue options to hear the background alone. The backgrounds are `pink`, `white`, `rain`, `wind`, `surf` and `rain-loop`.

Download a `.entrain` file from https://entrain.cc/studio/ and use its filename in the render command above. Source files can be edited as plain text. A seed chooses a repeatable variation of rain, wind or surf. Omitted or zero uses the default performance.

## Preview a cue entrance

The included wind and surf sessions introduce bells at 18 seconds. Render a ten-second excerpt beginning six seconds before that entrance:

```sh
./entrain render examples/surf-bells.entrain --start 12s -L 10s -o surf-entrance.wav
./entrain render examples/wind-bells.entrain --start 12s -L 10s -o wind-entrance.flac
```

An explicit `--start` selects exact samples from the original native timeline, including cue sections and fades. Without `--start`, a shortened `-L` render gets its own ending fade. Native session excerpts end at the full session endpoint if necessary.

For the full session or another example:

```sh
./entrain render examples/evolving-rain.entrain -o rain-30m.flac
./entrain render examples/rain-bell-timeline.entrain --dry-run
./entrain render examples/rain-bell-timeline.entrain -o timeline.flac
./entrain render examples/focus-40-pink.sbg -L 0:00:15 -o focus.wav
```

The last command exercises the legacy `.sbg` renderer, whose length uses a clock value. Use `./entrain mix --help` and `./entrain render --help` for other options. `beatscope` inspects binaural beats in WAV and FLAC files. `noisemaker` makes standalone noise beds.

## Earlier rain sessions

In v0.3.0 `rain` evolves continuously. To reproduce the original v0.2.0 minute loop, select `rain-loop` instead. An older file that names `rain` therefore gets different rain samples in this release. Cue settings and timings remain applicable to either bed. Pink and white retain their mastered loops.
