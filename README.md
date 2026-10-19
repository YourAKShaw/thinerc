<div align="center">

<img src="docs/assets/thine-wordmark.svg" alt="THINE" width="504">

# ✦ .thinerc

**One-word superpowers for your terminal — safe, animated, agent-aware, and impossible to regret.**

Free a stuck port, reclaim gigabytes of disk, read memory honestly, and turn your aliases into typed recipes — with previews, undo, a twinkling star spinner and a splash screen you'll actually enjoy. One static binary for Linux and macOS.

[![CI](https://img.shields.io/github/actions/workflow/status/YourAKShaw/.thinerc/ci.yml?branch=main&style=flat-square&label=CI)](https://github.com/YourAKShaw/.thinerc/actions/workflows/ci.yml)
[![CodeQL](https://img.shields.io/github/actions/workflow/status/YourAKShaw/.thinerc/codeql.yml?branch=main&style=flat-square&label=CodeQL)](https://github.com/YourAKShaw/.thinerc/actions/workflows/codeql.yml)
[![Go 1.27](https://img.shields.io/badge/Go-1.27-334155?style=flat-square)](https://go.dev/)
[![Linux · macOS](https://img.shields.io/badge/Linux%20%C2%B7%20macOS-supported-8B5CF6?style=flat-square)](#-install)
[![bash · zsh · fish](https://img.shields.io/badge/bash%20%C2%B7%20zsh%20%C2%B7%20fish-integrated-334155?style=flat-square)](docs/commands.md#shell-shortcuts)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-%2B%20gitmoji-8B5CF6?style=flat-square)](.github/CONTRIBUTING.md)
[![License: MIT](https://img.shields.io/badge/License-MIT-334155?style=flat-square)](LICENSE)

[Install](#-install) · [Quick tour](#-quick-tour) · [Power-user handbook](docs/guides/power-user.md) · [Safety](docs/safety.md) · [Docs](docs/README.md) · [Contribute](.github/CONTRIBUTING.md)

</div>

---

## Why thine?

Every developer's `.zshrc` grows the same helpers: `killport() { kill -9 $(lsof -t -i:$1); }`, a `freeup` alias that "frees" 2 GB of RAM, a long `rm -rf` cleanup one-liner. They work — until the day `kill -9` hits the wrong PID, the cleanup glob catches something you needed, or an AI agent runs them for you.

`thine` (the command behind **.thinerc**) keeps the one-word convenience and replaces the risk:

> Every destructive verb is **plan → preview → apply → log → undo**. Processes are stopped gracefully and identified by PID *and* start time. Your files go to a quarantine you can restore from. Nothing touches the network. And it looks gorgeous doing it.

## ⚙️ What it does

| Capability | What it means in practice |
| :--- | :--- |
| **Ports** | `whoport 3000` shows the process, command line, cwd and uptime. `kp 3000` / `killfb` stop it with SIGTERM → a visible grace bar → SIGKILL *only for dev processes*, stop the whole `pnpm dev` job, map Docker-published ports to `docker stop`, and exit 0 only when the port is really free — so `killfb && pnpm dev` chains. |
| **Disk** | `cleanup` scans 22 categories (npm, pnpm, yarn, pip, Go, Docker, Homebrew, Xcode, stale `node_modules`/`.next`/`target`, old installers…), shows a plan with honest totals, cleans caches with each tool's own command and **quarantines** your files — `thine undo` restores them for 7 days. |
| **Memory** | `freeup` shows *available* memory and pressure (not the "free" number cache-dropping inflates), groups consumers by app, and reclaims by stopping idle dev and language servers — then reports the measured change. |
| **Recipes** | Your aliases, grown up: typed parameters, risk labels, per-OS variants, previews for anything risky. `thine import` converts your `.bashrc` and flags the aliases that hide real programs. |
| **Dashboard** | `whatsup` — ports, memory, disk, Docker, running AI agents and git, in ~90 ms. |
| **Agents** | `thine notify` for Claude Code / Codex hooks, `thine ai doctor`, and an **agent mode** where risky verbs only produce plans a human applies with `thine apply <id>`. |
| **Look & feel** | A Claude-Code-inspired UI: animated ✦ splash, twinkling star spinner with a shimmer, nine themes (Catppuccin, Tokyo Night, Dracula, Nord…), cursive-italic hints, pinned Nerd Font installer, confetti, rain and fireworks. |
| **Zero cost** | A static Go binary: ≈ 2 ms cold start, ≈ 3 ms added to a new shell, **0 ms per command** — no prompt hooks. |

## 📥 Install

**Requirements:** Linux or macOS (x64 or arm64). bash, zsh or fish.

### Installer (verifies the SHA-256, no sudo)

```sh
curl -fsSL https://raw.githubusercontent.com/YourAKShaw/.thinerc/main/scripts/install.sh | sh
```

It downloads the release archive and `checksums.txt` from GitHub Releases, refuses on a checksum mismatch, verifies build provenance when the GitHub CLI is available, and installs to `~/.local/bin`. Read it first — it's short: [`scripts/install.sh`](scripts/install.sh).

### From source

```sh
git clone https://github.com/YourAKShaw/.thinerc.git
cd .thinerc
make install          # Go 1.27+ → ~/.local/bin/thine
```

### Wire it into your shell

```sh
thine setup           # shows a diff, backs up your rc file, adds one marked block at the end (+ completions)
exec $SHELL
thine                 # ✦ say hello
```

`thine setup --uninstall` removes everything, byte for byte.

<details>
<summary><strong>Verify a release archive by hand</strong></summary>

```sh
sha256sum --ignore-missing -c checksums.txt          # macOS: shasum -a 256 --ignore-missing -c checksums.txt
gh attestation verify thine_<version>_linux_amd64.tar.gz --repo YourAKShaw/.thinerc
```

</details>

## 🚀 Quick tour

```text
$ whoport 3000
⏺ :3000 node (4123) ▕dev▏
  ⎿  node /home/me/code/shop/node_modules/.bin/next dev
     ~/code/shop · me · up 2h14m · 127.0.0.1

$ killfb && pnpm dev
⏺ :3000 node (4123) ▕dev▏
  ⎿  next dev
     job scope · 2 processes: pnpm(4101) node(4123)
⏺ :3000 node (4123) stopped in 0.3s
⏺ :3000 and :3001 are free (0.4s)
…your dev server starts…

$ cleanup
⏺ 🧹 cleanup plan · 01M5ASRZTE… (dry run until you say yes)
  ▕regenerable▏   npm cache                                 8.0 GiB
                  ↳ npm cache clean --force
  ▕reversible▏    Stale node_modules ×20                    7.1 GiB
                  ↳ quarantine · undo for 7d
                    ~/code/old-landing                     1009 MiB idle 343d
                    …
──────────────────────────────────────────────────────────────────────────
  frees now 8.4 GiB · frees after retention 9.1 GiB
? Apply this plan? [y/N] y
🎉 Reclaimed 8.4 GiB now + 9.1 GiB in quarantine — your disk thanks you
  ⎿  restore anything with `thine undo 01M5ASRZTE` until Oct 27

$ freeup
⏺ 🧠 7.1 GiB total · 1.8 GiB available (25%)
  ⎿  ███████████████████████▌        75% used
     pressure 0.0% calm (PSI avg10) · swap 5.4 GiB / 7.4 GiB · page cache 1.1 GiB (reclaimable on demand)
⏺ Reclaimable by stopping (your dev and language servers)
      1.   612 MiB tsserver ×3
      2.   420 MiB node ×1 · next dev
? Stop which? numbers like 1,3 · all · none (none) all
⏺ available 1.8 GiB → 2.8 GiB (+1.0 GiB, measured)
```

More: `ports`, `whatsup`, `nuke-nm`, `thine doctor`, `thine fx all`. The **[power-user handbook](docs/guides/power-user.md)** walks through everything, level by level.

## 🛡️ The safety promise

- **Nothing changes without a plan you can see.** Without a terminal, risky verbs print the plan and exit 6 unless you pass `--apply`.
- **Processes are identified by PID + start time** (Linux pidfd), so a recycled PID is never signalled. PID 1, your shell and system daemons are never touched; databases, IDEs and other users' processes need confirmation.
- **Your files are quarantined, not deleted** — atomic same-filesystem moves, restorable with `thine undo`. Every item is re-checked immediately before acting; anything that changed is skipped.
- **Paths can't escape**: one guard canonicalises every path, refuses `~/.ssh`, keychains, system directories and `.git`, and never follows a symlink — fuzzed against random symlink trees.
- **Every apply is logged** write-ahead to `~/.local/state/thine/ops.jsonl` (credentials redacted) — even a crash leaves evidence in `thine history`.
- **No network, no telemetry.** The only download is the explicit `thine fonts install`, pinned and SHA-256-verified.
- **Repositories can't surprise you**: a project's `.thinerc.toml` is ignored until you `thine trust` it, and again after any edit.
- **Agents only plan.** Inside Claude Code, Codex or Gemini, risky verbs produce a plan for you to apply.

All twelve invariants have tests that CI traces — see the **[safety model](docs/safety.md)**.

## 🎨 Make it beautiful

```sh
thine theme ls                    # nine palettes with live swatches
thine theme set catppuccin-mocha
thine fonts install victor-mono   # cursive italics + ligatures + Nerd icons, pinned & verified
thine fonts test                  # check italics, ligatures, icons and colour depth
thine fx all                      # splash, spinner, wave, confetti, rain, fireworks
```

thine draws its hints in *italics*, so a font with a cursive italic (Victor Mono, Cascadia Code, Monaspace Radon, Fira Code iScript) makes them look hand-written. To theme your **whole** terminal, the [fonts and themes guide](docs/guides/fonts-and-themes.md) covers [Gogh](https://github.com/Gogh-Co/Gogh), [iTerm2-Color-Schemes](https://github.com/mbadolato/iTerm2-Color-Schemes) and [Catppuccin](https://github.com/catppuccin/catppuccin).

## 🧪 Recipes: aliases that grew up

```toml
# ~/.config/thine/config.toml
[recipes.db-reset]
description = "Drop, migrate and reseed the local database"
risk        = "destructive"           # asks you to type "run db-reset" first
cwd         = "project"
params      = [{ name = "env", type = "enum", values = ["dev", "test"], default = "dev" }]
steps = [
  { run = ["docker", "compose", "exec", "-T", "db", "dropdb", "--if-exists", "app_{{env}}"] },
  { run = ["pnpm", "db:migrate"] },
  { run = ["pnpm", "db:seed"] },
]
```

```sh
thine import --write          # your .bashrc aliases → recipes (with risk labels and warnings)
thine recipes starter --write # gpa, gpff, gbr, gprune, treen, cpwd, biggest…
thine run --param env=test db-reset
```

Parameters are validated and only ever substituted *inside* argv elements — never spliced into a shell. Share a project's recipes and port groups in a committed `.thinerc.toml`; teammates review and `thine trust` it once. See the [recipes cookbook](docs/guides/recipes-cookbook.md).

## 🤖 AI agents

```sh
thine ai doctor    # Claude Code, Codex, Gemini… versions, hooks, AGENTS.md / CLAUDE.md
thine ai hooks     # paste-ready hooks → "Claude Code · shop (feat/cart) · finished" on your desktop
```

thine never calls a model. It makes the agents you use easier to live with and safer to hand a terminal to — [AI agents guide](docs/guides/ai-agents.md).

## 🛠️ Commands

| Command | Purpose |
| :--- | :--- |
| `thine port` · `ports` · `kill` | Who's listening, every dev port, graceful job-aware stopping (`whoport`, `ports`, `kp`, `killport`, `killfb`) |
| `thine freeup` | Honest memory report and reclaim |
| `thine cleanup` · `apply` · `undo` · `history` · `trash` | Plan-first disk reclamation with quarantine and undo |
| `thine whatsup` | One-screen dashboard |
| `thine nm reset` | `nuke-nm`: reinstall `node_modules` with automatic rollback |
| `thine run` · `recipes` · `import` | Your own verbs |
| `thine theme` · `fonts` · `fx` · `hello` | Look and feel |
| `thine setup` · `init` · `doctor` · `bench` · `config` · `trust` | Setup and health |
| `thine notify` · `ai doctor` · `ai hooks` | Agent glue and notifications |

Every read verb takes `--json` (versioned schema); exit codes are stable (0 done · 4 partial · 5 refused · 6 needs approval). Full reference: [docs/commands.md](docs/commands.md).

## ⚡ Performance

Measured with `thine bench` on an 8 GB Linux laptop:

| Measurement | p50 | Budget |
| :--- | :---: | :---: |
| `thine --version` (cold start) | 2.2 ms | ≤ 5 ms |
| `thine init zsh` | 2.2 ms | ≤ 5 ms |
| Added to a new zsh | ≈ 2.8 ms | ≤ 5 ms |
| Added per command | **0 ms** | 0 ms |
| `whatsup` (full dashboard) | ≈ 90 ms | ≤ 300 ms |

Run `thine bench` to measure yours.

## 🤝 Plays well with

thine is a verbs layer — it deliberately doesn't rebuild what great tools already do. `thine doctor` suggests them: [Starship](https://starship.rs) (prompt) · [Atuin](https://atuin.sh) (history) · [zoxide](https://github.com/ajeetdsouza/zoxide) · [fzf](https://github.com/junegunn/fzf) · [eza](https://eza.rocks) · [bat](https://github.com/sharkdp/bat) · [fd](https://github.com/sharkdp/fd) · [mise](https://mise.jdx.dev).

## 📂 Inside the repository

```text
.thinerc/
├── cmd/thine/            # Entry point
├── internal/
│   ├── cli/              # Every verb's UX, flags, exit codes, JSON
│   ├── ui/               # Themes, gradients, star spinner, splash, effects, Sanitize
│   ├── procs/ ports/ mem/   # Linux (/proc, pidfd) and macOS (ps, lsof) adapters
│   ├── cleanup/ quarantine/ oplog/   # Plan, guard, quarantine, undo, write-ahead log
│   ├── recipes/ config/  # Typed recipes, import, strict TOML, trust
│   ├── shellinit/        # bash/zsh/fish init, setup, collision guard
│   ├── fonts/ netfetch/  # Pinned Nerd Fonts; the only network code
│   └── platform/         # Paths, argv-only exec, sudo registry, locks, detection
├── docs/                 # Handbook, guides, reference, safety model, ADRs
├── scripts/              # install.sh, boundary and invariant checks
└── .github/              # CI, CodeQL, release, Dependabot, issue/PR templates, community files
```

## 🧑‍💻 Development

```sh
git clone https://github.com/YourAKShaw/.thinerc.git && cd .thinerc
make build && ./bin/thine     # build and say hello
make check                    # gofmt, vet, boundaries, S1–S12 invariant tracing, tests, cross-compile
make demo                     # every animation
```

Tests run against throwaway homes (`THINE_HOME`), source the generated scripts in real shells, and spawn a real TCP listener to kill. See [CONTRIBUTING.md](.github/CONTRIBUTING.md), [the architecture](docs/architecture.md) and [the decision records](docs/adr/README.md).

---

## 🤝 Contributing, support and security

Contributions are welcome — start with the [contribution guide](.github/CONTRIBUTING.md), use the [issue forms](https://github.com/YourAKShaw/.thinerc/issues/new/choose), or read the [support guide](.github/SUPPORT.md). If thine ever deletes, moves or kills something it shouldn't have, that's a release blocker — please report it. Vulnerabilities go through the private [security policy](.github/SECURITY.md).

## License

Distributed under the **MIT License**. See [LICENSE](LICENSE).
