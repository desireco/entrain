#!/bin/bash
# Cross-engine validation: render sessions with entrain AND the reference
# SBaGen binary, detect the binaural beats in both outputs with beatscope,
# and compare. This is the project's behavioral-comparison methodology
# (docs/migration-plan.md: clean-room ground rules).
#
# Usage:
#   scripts/validate-vs-sbagen.sh [session.sbg ...]
#
# Environment:
#   SBAGEN    path to a sbagen binary (default: ../sbagen/sbagen-x86_64
#             relative to this repo — the workspace's vendored copy)
#   LENGTH    render length per session (default 00:00:30)
#
# Requires: entrain and beatscope on PATH (go build ./cmd/...), python3.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO=$PWD
SBAGEN=${SBAGEN:-../sbagen/sbagen-x86_64}
LENGTH=${LENGTH:-00:00:30}
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

if [ ! -x "$SBAGEN" ]; then
	echo "validate: sbagen binary not found at $SBAGEN (set SBAGEN=path)" >&2
	exit 2
fi
SBAGEN=$(cd "$(dirname "$SBAGEN")" && pwd)/$(basename "$SBAGEN") # absolutize: sessions run from their own dirs
command -v entrain >/dev/null || { echo "validate: build first: go build ./cmd/..." >&2; exit 2; }
command -v beatscope >/dev/null || { echo "validate: build first: go build ./cmd/..." >&2; exit 2; }

sessions=("$@")
if [ ${#sessions[@]} -eq 0 ]; then
	sessions=("$REPO"/../sessions/focus-40-24-pink.sbg)
fi

compare() {
	# compare NAME ENTRAIN_JSON SBAGEN_JSON → MATCH/DIFFER line
	NAME="$1" A="$2" B="$3" python3 -c '
import json, os, sys

def parse(doc):
    return {(round(b["Carrier"]), round(b["Beat"])): b["Confidence"] for b in doc["beats"] if b["Confidence"] >= 0.5}

a = parse(json.loads(os.environ["A"]))
b = parse(json.loads(os.environ["B"]))
name = os.environ["NAME"]

matched = sorted(set(a) & set(b))
diffs = [("entrain-only", k) for k in sorted(set(a) - set(b))] + \
        [("sbagen-only", k) for k in sorted(set(b) - set(a))]
ok = not diffs
verdict = "MATCH" if ok else "DIFFER"
detail = " ".join(f"{c}Hz@{beat}Hz-beat" for c, beat in matched)
extra = "" if ok else f"  diffs: {diffs}"
print(f"{name:38} {verdict:8} {len(matched)} pair(s): {detail}{extra}")
sys.exit(0 if ok else 1)
'
}

status=0
for s in "${sessions[@]}"; do
	name=$(basename "$s" .sbg)
	sdir=$(dirname "$s")
	# Run both engines from the session's directory so in-file -m paths
	# resolve the same way sbagen users expect.
	if ! (cd "$sdir" && entrain render "$s" -L "$LENGTH" -o "$TMP/$name.entrain.wav" >/dev/null 2>&1); then
		echo "validate: entrain failed on $s" >&2
		status=1
		continue
	fi
	if ! (cd "$sdir" && "$SBAGEN" -Q -S -W -o "$TMP/$name.sbagen.wav" -L "$LENGTH" "$s" >/dev/null 2>&1); then
		# sbagen sessions with -m soundtracks often resolve paths from the
		# distribution root rather than the session's directory; retry there.
		if ! (cd "$sdir/.." && "$SBAGEN" -Q -S -W -o "$TMP/$name.sbagen.wav" -L "$LENGTH" "${s#$sdir/}" >/dev/null 2>&1); then
			echo "validate: sbagen failed on $s" >&2
			status=1
			continue
		fi
	fi

	entrain_json=$(beatscope --json "$TMP/$name.entrain.wav")
	sbagen_json=$(beatscope --json "$TMP/$name.sbagen.wav")
	compare "$name" "$entrain_json" "$sbagen_json" || status=1
done

exit $status
