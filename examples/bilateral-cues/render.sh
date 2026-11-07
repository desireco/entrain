#!/usr/bin/env bash
# Render three independent, level-matched cue previews with softer stereo placement.
set -euo pipefail
export LC_ALL=C
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
dest=${1:-"$repo/output/bilateral-cues"}
mkdir -p "$dest"
dest=$(cd "$dest" && pwd)
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
(cd "$repo" && go build -o "$work/entrain" ./cmd/entrain)
cd "$work"

for shape in bell tone noise; do
  sed "s/shape=bell pink/shape=$shape pink/" "$here/soft-bell.sbg" > "$shape.sbg"
  ./entrain render "$shape.sbg" -b 24 -o "$shape.wav"
  # A copy 12 dB down in the opposite ear keeps each cue easy to locate.
  width='pan=stereo|c0=0.96987*c0+0.24362*c1|c1=0.24362*c0+0.96987*c1'
  # Measure the stereo preview first, then apply one fixed trim; the natural
  # decay of each cue is preserved.
  gain=$(ffmpeg -hide_banner -nostats -nostdin -i "$shape.wav" \
    -af "$width,ebur128=peak=true" -f null - 2>&1 | awk -f "$here/../master-gain.awk")
  ffmpeg -v error -nostdin -y -i "$shape.wav" -af "$width,volume=${gain}dB" \
    -codec:a libmp3lame -b:a 320k -metadata title="Alternating $shape" "$shape.mp3"
done
mv bell.mp3 tone.mp3 noise.mp3 "$dest/"
