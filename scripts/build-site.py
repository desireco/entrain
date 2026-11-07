#!/usr/bin/env python3
"""Build the portable static site from its shared page shell and session panel."""
import argparse
import re
from pathlib import Path
from html import escape

ROOT = Path(__file__).resolve().parents[1]
PAGES = ROOT / "web/pages"
SITE = ROOT / "site"
NAV = {
    "home": "Home", "studio": "Studio",
    "listening-guide": "Listening Guide", "handbook": "Handbook",
}
DESC = {
    "home": "Binaural beats that guide your brain toward sleep, calm and focus — write sessions as plain text and render them to stereo on your machine with the Entrain CLI.",
    "studio": "Audition five backgrounds, mix alternating cue sections, and download a native .entrain session to render locally.",
    "listening-guide": "Compare the backgrounds, stereo cues and binaural presets by ear.",
    "handbook": "Get the Entrain CLI and render an editable session to WAV, FLAC or MP3.",
}
EXAMPLES = ["evolving-rain", "wind-bells", "surf-bells", "rain-bell-timeline"]


def route(page):
    return "./" if page == "home" else page + "/"


def link(fragment, page):
    if fragment in ("top", "sessions", "engine", "faq", "session-examples"):
        return "./" if fragment == "top" else "./#" + fragment
    if fragment in NAV:
        return route(fragment)
    return route(page) + "#" + fragment


def render(page):
    builder = page == "studio"
    partial = "builder" if builder else page
    content = (PAGES / (partial + ".html")).read_text()
    scripts = ["session-editor", "session-document", "soundscapes/player", "studio"] if builder else {
        "home": ["home"], "handbook": ["handbook"], "listening-guide": []
    }[page]
    title = "Entrain — local soundscapes and binaural audio" if page == "home" else NAV[page] + " · Entrain"
    page_head = '<link rel="stylesheet" href="studio.css">' if builder else ""
    if page == "home":
        page_head = '<script src="legacy-links.js"></script>'
    nav = []
    for key, label in NAV.items():
        active = ' class="active" aria-current="page"' if key == page else ""
        nav.append(f'<a href="{route(key)}"{active}>{label}</a>')
    values = {
        "BASE": "./" if page == "home" else "../",
        "TITLE": escape(title), "DESCRIPTION": escape(DESC[page]),
        "CANONICAL": "https://entrain.cc/" + ("" if page == "home" else route(page)),
        "PAGE": page, "NAV": "\n      ".join(nav), "CONTENT": content,
        "SCHEMA": ('<script type="application/ld+json">\n' + (PAGES / "software.json").read_text()
                   + '</script>') if page == "home" else "",
        "PAGE_HEAD": page_head,
        "SCRIPTS": "\n".join(f'<script src="{script}.js"></script>' for script in scripts),
    }
    shell = (PAGES / "shell.html").read_text()
    html = re.sub(r"{{(\w+)}}", lambda m: values[m[1]], shell)
    return re.sub(r'href="#([^"]+)"', lambda m: f'href="{link(m[1], page)}"', html)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="report stale static files without writing")
    args = parser.parse_args()
    files = {SITE / route(page) / "index.html": render(page) for page in NAV}
    files[SITE / "sitemap.xml"] = ('<?xml version="1.0" encoding="UTF-8"?>\n'
        '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n' +
        ''.join(f'  <url><loc>https://entrain.cc/{"" if key == "home" else route(key)}</loc></url>\n'
                for key in NAV) + '</urlset>\n')
    for name in EXAMPLES:
        files[SITE / "examples" / (name + ".entrain")] = (ROOT / "examples" / (name + ".entrain")).read_text()
    for name in ("cli-guide.md", "listening-guide.md", "sequence-format.md"):
        files[SITE / "docs" / name] = (ROOT / "docs" / name).read_text()
    stale = []
    for path, content in files.items():
        if not path.is_symlink() and path.exists() and path.read_text() == content:
            continue
        if args.check:
            stale.append(str(path.relative_to(ROOT)))
        else:
            path.parent.mkdir(parents=True, exist_ok=True)
            if path.is_symlink():
                path.unlink()
            path.write_text(content)
    if stale:
        raise SystemExit("Rebuild with python3 scripts/build-site.py: " + ", ".join(stale))


if __name__ == "__main__":
    main()
