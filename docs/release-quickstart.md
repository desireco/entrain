# Entrain CLI

The archive contains three executables, four editable examples and the MIT license. The backgrounds and cues are inside the Entrain executable. Rendering requires no network connection or separate audio assets.

## Run the examples

Extract the archive, open the extracted directory in a terminal, and run these commands there. On Windows PowerShell, replace `./entrain` with `.\entrain.exe`.

```sh
./entrain version
./entrain mix rain --cue bell --cue-volume 65 --cue-interval-ms 850 --duration 20s --save-session my-rain.entrain
./entrain render my-rain.entrain --dry-run
./entrain render my-rain.entrain -o my-rain.wav
```

Open `my-rain.wav` in your audio player. The bell alternates between the ears every 850 ms. To hear only the background, omit the cue options.

Use the included timeline to hear bells enter and leave six minutes of continuous rain. The included `.sbg` score demonstrates the legacy renderer. Its length flag below limits the audition to 15 seconds.

```sh
./entrain render examples/rain-bell-timeline.entrain --dry-run
./entrain render examples/rain-bell-timeline.entrain -o timeline.flac
./entrain render examples/focus-40-pink.sbg -L 0:00:15 -o focus.wav
```

Use `./entrain mix --help` for all soundscape options. `beatscope` inspects binaural beats in WAV and FLAC files. `noisemaker` makes standalone noise beds.
