# Entrain CLI roadmap

## Available in v0.3.0

Evolving rain, wind and surf, repeatable seeds, and exact native excerpts are released. Studio sources render offline with the installed CLI.

The [CLI release](https://github.com/desireco/entrain/releases/tag/v0.3.0) packages the renderer, Beatscope, Noisemaker and seven editable examples. The CLI runs offline from the extracted archive and reads both native TOML soundscapes and legacy `.sbg` scores. [Packaged quickstart](release-quickstart.md) · [CLI guide](cli-guide.md)

- **Repeat a mix.** `entrain mix rain --cue bell --duration 20s --save-session my-rain.entrain` writes editable source. Dry-run or render the saved file on another machine using the same version. [Native format](sequence-format.md)
- **Shape a longer session.** Play a continuous background with a cue entering later, fading in and fading out. The six-minute example starts with rain alone and plays alternating bells from minute one through minute five. [Timeline example](../examples/rain-bell-timeline.entrain)
- **Inspect an export.** `beatscope` reports binaural frequency pairs in WAV or FLAC. Native WAV, FLAC and MP3 encoders stream frames. Legacy `.sbg` rendering still builds an in-memory stereo buffer. [Analyzer](../beatscope) · [Legacy renderer](../seq/render.go)

## Next fidelity work

1. **Test repetition during a real listening session.** Packaged beds repeat a seamless 60-second section. The production recipes generate six minutes of continuously evolving material. Compare the two at matched integrated loudness on headphones for several minutes, including a bed-loop boundary. Record whether listeners can identify the repeat or prefer one rendition before increasing the bundled asset size. [Recipes](../examples/noise-backgrounds/README.md)
2. **Check the final left/right cue mix across encoders.** Subtract an otherwise identical bed-only lossless render to isolate the cues. Check cadence and dominant ear before and after the 60-second bed wrap. Decode the equivalent MP3 for a second measurement and compare true peak at maximum cue level. This would complement Beatscope’s existing binaural-pair analysis. [Cue regression tests](../internal/train/cue_test.go) · [Beatscope](../beatscope)
3. **Evaluate additional timbres or stereo width by listening.** Audition tone, bell and noise bursts over the same bed and at the same perceived loudness. Preserve the existing cue sound by default. Broader panning and evolving textures have existing stories. [Bilateral voice #5](https://github.com/desireco/entrain/issues/5) · [Textures #6](https://github.com/desireco/entrain/issues/6)

Independent voice scheduling and a shared browser engine remain separate proposals. [Scheduling #7](https://github.com/desireco/entrain/issues/7) · [WASM #8](https://github.com/desireco/entrain/issues/8)
