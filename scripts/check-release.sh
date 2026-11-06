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
tar -C "$tmp" -xzf "$archive"
cd "$tmp/entrain-linux-amd64"
test "$(./entrain version)" = "entrain $tag"
test "$(./beatscope --version)" = "beatscope $tag"
test "$(./noisemaker --version)" = "noisemaker $tag"
./entrain mix rain --cue bell --cue-volume 65 --cue-interval-ms 850 --duration 20s --save-session my-rain.entrain
./entrain render my-rain.entrain --dry-run
./entrain render my-rain.entrain -o replay.wav
./entrain mix rain --cue bell --cue-volume 65 --cue-interval-ms 850 --duration 20s -o direct.wav
cmp direct.wav replay.wav
./entrain render examples/rain-bell.entrain -L 0:00:02 -o sample.mp3
./entrain render examples/rain-bell-timeline.entrain --dry-run
./entrain render examples/rain-bell-timeline.entrain -o timeline.flac
./entrain render examples/focus-40-pink.sbg -L 0:00:15 -o focus.wav
./beatscope focus.wav
./noisemaker pink 1s -o noise.wav
python3 - <<'PY'
import wave
from pathlib import Path
for path, seconds in [('replay.wav', 20), ('focus.wav', 15), ('noise.wav', 1)]:
    with wave.open(path) as audio:
        assert audio.getframerate() == 44100 and audio.getnchannels() == 2, path
        assert audio.getnframes() == seconds * 44100, path
        assert any(audio.readframes(audio.getnframes())), path
flac = Path('timeline.flac').read_bytes()
assert flac[:4] == b'fLaC' and (flac[4] & 127) == 0
assert int.from_bytes(flac[18:26], 'big') & ((1 << 36) - 1) == 360 * 44100
assert Path('sample.mp3').stat().st_size > 4096
PY
