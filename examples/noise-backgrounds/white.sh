#!/usr/bin/env bash
# Six minutes of soft air, from one continuous seeded stereo white-noise source.
# Run in the producer's scratch directory with NOISEMAKER set to the built CLI.
set -euo pipefail
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT

"$NOISEMAKER" white 360s --seed 4751 --level 0.155 -r 48000 -b 24 -f wav -o "$scratch/source.wav"

# The main layer keeps a steady center. A quieter band of the same white source
# retains its original width and changes color gently over the six-minute take.
# Its gain variation never gates the bed or interrupts the underlying stream.
ffmpeg -nostdin -v error -y -i "$scratch/source.wav" -filter_complex "
  [0:a]highpass=f=75:p=2,asplit=2[body][air];
  [body]lowpass=f=3200:p=2,pan=stereo|c0=0.83*c0+0.17*c1|c1=0.17*c0+0.83*c1[center];
  [air]highpass=f=2300:p=2,lowpass=f=5500:p=2,
    volume='0.14+0.026*sin(2*PI*t/113)+0.017*sin(2*PI*t/73.2)':eval=frame[airband];
  [center][airband]amix=inputs=2:normalize=0,volume=-2.0dB,
    afade=t=in:d=6,afade=t=out:st=354:d=6[out]
" -map '[out]' -ar 48000 -c:a pcm_s24le white.wav
