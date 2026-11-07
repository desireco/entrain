#!/usr/bin/env bash
# Build the offline browser DSP from the exact Go synth used by entrain mix.
set -euo pipefail
cd "$(dirname "$0")/.."
dest=site/soundscapes
goroot=$(go env GOROOT)
mkdir -p "$dest"
GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags='-s -w' -o "$dest/synth.wasm" ./web/soundscape-wasm
chmod 0644 "$dest/synth.wasm"
cp "$goroot/lib/wasm/wasm_exec.js" "$dest/wasm_exec.js"
cp "$goroot/LICENSE" "$dest/GO-LICENSE.txt"
