# Choosing a sound

Start with the cue off. Play each background at the same low, comfortable level for a few seconds. If the room has intermittent sounds that catch your attention, compare the track with quiet while doing the same task. Choose the setting you find easiest to leave in the background.

| Sound | What to listen for |
| --- | --- |
| Pink noise | A warm wash with a quieter, airy layer. Ideal pink noise spreads equal energy across each octave, such as 250–500 Hz and 500–1,000 Hz. |
| White noise | A steady hiss. Ideal white noise spreads equal energy across each hertz. Entrain filters the upper frequencies to soften its edge. |
| Soft rain | A synthetic rain texture. Slowly varying bands of generated noise give the shower a finer hiss above a steadier wash. |
| Brown noise | A darker rumble with still more weight at low frequencies. The separate `noisemaker brown 30s` command generates it. |

The colors describe the spectrum supplying the sound. Entrain shapes its listening tracks further, so a track’s final spectrum can differ from its ideal noise color. [MathWorks’ colored-noise reference](https://www.mathworks.com/help/dsp/ref/dsp.colorednoise-system-object.html) · [Entrain background recipes](../examples/noise-backgrounds/README.md)

## Pitch, beats, and alternating cues

- **Pitch.** A 200 Hz tone vibrates 200 times per second. It is a sound frequency you can hear.
- **Binaural beat.** With stereo headphones, a steady 200 Hz tone in one ear and 210 Hz in the other can produce the perception of a 10 Hz beat. The 10 Hz is the difference between the two pitches. [Binaural-beat research review](https://doi.org/10.1371/journal.pone.0286023)
- **Alternating cue.** A bell, tone, or short noise burst moves from one ear to the other. At the default 625 ms interval, each cue begins 0.625 seconds after the preceding cue. You hear 1.6 cues per second and return to the same ear every 1.25 seconds. Stereo headphones make the direction easiest to hear. The background works by itself on headphones or speakers. [Entrain cue examples](../examples/bilateral-cues/soft-bell.sbg)

EMDR is a structured psychotherapy in which a clinician can use bilateral stimulation as one component. Entrain’s `emdr:` voice name refers to the audio pattern it generates. [US Department of Veterans Affairs overview](https://www.ptsd.va.gov/professional/treat/txessentials/emdr_pro.asp)

## What the EEG band names mean

EEG measures electrical activity at the scalp. One clinical convention groups measured frequencies into delta (roughly below 4 Hz), theta (4 to below 8 Hz), alpha (8–13 Hz), beta (14–30 Hz), and gamma (above 30 through 80 Hz). Boundaries depend on convention. For example, an alpha rhythm is often visible during relaxed wakefulness with eyes closed, while brain waves slow during deep non-REM sleep. Those observations come from recordings of people. An Entrain preset labeled “alpha” tells you about the beat chosen for the recording. Determining a listener’s brain activity would require a measurement. A review of binaural-beat studies found inconsistent evidence for a corresponding change in EEG rhythms. [International Federation of Clinical Neurophysiology glossary](https://doi.org/10.1016/j.cnp.2017.07.002) · [US National Institute of Neurological Disorders and Stroke guide to sleep](https://www.ninds.nih.gov/sites/default/files/2025-05/understanding-sleep.pdf) · [Binaural-beat research review](https://doi.org/10.1371/journal.pone.0286023)

## What a background might help with

**Masking.** A steady background can make an intermittent sound less noticeable. For everyday listening, a useful comparison is whether the nearby conversation or traffic stands out less, and whether you prefer the added sound to quiet. [Systematic review of auditory stimulation and sleep](https://pubmed.ncbi.nlm.nih.gov/34964434/)

**Concentration.** A meta-analysis found a small improvement on laboratory attention tasks among young people with elevated attention difficulties, and poorer performance in comparison groups without those difficulties. The studies tested specific tasks and populations. A short comparison with your own work is a better guide to whether you prefer the sound. [2024 task-performance meta-analysis](https://pubmed.ncbi.nlm.nih.gov/38428577/)

**Sleep.** A systematic review found very low-quality evidence for continuous noise as a sleep aid. In one later seven-night study of 25 healthy adults, continuous pink noise reduced REM sleep compared with a quiet night. A track can feel pleasant while awake even when its effects on sleep are uncertain. [Systematic review](https://pubmed.ncbi.nlm.nih.gov/33007706/) · [Controlled sleep-laboratory study](https://pubmed.ncbi.nlm.nih.gov/41627391/)
