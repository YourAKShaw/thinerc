# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html). While the version is `0.x`, minor releases may contain breaking changes; they are listed under **Changed** with migration notes. JSON output carries its own `schema_version`.

## [Unreleased]

## [0.1.0] - 2026-10-20

First version of `thine`, the command behind **.thinerc**.

### Added

- **Ports & processes:** `thine port` (`whoport`), `thine ports` and `thine kill` (`kp`, `killport`, `killfb`, `kill<group>`). Graceful SIGTERM → grace period → SIGKILL for dev processes only; job scope for runner + child; PID + start-time identity with Linux pidfd; protected/confirm/dev tiers; Docker-published ports mapped to `docker stop`; `--sudo` for other users' processes; port verification so `killfb && pnpm dev` chains.
- **Disk:** `thine cleanup` with a 22-category catalog (package-manager caches, Go, Docker, Homebrew, Xcode, Playwright, Linux thumbnails, stale `node_modules`/`.next`/`target`/`.venv`, old installers, opt-in apt/journald/dnf), honest totals (frees now / after retention / not reclaimable), quarantine with `thine undo`, `thine history`, `thine trash ls|purge`, and saved plans applied with `thine apply <plan-id>`.
- **Memory:** `thine freeup` — available memory, PSI / memory_pressure, swap, PSS-grouped consumers, stopping idle dev and language servers with a measured result, and an explained, opt-in `--drop-caches`.
- **Dashboard:** `thine whatsup` — ports, memory, disk, Docker, running AI agents and git.
- **node_modules:** `thine nm reset` (`nuke-nm`) with package-manager detection and automatic rollback when the install fails.
- **Recipes:** typed, risk-labelled user verbs with validated parameters, argv-only substitution, positional arguments for shell recipes, per-OS overrides and `continue_on_error`; `thine run`, `thine recipes [ls|show|starter]`, and `thine import` to convert `.bashrc`/`.zshrc` aliases.
- **Configuration:** strict TOML (`~/.config/thine/config.toml`) with `thine config path|init|validate|edit|show`; project `.thinerc.toml` honoured only after `thine trust` (SHA-256 pinned).
- **Shell integration:** `thine init bash|zsh|fish` (silent, collision-guarded, no hooks) and idempotent `thine setup` with backup, completions and byte-identical `--uninstall`.
- **Look & feel:** a Claude-Code-inspired UI — animated ✦ splash and welcome box, twinkling star spinner with a shimmer, `⏺`/`⎿` output grammar, nine themes (`thine theme ls|preview|set|terminal`), a pinned and SHA-256-verified Nerd Fonts installer (`thine fonts install`), `thine fonts test`, and `thine fx` (confetti, rain, fireworks, wave).
- **Agents:** `thine notify` for Claude Code / Codex hooks and scripts, `thine ai doctor`, `thine ai hooks`, and agent mode (risky verbs only plan).
- **Health:** `thine doctor` (including your own definitions that shadow shortcuts, with file:line) and `thine bench`.
- **Safety:** twelve invariants (S1–S12) with traced tests, write-ahead operations log with credential redaction, exclusive apply lock, path guard fuzzing, no network outside the explicit font installer, no telemetry.
- **Project:** CI on Linux (x64, arm64) and macOS (arm64, Intel) with the race detector, fuzz smoke, staticcheck, govulncheck and CodeQL; a release workflow with deterministic archives, checksums and build-provenance attestations; `scripts/install.sh`; documentation with a power-user handbook, guides, references and eight ADRs.

[Unreleased]: https://github.com/YourAKShaw/thinerc/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/YourAKShaw/thinerc/releases/tag/v0.1.0