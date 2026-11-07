# Task runner for the entrain repo. `just` alone lists recipes.

default:
    @just --list

# Serve the static site for review at http://localhost:{{port}}
site port="8931" *extra="":
    python3 -m http.server {{port}} --directory site {{extra}}

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
