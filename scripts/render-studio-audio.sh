#!/usr/bin/env bash
# Rebuild the shared CLI/Studio loops from the six-minute noise master recipes
# and the same shaped cue voices used by the listening examples.
set -euo pipefail
export LC_ALL=C
repo=$(cd "$(dirname "$0")/.." && pwd)
masters=${1:-"$repo/output/noise-backgrounds"}
dest=${2:-"$repo/site/audio"}
mkdir -p "$masters" "$dest"
masters=$(cd "$masters" && pwd)
dest=$(cd "$dest" && pwd)
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
ff=(ffmpeg -nostdin -v error -y)

for color in pink white rain; do
  if [[ ! -f "$masters/$color.flac" ]]; then
    "$repo/examples/noise-backgrounds/render.sh" "$color" "$masters"
  fi
  # A 2 s equal-power overlap joins the material after this excerpt to
  # its head. The first sample follows the last naturally on every repeat.
  "${ff[@]}" -ss 75 -i "$masters/$color.flac" -filter_complex "
    [0:a]asplit=3[head][body][tail];
    [head]atrim=end=2,asetpts=PTS-STARTPTS[h];
    [body]atrim=start=2:end=60,asetpts=PTS-STARTPTS[b];
    [tail]atrim=start=60:end=62,asetpts=PTS-STARTPTS[t];
    [t][h]acrossfade=d=2:c1=qsin:c2=qsin[wrap];
    [wrap][b]concat=n=2:v=0:a=1[out]
  " -map '[out]' -ar 44100 -sample_fmt s16 -c:a flac -frame_size 4608 -compression_level 8 \
    "$work/$color.flac"
done

(cd "$repo" && go build -o "$work/entrain" ./cmd/entrain)
cd "$work"
for shape in bell tone noise; do
  # Omit the example's pink voice so a cue can be layered over any bed.
  sed "s/shape=bell pink\/14/shape=$shape/" \
    "$repo/examples/bilateral-cues/soft-bell.sbg" > cue.sbg
  ./entrain cue.sbg -r 44100 -b 24 -o rendered.wav

  # Cut from a steady part on an exact left onset. Ten seconds holds eight
  # L→R→L cycles at 0.8 Hz. The trailing rest ends before the next left cue.
  spatial='pan=stereo|c0=0.96987*c0+0.24362*c1|c1=0.24362*c0+0.96987*c1'
  filters="atrim=start=5:end=15,asetpts=PTS-STARTPTS,$spatial"
  gain=$(ffmpeg -hide_banner -nostats -nostdin -i rendered.wav \
    -af "$filters,ebur128=peak=true" -f null - 2>&1 \
    | awk -v target=-27 -v ceiling=-6 -f "$repo/examples/master-gain.awk")
  "${ff[@]}" -i rendered.wav -af "$filters,volume=${gain}dB" \
    -ar 44100 -ac 2 -c:a pcm_s16le "cue-$shape.wav"
done

for name in pink.flac white.flac rain.flac cue-bell.wav cue-tone.wav cue-noise.wav; do
  mv "$work/$name" "$dest/$name"
done
