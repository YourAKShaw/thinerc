# ✦ thine's visual language

thine should feel as calm and polished as Claude Code's CLI — a recognisable mark, a consistent output grammar, motion that tells you something is happening — while staying scriptable, accessible and fast. This page is the style guide for anyone adding output. The decision record is [ADR-0008](../adr/0008-terminal-ux-and-animation.md).

## The mark

thine's logo is a **four-pointed star, `✦`**. It appears in the wordmark, the welcome box, `whatsup`, `doctor` and the version line — and it *moves*:

```text
·  ✧  ✦  ✶  ✷  ✸  ✹  ✸  ✷  ✶  ✦  ✧      ← StarFrames, 12 fps
```

Beside every long operation the star twinkles through those frames while its colour walks along the theme gradient — the counterpart of Claude Code's pulsing asterisk. The message next to it carries a moving highlight (a "shimmer" sweeping across the letters like light on foil) and an elapsed timer:

```text
✶ Scanning caches and projects… (7/22 · 3s · ctrl+c to cancel)
```

## The splash

Bare `thine` (and `thine hello`) plays a ~1 s sequence on stderr:

1. **Star burst** — the star blooms from `·` to `✸` in the middle of the logo row.
2. **Wipe-in** — the ANSI-Shadow `THINE` wordmark opens from the centre outwards, painted with the theme gradient, with a bright leading edge.
3. **Shimmer** — one diagonal highlight crosses the letters.
4. **Typewriter** — the tagline types itself out.

Then a rounded welcome box (user, cwd, OS, shell), a live pulse (dev ports, free disk) and a typewritten tip. Narrow terminals get a one-line `✦ t h i n e`. `ui.splash = false` skips it.

## Output grammar

| Element | Look | Use |
|---|---|---|
| Step | `⏺ title` (marker coloured by meaning) | Every result headline |
| Result | `  ⎿  detail` | Detail hanging under a step |
| Continuation | five-space indent | Further detail lines |
| Error | `✖ what — why → fix` (stderr) | Every failure, always with a next step |
| Note | dim italic (stderr) | Hints, suppressed by `--quiet` |
| Badge | `▕ dev ▏` (colour pill) | Tiers and risk levels |
| Gauge | eighth-block bar coloured green → amber → red | Memory, disk, grace periods |
| Rule | `── label ────` | Section separators |

Rules of thumb: **lead with the outcome** (`⏺ :3000 and :3001 are free (0.4s)`), keep detail dim, and never print escape codes to a pipe.

## Colour

- Semantic roles, never raw colours: `Success` (done), `Warning` (attention), `Error` (refused/destructive), `Info`, `Accent` (commands, highlights), `Muted` (metadata), `Subtle` (borders), plus a 4-stop `Brand` gradient.
- Truecolor with automatic fallback to xterm-256 and the 16 ANSI colours (so the terminal's own theme picks the hue).
- Nine palettes (`thine theme ls`). Themes change hues only — meaning never changes.

## Type

Metadata and hints are **italic** (`Muted` = dim + italic). In a font with a cursive italic — Victor Mono, Cascadia Code, Monaspace Radon, Fira Code iScript — they read like margin notes. Commands the user can type are **bold accent**.

## Icons

Three sets with identical meanings: `emoji` (default, no special font), `nerd` (Nerd Font v3 glyphs) and `ascii` (`[ok]`, `[x]`…, used automatically by `--plain`).

## Motion

| Effect | Where |
|---|---|
| Star spinner + shimmer | Every operation that can take > 100 ms |
| Draining grace bar | Between SIGTERM and SIGKILL in `kill` |
| Fading pulse `◉ ◎ ○ ◌ ·` | When a port is freed |
| Gauge fill | `freeup` memory bar |
| Staggered reveal | `whatsup` sections, `doctor` checks |
| Gradient wave | `theme set` confirmation |
| Confetti burst | Cleanup ≥ 1 GiB, freeup ≥ 512 MiB, setup, nuke-nm success |
| Rain, fireworks | `thine fx` |

**Motion rules:** stderr only; 12 fps or slower; never at shell startup; the cursor is hidden only while animating and always restored (even on Ctrl-C); everything turns off for `--plain`, `--json`, `--no-anim`, `NO_COLOR`, `THINE_NO_ANIM`, `TERM=dumb`, CI and non-TTY streams.

## Untrusted text

Process names, command lines, paths and agent payloads go through `ui.Sanitize` before they're printed: escape sequences and control characters are stripped and length is bounded, so a process named `\x1b]0;pwned\x07` can't retitle your terminal.
