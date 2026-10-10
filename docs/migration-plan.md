# Migration Plan: SBaGen → Clean-Room Go Engine

Status: draft v1 (October 2026). Companion spec: [sequence-format.md](sequence-format.md).

**Naming scheme** — Brand/project: **EntrainSpace**; software/CLI: **Entrain** (binary `entrain`, from *brainwave entrainment*; `entrain.app` is an unrelated same-space web app, hence the umbrella brand). Sequence files: plain `.toml` — informally called "trains" (`entrain render sleep.toml`). Domains (Oct 2026): `entrainspace.io` available (confirmed); `.com`/`.app`/`.dev` taken. Repo: public at `github.com/desireco/entrain` (MIT). Local workspace `~/Work/entrain` holds the vendored `sbagen/` reference and stays out of git; the repo root is the `space/` subfolder.

## Goal

Replace the vendored GPLv2 SBaGen C binary with an original Go implementation of the same *functionality* — sequenced binaural-beat audio rendering — with modern output formats and a permissive license.

## Ground rules (clean-room)

- Never read `sbagen/sbagen-1.4.5/sbagen.c` or port its code; never copy text from its docs.
- Public DSP technique only: detuned carrier pairs (binaural), Voss-McCartney / Paul Kellet pink noise, linear/exponential envelopes.
- Behavioral comparison against the *running* sbagen binary is allowed (run both, listen/analyze waveforms).
- `sbagen/` stays vendored, read-only, unmodified — reference and comparison tooling only.
- Expected outcome: **MIT-licensed** codebase (LICENSE written, © 2026 Zeljko Dakic). Rendered audio is always user-owned.

## Target architecture

```
sequence (TOML) → synth (osc/noise/envelope) → mixer (float64) → encoder → file
```

- Pure Go, zero cgo for WAV / FLAC / MP3. Opus/AAC later via cgo or ffmpeg subprocess.
- Offline rendering first; live playback out of scope for v1.

## Phases

### Phase 0 — Scaffold
- Go module, package layout (`synth/`, `seq/`, `codec/`, `cmd/entrain`), CI, license file.
- Exit: `go build ./...` green, repo skeleton committed.

### Phase 1 — Synth core → WAV
- Sine oscillator (phase-accumulating, float64), binaural voice (L/R = carrier ± beat/2), pink noise, amplitude envelopes with crossfade.
- 44.1k/48k, dithered 16-bit WAV out.
- Exit: render a known sequence by hand-written code; waveform spot-checks (frequency peaks via FFT/analysis) match math expectations.

### Phase 2 — Sequence format v0 + timeline engine
- Parse the TOML schema from [sequence-format.md](sequence-format.md); sequential timeline with crossfades.
- Exit: `entrain render session.toml -o out.wav` reproduces Phase-1 results from a file.

### Phase 3 — Encoders
- FLAC via `mewkiz/flac`; MP3 320kbps via `braheezy/shine-mp3`; dithering stage shared.
- Exit: same session renders to wav/flac/mp3; spot-check spectrum integrity; **decision point**: Shine quality acceptable, or wire LAME (cgo) as optional build.

### Phase 4 — CLI polish + behavioral validation
- Flags: sample rate, bit depth, output format, output path; sensible errors; docs.
- Validation: comparable sequences rendered by both engines, compare spectral content perceptually/numerically.
- Exit: usable v0.1 tagged, MIT license in place.

### Phase 5 — Later / optional
- Opus/AAC outputs, voice types beyond binaural/pink (bell, spin — from public descriptions only), background audio mixing, live playback, drift/slides (`slide_to`), randomized variation.

## Risks

- Shine MP3 quality may fall short for critical listening → mitigation is the LAME cgo path, isolated behind the encoder interface.
- Format schema will evolve during Phase 2 — spec marked draft on purpose; breaking changes acceptable until v0.1.
- Clean-room discipline is procedural, not technical — keep `sbagen/` untouched and resist "just checking" the C source.
