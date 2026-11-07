# Session library

Small, ready-to-render `.sbg` sessions — plain text, edit freely in any
editor. Amplitudes are on the 0–100 scale; keep the sum per tone-set
under 100. Headphones required for binaural beats.

| Session | What it is |
|---|---|
| `focus-40-pink.sbg` | 40 Hz gamma binaural over a pink-noise bed (1 h) |
| `meditate-alpha.sbg` | 10 Hz alpha binaural over a soft pink bed (1 h) |
| `sleep-delta.sbg` | 3 Hz delta binaural over a brown-noise bed (1 h) |
| `pink-bed.sbg` | Pink noise alone, half level (1 h) |
| `flip-10.sbg` | Hemisphere-flip binaural: 110/100 Hz swinging sides (30 min) |
| `emdr-taps.sbg` | Bilateral 600 Hz taps L/R at 1 Hz over brown noise (30 min) |
| `emdr-focus.sbg` | EMDR taps + 10 Hz alpha binaural + pink bed, mixed (30 min) |
| `emdr-timbres.sbg` | Audition tone, bell, and soft noise cues at the same rate (45 s) |

`flip-10.sbg` uses entrain's extended `.sbg` syntax (the `flip:` voice,
`flip:<carrier>±<beat>/<amp> rate<hz>`) — original sbagen cannot parse
it. Omit `rate` for the default 20-second swing cycle.

The `emdr:` voice alternates hard left and right. Omit `shape` for the
original tone, or select `shape=tone`, `shape=bell`, or `shape=noise`.
Omitting `rate` gives one full left–right–left cycle per second and two
cues per second. The cue can mix with a bed and binaural pairs in a
tone-set. The separate preview recipe below narrows the stereo placement
for its MP3s.

A starter library of 11 short curated sessions (15–25 min) —
see [`library/`](library/).

Render any of them:

```sh
entrain render examples/focus-40-pink.sbg -f mp3
entrain render examples/sleep-delta.sbg -f flac -b 24
```

Copy one, tweak the numbers, make it yours — sessions are plain text
by design.

## Noise backgrounds

Generate six-minute pink noise, white noise, and soft rain with full-length
MP3s, lossless masters, and short previews. See the
[noise background recipes](noise-backgrounds/README.md).

## Bilateral cue previews

Listen to the same 25-second phrase as a soft bell, a tone, or a noise burst.
The `emdr:` voice alternates every 625 ms at `rate0.8`. The example score
can be rendered directly or used with the following recipe for narrower,
level-matched stereo previews over quiet pink noise.

```sh
./examples/bilateral-cues/render.sh
```

The script uses Bash, Go and FFmpeg (including libmp3lame), and writes the
three MP3s to `output/bilateral-cues/`. Change the `shape=` value in
[the example score](bilateral-cues/soft-bell.sbg) to choose the cue.

## Evolving rain in the local CLI

Render [the thirty-minute rain session](evolving-rain.entrain) with `entrain render examples/evolving-rain.entrain -o evolving-rain.flac`. It contains rain alone. This local build generates a continuous bed. The v0.2.0 release renders the same session using the minute-long rain loop.
