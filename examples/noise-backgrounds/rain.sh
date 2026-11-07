#!/usr/bin/env bash
set -euo pipefail

# Run in a scratch directory. Two independent, continuous stereo sources
# provide the body of the shower and its finer, brighter surface.
"${NOISEMAKER:?}" pink 6m -r 48000 -b 24 --seed 1803 --level 0.12 -o rain-pink.wav
"$NOISEMAKER" white 6m -r 48000 -b 24 --seed 4911 --level 0.12 -o rain-white.wav

# A slow drift in brightness moves across the warm bed. The fine texture is
# multiplied by smoothed random pressure in each channel, with a nonzero
# floor throughout. The shared render step supplies the final fade-out.
ffmpeg -hide_banner -loglevel error -y -i rain-pink.wav -i rain-white.wav \
  -filter_complex "
    [0:a]asplit=2[body][fine];
    [1:a]asplit=2[air][pressure];
    [body]highpass=f=125,lowpass=f=1450,
      pan=stereo|FL=0.84*FL+0.16*FR|FR=0.84*FR+0.16*FL,
      volume='1.18+0.08*sin(t/19.3)+0.06*sin(t/31.7)':eval=frame[warm];
    [air]highpass=f=1000,lowpass=f=4900,lowpass=f=7400,
      volume='0.76+0.11*sin(t/11.1+0.7*sin(t/29))+0.064*sin(t/6.8)':eval=frame[hiss];
    [fine]highpass=f=650,lowpass=f=2700[close];
    [pressure]lowpass=f=2.9,
      aeval='0.63+0.24*tanh(560*val(0))|0.63+0.24*tanh(560*val(1))'[density];
    [close][density]amultiply[grains];
    [warm][hiss][grains]amix=inputs=3:normalize=0,
      highpass=f=50,lowpass=f=6600,volume=-6dB,afade=t=in:d=6:curve=hsin[out]
  " -map '[out]' -c:a pcm_s24le rain.wav
