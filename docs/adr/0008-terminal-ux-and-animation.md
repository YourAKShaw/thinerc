# ADR-0008: A Claude-Code-inspired visual language with honest, opt-out animation

Status: accepted
Date: 2026-10-19
Owner / decision authority: maintainer (@YourAKShaw)
Scope / related issue: colours, fonts, icons, animation and output grammar

## Context and constraints

The owner asked for a CLI that looks and feels like Claude Code — a logo on launch, an animated mark beside long operations — with italics, colour themes and "as many animations as possible". At the same time, output must stay scriptable (`--json`), accessible (`NO_COLOR`, screen readers) and must never slow shell startup.

## Decision and rationale

- **Grammar:** `⏺` lead lines and `⎿` result lines (Claude Code's layout); errors as `✖ what — why → fix`; data on stdout, animation and prompts on stderr.
- **Mark:** thine's logo is a star (`✦`). It twinkles `· ✧ ✦ ✶ ✷ ✸ ✹` along the theme gradient as the spinner beside every long operation, with a shimmer sweeping across the message and an elapsed timer — the counterpart of Claude Code's pulsing asterisk. Bare `thine` plays a splash: a star burst opens into the ANSI-Shadow `THINE` wordmark, one shimmer crosses it, the tagline types itself.
- **Colour:** truecolor with automatic 256/16 fallback; nine palettes (Thine Aurora, Catppuccin Mocha/Latte, Tokyo Night, Dracula, Nord, Gruvbox, Rosé Pine, Synthwave '84). Themes change hues, never meanings.
- **Fonts:** metadata and hints are *italic*, so a font with a cursive italic (Victor Mono, Cascadia Code, Fira Code iScript, Monaspace Radon) makes them look hand-written; `thine fonts` installs pinned Nerd Fonts and tells each terminal how to use them.
- **Animation rules:** 12 fps, stderr only, never at shell startup, and off for `--plain`, `--json`, `--no-anim`, `NO_COLOR`, `THINE_NO_ANIM`, `TERM=dumb`, CI and non-TTY streams. The cursor is always restored, even on Ctrl-C.

## Consequences and risk ownership

Benefits: a distinctive, delightful CLI that still pipes cleanly into `jq`.
Costs / limitations: the splash costs ~1 s on bare `thine` (disable with `ui.splash = false`); emoji widths vary between terminals.

## Validation and acceptance

| Claim / acceptance criterion | Method or exact command | Evidence / state | Limitation / owner |
|---|---|---|---|
| Animations run without panics under a PTY | `script -qec "thine fx all"` | PASS (executed) | Visual review is manual |
| No escape codes when piped | `TestInitIsSilentScript`, JSON tests | PASS (executed) | — |
| Untrusted text can't inject escapes | `TestS8_SanitizeStripsEscapesAndControls` | PASS (executed) | — |

## Revisit when

Users report motion sensitivity issues beyond the existing opt-outs, or terminals standardise richer graphics (sixel/kitty graphics) worth supporting.
