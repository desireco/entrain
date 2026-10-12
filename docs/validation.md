# Cross-Engine Validation

How we verify entrain renders what SBaGen renders: render the same
session through both engines, detect the binaural beats in both outputs
with `beatscope`, and compare the (carrier, beat) sets. This is the
behavioral-comparison methodology the clean-room ground rules allow
([migration-plan.md](migration-plan.md)) — the reference is the running
binary, never its source.

## Harness

```sh
go build -o /tmp/bin ./cmd/...          # entrain + beatscope
PATH=/tmp/bin:$PATH \
SBAGEN=/path/to/sbagen \
scripts/validate-vs-sbagen.sh session.sbg ...
```

Per session, both engines render 30 s WAVs (`-L`, default fades) and
beatscope reports each side's detected pairs at confidence ≥ 0.5. A
session matches when the detected sets are equal. Note: sessions with
`-m` soundtracks need an OGG-enabled sbagen build (the stock 1.4.5
prebuilt lacks it); the vendored fork build carries it.

## Recorded results (2026-10-12, v0.1.0)

| Session | Result | Detected pairs (both engines) |
|---|---|---|
| focus-40-24-pink (owner's) | **MATCH** | 180 Hz @ 40 Hz, 320 Hz @ 24 Hz |
| ts-brain-alpha | **MATCH** | 300 Hz @ 10 Hz |
| ts-brain-beta | **MATCH** | 300 Hz @ 16 Hz |
| ts-brain-delta | **MATCH** | 150 Hz @ 2 Hz |
| ts-brain-theta | **MATCH** | 150 Hz @ 6 Hz |
| prog-drop-old-demo (30 s) | DIFFER — expected | entrain-only: 205 Hz @ 10 Hz |
| prog-drop-old-demo (3 min) | MATCH (both quiet) | none above the detection floor |

The drop-demo divergence is the known, warned limitation, not a defect
in the comparison: its tones sit at amplitude 1.0 (0.01 peak) behind
`->` hour-long slide periods, which entrain approximates with default
`--` fades (parsed, semantics deferred — see the Phase 2 record).
Entrain's fades ramp to full level within 30 s, so the very quiet tone
is detectable there but not yet in sbagen's render; by 3 minutes both
engines' output is below beatscope's confidence floor and the sets
agree. Implementing `==`/`->` slide semantics closes the gap and is
tracked on the roadmap.

## Scope

Covered today: tone-set subset (sines, binaural pairs, pink beds),
absolute/NOW/relative timelines, default crossfades, tone continuity.
Not exercised by this table: `==`/`->` slides (see above), bell/spin/
wave voices, `-p` predefined sequences (out of scope). `-m` bed mixing
is implemented since the noise split (looping beds; re-run this harness
with a WAV bed to extend the table). Add rows here whenever a semantics gap is
closed or a new voice lands.
