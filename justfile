# Task runner for the entrain repo. `just` alone lists recipes.

default:
    @just --list

# Serve the static site for review; --open launches the browser
site *args:
    #!/usr/bin/env bash
    set -euo pipefail
    port=8931
    open=false
    pass=()
    for arg in {{args}}; do
      case $arg in
        "") ;;
        --open) open=true ;;
        -*) pass+=("$arg") ;;
        *) port=$arg ;;
      esac
    done
    python3 -m http.server "$port" --directory site "${pass[@]}" &
    server=$!
    trap 'kill "$server" 2>/dev/null || true' EXIT INT TERM
    if $open; then
      sleep 0.6
      xdg-open "http://localhost:$port" >/dev/null 2>&1 || echo "Could not open a browser — visit http://localhost:$port"
    fi
    wait "$server"

# Check whether rendered pages in site/ are stale against web/pages
site-check:
    python3 scripts/build-site.py --check

# Re-render site/ pages from web/pages
site-build:
    python3 scripts/build-site.py

# Rebuild the browser WASM synth into site/soundscapes
site-wasm:
    ./scripts/build-studio-soundscapes.sh

build:
    go build ./...

test:
    go test ./...

vet:
    go vet ./...
