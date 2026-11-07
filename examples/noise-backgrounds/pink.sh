#!/usr/bin/env bash
# A narrow, warm pink center and two wide, slowly changing bands of pink air.
set -euo pipefail
"$NOISEMAKER" pink 6m -r 48000 -b 24 --level 0.125 --seed 207 -o pink-source.wav
ffmpeg -nostdin -v error -y -i pink-source.wav -filter_complex "
  [0:a]asplit=2[base][air];
  [base]highpass=f=48,lowpass=f=580,
    pan=stereo|c0=0.77*c0+0.23*c1|c1=0.23*c0+0.77*c1,
    volume='0.86+0.035*sin(2*PI*t/73)+0.018*sin(2*PI*t/43+1)':eval=frame[body];
  [air]highpass=f=420,lowpass=f=8000,
    volume='0.57+0.055*sin(2*PI*t/53+1)+0.035*sin(2*PI*t/97)':eval=frame[top];
  [body][top]amix=inputs=2:normalize=0[mix]" \
  -map '[mix]' -c:a pcm_s24le pink.wav
