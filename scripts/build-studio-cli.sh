#!/usr/bin/env bash
# Build the Linux Studio download and record the source needed to reproduce it.
set -euo pipefail
cd "$(dirname "$0")/.."
revision=$(git rev-parse HEAD)
version="site-${revision:0:12}"
if ! git diff --quiet HEAD -- cmd internal synth noise seq codec go.mod go.sum site/audio; then
  version="$version-dirty"
fi
mkdir -p site/local-cli
tmp=$(mktemp site/local-cli/.entrain.XXXXXX)
trap 'rm -f "$tmp"' EXIT
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false \
  -ldflags "-X main.version=$version" -o "$tmp" ./cmd/entrain
chmod 755 "$tmp"
mv "$tmp" site/local-cli/entrain-linux-amd64
(cd site/local-cli && sha256sum entrain-linux-amd64 > entrain-linux-amd64.sha256)
cat > site/local-cli/build-info.txt <<EOF
Entrain Studio CLI for Linux x86-64
Version: $version
Source commit: $revision
Compiler: $(go version)
License: MIT

Source and history:
https://github.com/desireco/entrain/tree/$revision

Build this version on Linux, macOS or Windows with Go:
https://go.dev/doc/install

git clone https://github.com/desireco/entrain.git
cd entrain
git checkout $revision
go build -trimpath -buildvcs=false -ldflags "-X main.version=$version" -o entrain ./cmd/entrain

On Windows use -o entrain.exe and run .\entrain.exe.

The downloaded executable runs offline. Save a Studio .entrain session in
its folder, then run:
chmod +x ./entrain
./entrain render my-session.entrain

Verify the downloaded file with the adjacent entrain-linux-amd64.sha256.
The file in the checksum is named entrain-linux-amd64; browsers may save
it as entrain. Rename it back before running sha256sum -c.
EOF
