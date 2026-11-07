# UX Review — Studio vs Session Creator differentiation

Review of entrain.cc after the studio/editor changes: the two tool pages, the
handoff between them, and the naming that ties the site together. Evidence:
live screenshots (desktop 1440px + mobile 390px, served locally) and source
(`site/studio.js`, `site/studio.css`, `site/session-editor.js`,
`site/session-document.js`, `site/studio/index.html`,
`site/session-creator/index.html`).

## Headline finding

**Studio and Session Creator are the same screen.** The two HTML files are
300 lines each with a 54-line diff — title, heading, nav state, and exactly
two hidden elements:

```css
/* studio.css */
.studio[data-creator="true"] #creator-link,
.studio[data-creator="true"] #session-details { display: none; }
```

Same layout, bed picker, transport, source pane, `studio.js` and `studio.css`.
Clicking "Open Session Creator →" from Studio lands on what reads as the page
just left, minus one accordion. The two entries in the nav currently promise
two different things but deliver one.

## Scorecard (8 pillars)

| Pillar | Score | Verdict |
|---|---|---|
| Needs anticipation & friction removal | 3 | Strong persistence; the Studio→Creator handoff silently drops the auditioned sound |
| Transitional elegance | 3 | STANDBY→LIVE scope states, "Loading the selected sounds…"; pacing needs a runtime check |
| Strategic micro-delight | 3 | Beatscope scope is the delight; the download moment ends in a whisper |
| Animation restraint | 4 | Scroll reveals only; `prefers-reduced-motion` honored |
| Invisible consistency | 2 | Within pages excellent; across pages two identical consoles + naming drift |
| Intentional empty states | 4 | "No sections yet. The background plays alone." and scope STANDBY are genuinely good |
| Rewarding discovery | 3 | Excerpt preview and per-section "Preview entrance" reward exploration |
| Prescriptive error states | 5 | Best-in-class validation copy — do not regress |

## The differentiation contract

| | Studio (`/studio/`) | Session Creator (`/session-creator/`) |
|---|---|---|
| Job | **Hear it.** | **Build it.** |
| Arrives with | Your last sound | Studio's sound + duration (handed off, not reset) |
| Primary surface | Transport, live scope, bed picker | The score (.entrain text) + cue-section timeline |
| Binaural presets | Live here (3 chips) | Never shown |
| Ends with | "Make this a session →" | "Saved. Here's your render command." |
| Metaphor | Listening room | Workbench |

Naming: the artifact is a **session**; the editable text is the **score**.
Nav name stays "Session Creator" (URL, sitemap and social cards are shipped).
"Create a session." belongs to the Creator page exclusively; the homepage
section heading renames to "Sessions and examples." (homepage CTA keeps the
verb form — a button announcing the destination is correct).

## Work plan (in implementation order)

1. **Handoff fix** — `studio.js` forces `current = 'pink'` on the creator
   page (`if (creatorPage && SESSIONS[current]) current = 'pink'`), silently
   dropping the auditioned sound while the link says "Keep this sound and
   duration." Replace with: inherit `entrain.background` + `entrain.duration`;
   map binaural presets to their bed (`focus-40-pink`/`meditate-alpha` → pink,
   `sleep-delta` → pink with the note that brown renders offline).
2. **Creator grid flip** — under `data-creator="true"`, source pane becomes
   the primary column (`grid-template-areas: "source listen" "source
   settings"`); score textarea height cap 18→24 rows; mobile stacks score
   first. Studio's layout is unchanged.
3. **Cue-section timeline strip** — creator-only component: proportional bar
   of the session duration; sections as accent blocks, gaps marked
   "background alone"; click focuses the section row; playhead tracks preview
   position (hook the existing elapsed update loop). Empty state: "No cue
   sections yet — the background plays alone. Add a section to place bells,
   tones, or noise." This is the one component Studio never gets.
4. **Seed variation promoted** — on creator, "Sound variation" renders as an
   open field, not a collapsed `<details>`, when a generated bed is selected.
5. **Handoff chip + return path** — creator shows "Continuing pink noise ·
   6 min" with a reset; quiet "Audition more backgrounds in Studio →" link
   under the source actions (creator currently has no way back but the nav).
6. **Download completion state** — replace "Download started for …" (small
   inline span) with a confirm strip: "pink.entrain saved — render it:
   `./entrain render pink.entrain -o pink.wav`" (command already computed in
   `buildSource()`).
7. **Copy** — Studio sub: "Audition the sounds of the Entrain CLI, or open
   the Session Creator to build a render offline." Creator sub: "Choose a
   sound, shape the cue sections, and download the .entrain file to render
   with the CLI." Creator source caption: "This is the score of your session.
   Edit it directly, or shape it with the controls."
8. **Homepage** — section heading "Create a session." → "Sessions and
   examples." (`index.html`).

Not touching: URLs, nav labels, footer, social cards, sitemap, the audio
engine, validation copy, the draft/controls source model, design tokens.

## Strengths to preserve

- Validation copy (`session-editor.js`): "enter a start time such as 0s or
  1m15s", "starts before the previous section ends at 2m", "interval must be
  350–2000 whole milliseconds" — prescriptive, per-field, `aria-invalid` wired.
- Draft-vs-controls source model (`session-document.js`): edited text owns
  formatting; apply/replace-confirm with "Keep my text".
- One design-token system (warm paper, burnt orange, Space Grotesk/Plex).
- Persistence: every control survives reload via localStorage.

## Verification plan

Screenshot diff: Studio desktop/mobile unchanged except the one sub-copy
line; creator shows flipped grid + timeline in both cue modes + empty state;
handoff carries background/duration; `sleep-delta` note appears; download
strip renders; console clean. Audio behavior (WASM bed loads, playhead sync)
is a manual pass — headless browsers cannot exercise Web Audio autoplay
policies.
