# Curated session library

A starter set of curated short sessions — 11 pieces, 15–25 minutes —
re-authored as SBaGen-format `.sbg` for entrain. Acoustic parameters
(carriers, beats, amplitudes, timelines) mirror the published
originals; text is re-authored.

Where entrain lacks a source feature, the closest supported
substitution is used and noted in the session header:

- isochronic tones → binaural pairs at the same beat rates (`alert-flow`)
- panned noise → wide decorrelated stereo beds (`neutral-relax`)
- `smooth N` / ease-in/out curve shapes → entrain's standard 30 s
  linear changeovers
- templates → values inlined

Doppler is native since entrain gained the `doppler:` voice:
`binaural-doppler` uses it directly (no substitution).

| Session | Length | What it is |
|---|---|---|
| `calm-state.sbg` | 15 min | Pink bed, 200 Hz carrier, 10 → 8 Hz alpha |
| `quiet-descent.sbg` | 25 min | Deep descent: 250 → 180 Hz, 10.5 → 4.5 Hz |
| `alert-flow.sbg` | 20 min | Focus climb: 250 Hz warm-up, 400 Hz rising 14 → 22 Hz |
| `neutral-relax.sbg` | 20 min | Brown bed, 250 Hz, 10 ↔ 5 Hz, very quiet |
| `alpha.sbg` | 25 min | Pure alpha: 12 → 8 Hz, no bed |
| `beta.sbg` | 25 min | Pure beta: 13 → 22 Hz, no bed |
| `theta.sbg` | 25 min | Pure theta: 7 → 4 Hz, no bed |
| `delta.sbg` | 25 min | Pure delta: 3 → 0.5 Hz, no bed |
| `binaural-doppler.sbg` | 25 min | Alpha/theta phases, native doppler pitch swing |
| `good-night.sbg` | 25 min | Sleep descent down to 2 Hz delta |

Render any of them:

```sh
entrain render examples/library/good-night.sbg -f mp3
```
