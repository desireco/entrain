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

Render any of them:

```sh
entrain render examples/focus-40-pink.sbg -f mp3
entrain render examples/sleep-delta.sbg -f flac -b 24
```

Copy one, tweak the numbers, make it yours — sessions are plain text
by design.
