#!/usr/bin/env bash
# Build one background or all three. Usage: render.sh [all|pink|white|rain] [directory]
set -euo pipefail
export LC_ALL=C
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
case ${1:-all} in
  all) tracks=(pink white rain) ;;
  pink|white|rain) tracks=("$1") ;;
  *) printf 'Choose all, pink, white, or rain.\n' >&2; exit 2 ;;
esac
dest=${2:-"$repo/output/noise-backgrounds"}
mkdir -p "$dest"
dest=$(cd "$dest" && pwd)
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
export ENTRAIN="$work/entrain" NOISEMAKER="$work/noisemaker"
(cd "$repo" && go build -o "$ENTRAIN" ./cmd/entrain && go build -o "$NOISEMAKER" ./cmd/noisemaker)
ff=(ffmpeg -nostdin -v error -y)

for track in "${tracks[@]}"; do
  mkdir "$work/$track"
  cd "$work/$track"
  bash "$here/$track.sh"
  mv "$track.wav" bed.wav
  cp "$here/background.sbg" .
  "$ENTRAIN" background.sbg -r 44100 -b 24 -o mix.wav

  # Match whole-track loudness with a single gain. Clamp gain if needed so
  # true peak stays at or below -3 dBTP without compressing the sound.
  gain=$(ffmpeg -hide_banner -nostats -nostdin -i mix.wav \
    -af ebur128=peak=true -f null - 2>&1 | awk -f "$here/../master-gain.awk")
  case $track in
    pink) title="Pink Noise" ;;
    white) title="White Noise" ;;
    rain) title="Soft Rain" ;;
  esac
  "${ff[@]}" -i mix.wav -af "volume=${gain}dB" -c:a flac -sample_fmt s32 \
    -bits_per_raw_sample 24 -metadata title="$title" "$track.flac"
  "${ff[@]}" -i "$track.flac" -codec:a libmp3lame -b:a 320k "$track.mp3"
  # Extract the same span from the finished master. Only the excerpt's two
  # boundaries need tapers to avoid an abrupt start or stop.
  "${ff[@]}" -ss 60 -i "$track.flac" -t 30 \
    -af "afade=t=in:d=0.20,afade=t=out:st=29.7:d=0.30" \
    -codec:a libmp3lame -b:a 320k -metadata title="$title (30-second excerpt)" \
    "$track-preview.mp3"
  mv "$track.flac" "$track.mp3" "$track-preview.mp3" "$dest/"
  printf 'Wrote %s to %s\n' "$title" "$dest"
done
