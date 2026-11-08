#!/usr/bin/env bash
# Build one of the published archives and its checksum from a source checkout.
set -euo pipefail
if [[ $# != 4 ]]; then
  echo 'Usage: package-release.sh <vTAG> <os> <arch> <output-directory>' >&2
  exit 2
fi
tag=$1
export CGO_ENABLED=0 GOOS="$2" GOARCH="$3"
dest=$(realpath -m "$4")
mkdir -p "$dest"
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
name="entrain-$GOOS-$GOARCH"
dir="$tmp/$name"
mkdir -p "$dir/examples"
ext=""
if [[ $GOOS == windows ]]; then ext=".exe"; fi
for tool in entrain beatscope noisemaker; do
  go build -trimpath -ldflags "-X main.version=$tag" -o "$dir/$tool$ext" "./cmd/$tool"
done
cp LICENSE "$dir/"
cp docs/release-quickstart.md "$dir/README.md"
cp examples/{rain-bell.entrain,rain-bell-timeline.entrain,focus-40-pink.sbg,emdr-timbres.sbg,evolving-rain.entrain,wind-bells.entrain,surf-bells.entrain} "$dir/examples/"
tar --mtime="@$(git show -s --format=%ct HEAD)" --owner=0 --group=0 --numeric-owner --sort=name -czf "$dest/$name.tar.gz" -C "$tmp" "$name"
(cd "$dest" && sha256sum "$name.tar.gz" > "$name.tar.gz.sha256")
