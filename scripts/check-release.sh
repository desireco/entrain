#!/usr/bin/env bash
# Exercise the Linux x86-64 archive with the same layout a listener receives.
set -euo pipefail
if [[ $# != 2 ]]; then
  echo 'Usage: check-release.sh <vTAG> <linux-amd64.tar.gz>' >&2
  exit 2
fi
tag=$1
archive=$(realpath "$2")
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
(cd "$(dirname "$archive")" && sha256sum -c "$(basename "$archive").sha256")
tar --no-same-owner -C "$tmp" -xzf "$archive"
cd "$tmp/entrain-linux-amd64"
test "$(./entrain version)" = "entrain $tag"
test "$(./beatscope --version)" = "beatscope $tag"
test "$(./noisemaker --version)" = "noisemaker $tag"
./entrain mix rain --cue bell --cue-volume 65 --cue-interval-ms 850 --duration 20s --save-session my-rain.entrain
./entrain render my-rain.entrain --dry-run
./entrain render my-rain.entrain -o replay.wav
./entrain mix rain --cue bell --cue-volume 65 --cue-interval-ms 850 --duration 20s -o direct.wav
cmp direct.wav replay.wav
# Source downloaded from Studio uses these same procedural voices and fields.
for background in rain wind surf; do
  ./entrain mix "$background" --seed 6315 --cue bell --cue-volume 52.5 --cue-interval-ms 911 --duration 9s -f wav -b 24 --save-session "$background.entrain"
  ./entrain render "$background.entrain" -o "$background-full.wav"
  ./entrain render "$background.entrain" --start 1375ms -L 2500ms -o "$background-excerpt.wav"
done
./entrain mix rain-loop --duration 1s -o compatible-rain.wav
./entrain render examples/evolving-rain.entrain --dry-run
./entrain render examples/wind-bells.entrain --start 12s -L 10s -o wind-entrance.flac
./entrain render examples/surf-bells.entrain --start 12s -L 10s -o surf-entrance.flac
./entrain render examples/rain-bell.entrain -L 0:00:02 -o sample.mp3
./entrain render examples/rain-bell-timeline.entrain --dry-run
./entrain render examples/rain-bell-timeline.entrain -o timeline.flac
./entrain render examples/focus-40-pink.sbg -L 0:00:15 -o focus.wav
./beatscope focus.wav
./noisemaker pink 1s -o noise.wav
python3 - <<'PY'
import wave
from math import ceil
from pathlib import Path
for path, seconds in [('replay.wav', 20), ('focus.wav', 15), ('noise.wav', 1)]:
    with wave.open(path) as audio:
        assert audio.getframerate() == 44100 and audio.getnchannels() == 2, path
        assert audio.getnframes() == seconds * 44100, path
        assert any(audio.readframes(audio.getnframes())), path
for name in ('rain', 'wind', 'surf'):
    with wave.open(name + '-full.wav') as full, wave.open(name + '-excerpt.wav') as excerpt:
        assert full.getsampwidth() == excerpt.getsampwidth() == 3
        first, last = ceil(1.375 * 44100), ceil(3.875 * 44100)
        full.setpos(first)
        assert excerpt.getnframes() == last - first
        assert full.readframes(last - first) == excerpt.readframes(last - first)
for name in ('wind', 'surf'):
    data = Path(name + '-entrance.flac').read_bytes()
    assert data[:4] == b'fLaC'
    assert int.from_bytes(data[18:26], 'big') & ((1 << 36) - 1) == 10 * 44100
flac = Path('timeline.flac').read_bytes()
assert flac[:4] == b'fLaC' and (flac[4] & 127) == 0
assert int.from_bytes(flac[18:26], 'big') & ((1 << 36) - 1) == 360 * 44100
assert Path('sample.mp3').stat().st_size > 4096
PY
