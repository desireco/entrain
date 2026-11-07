# Static pages

Edit the page partials here and run `python3 scripts/build-site.py` from the
Go module root. The shared shell supplies navigation and metadata. Studio and
Session Creator use the same builder markup and `site/studio.js`, with the
active page identified by its body’s `data-page` attribute. The local web
server and static host both serve the generated `site/` directory directly.

Use `python3 scripts/build-site.py --check` to check the generated documents
and downloadable examples against their sources.
