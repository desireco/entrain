#!/usr/bin/env bash
# Prepare the Linux download linked by the local Studio on localhost.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p site/local-cli
tmp=$(mktemp site/local-cli/.entrain.XXXXXX)
trap 'rm -f "$tmp"' EXIT
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags '-X main.version=v0.2.0+local.session-editor' -o "$tmp" ./cmd/entrain
chmod 755 "$tmp"
mv "$tmp" site/local-cli/entrain-linux-amd64
