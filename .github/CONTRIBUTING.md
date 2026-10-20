<div align="center">

# 🤝 Contributing to .thinerc

**Help make the safest, most delightful verbs layer a terminal has ever had.**

[![PRs Welcome](https://img.shields.io/badge/PRs-Welcome-8B5CF6?style=flat-square)](https://github.com/YourAKShaw/thinerc/pulls)
[![Go 1.27](https://img.shields.io/badge/Go-1.27-334155?style=flat-square)](https://go.dev/)
[![Conventional Commits + gitmoji](https://img.shields.io/badge/Conventional%20Commits-%2B%20gitmoji-8B5CF6?style=flat-square)](https://www.conventionalcommits.org/en/v1.0.0/)
[![License: MIT](https://img.shields.io/badge/License-MIT-8B5CF6?style=flat-square)](../LICENSE)

[Project overview](../README.md) · [Architecture](../docs/architecture.md) · [Safety model](../docs/safety.md) · [Get help](SUPPORT.md) · [Security policy](SECURITY.md)

</div>

---

## Start with the user's problem

`thine` turns the chores developers hand-roll in `.zshrc` — freeing a stuck port, reclaiming disk, checking memory, sharing team commands — into one-word verbs that are **safe by default, undoable, cross-shell and fun to use**. Contributions should make that faster, clearer, safer, or more delightful.

Useful contributions include:

- A bug report with a minimal reproduction (the commands, the output, `thine doctor --json`).
- A cleanup category for a tool we miss — with its *own* clean command, a detector, a risk level and fixture tests.
- A parser fixture from your OS (`lsof -F`, `/proc/net/tcp`, `vm_stat`, `memory_pressure`) that breaks us.
- Clearer messages, hints, docs or animations that respect reduced motion.
- Tests for an edge case: hostile process names, symlink tricks, odd filesystems, WSL, macOS.

Read the [architecture overview](../docs/architecture.md) and the [safety model](../docs/safety.md) before touching anything that signals processes, deletes or moves files, runs `sudo`, touches the network, or decides trust.

## 🧭 Choose the right starting point

| Contribution | Where to begin |
| :--- | :--- |
| Small fix or docs correction | Open a pull request with the problem and how you verified the fix. |
| Bug | Search existing issues, then use the [bug report form](https://github.com/YourAKShaw/thinerc/issues/new/choose) with `thine doctor --json` output. |
| New verb, category or behaviour change | Open a [feature request](https://github.com/YourAKShaw/thinerc/issues/new/choose) first with the use case, risk level and trade-offs. |
| Security vulnerability | **Do not open an issue.** Follow the [security policy](SECURITY.md). |
| Usage question | Follow the [support guide](SUPPORT.md). |

## 🛠️ Set up

Requirements: **Go 1.27+**, **Git**, and optionally `bash`, `zsh` and `fish` (the shell-integration tests source the generated scripts in every shell that is installed).

```sh
git clone https://github.com/YourAKShaw/thinerc.git
cd thinerc
make build          # → bin/thine
./bin/thine         # the animated welcome screen
make check          # everything CI runs: fmt, vet, boundaries, invariants, tests, cross-compile
```

### Useful targets

| Command | What it does |
| :--- | :--- |
| `make check` | `lint` + `test` + `cross` — the same gates as CI |
| `make test` | Unit, integration and end-to-end CLI tests (a real TCP listener is spawned and killed) |
| `make race` | Tests with the race detector (needs cgo) |
| `make lint` | `gofmt`, `go vet`, [boundary checks](../scripts/check-boundaries.sh), [invariant tracing](../scripts/trace-invariants.sh) |
| `make cross` | Builds linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 into `dist/` |
| `make install` | Installs to `~/.local/bin` (no sudo) |
| `make demo` | Plays every animation |
| `go test -fuzz=FuzzGuard ./internal/cleanup` | Fuzz the cleanup path guard with random symlink trees |

Tests never touch your real home: they set `THINE_HOME` to a temporary directory. Never point a destructive test at a real directory.

## Make a focused change

1. Fork the repository and create a branch (`feat/…`, `fix/…`, `docs/…`).
2. Make the smallest coherent change that solves the problem, **with tests** — at the cheapest layer that can actually catch the failure. Don't mock away the OS boundary whose behaviour matters.
3. Run `make check`.
4. Update docs (`README.md`, `docs/`) and `CHANGELOG.md` (under **Unreleased**) for user-visible changes.
5. Commit following the convention below, and open a pull request using the template.

### 🛡️ Safety-critical changes

Any change to `internal/procs`, `internal/cleanup`, `internal/quarantine`, `internal/platform` (exec, sudo, locks), `internal/netfetch`, recipe execution or trust gets the **`safety`** label and must:

- keep every invariant **S1–S12** in [docs/safety.md](../docs/safety.md) true — `scripts/trace-invariants.sh` fails if one loses its test;
- add a negative test for the failure you are guarding against;
- answer these four questions in the PR body:
  1. What could this delete, move, signal or execute that it shouldn't?
  2. What happens if it's interrupted halfway, or run twice at once?
  3. What does an agent (or a malicious repository) get to do with it?
  4. How does the user see — and undo — what happened?

### Architectural fitness functions

These are enforced by `make lint` and CI, not just written down:

| Rule | Why |
| :--- | :--- |
| `net/http` only in `internal/netfetch` | No hidden network calls, no telemetry (S11) |
| `os/exec` only in `internal/platform` | Every subprocess is an argv array with a deadline (S5) |
| `sh -c` only in `internal/recipes` | Shell is explicit and labelled, parameters are positional |
| Every invariant S1–S12 has a `TestS<n>_…` | Safety claims stay tested |
| `thine init` prints nothing but shell code | Compatible with instant prompts; zero startup noise |

## ✍️ Commit messages

Every commit follows [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/) **with a [gitmoji](https://gitmoji.dev/) after the colon**, a body that explains *why*, and filled footers. Merge commits can keep Git's default message.

```text
<type>(<scope>): <gitmoji> <imperative summary, ≤ 72 chars>

<body: what changed and why — wrapped at ~72 columns, bullet lists welcome>

Breaking-Changes: none (<why it is compatible>)
Refs: <doc, ADR or #issue>
```

- **Types:** `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.
- **Scopes:** the area touched — `cli`, `ui`, `procs`, `ports`, `kill`, `freeup`, `cleanup`, `nm`, `recipes`, `shell`, `config`, `fonts`, `look`, `ai`, `doctor`, `docs`, `repo`, `deps`.
- **Common gitmoji:** ✨ feature · 🐛 fix · 📝 docs · ♻️ refactor · ⚡️ performance · ✅ tests · 🔒️ security · 👷 CI · 🔧 config · ⬆️ dependencies · 🎨 look & feel · 🚨 lint · 🎉 initial.
- **Breaking changes:** when a change *does* break something, use the spec's footer `BREAKING CHANGE: <what breaks and how to migrate>` (and `!` after the scope). Otherwise write `Breaking-Changes: none (…)` — a distinct token on purpose, so changelog tools never mistake "none" for a break.

Example:

```text
fix(kill): 🐛 keep --json output free of human status lines

`kill --json` printed "stopped" lines to stdout, corrupting the JSON
document scripts and agents parse. Stop-process output is now
suppressed in JSON mode; results live only in the document.

Breaking-Changes: none (output is now valid JSON, as documented)
Refs: docs/reference/json.md
```

CI checks every new commit against this convention. Run the same check before you push:

```sh
scripts/check-commits.sh origin/main..HEAD
```

## 📬 Open a reviewable pull request

Use the [pull request template](pull_request_template.md) to explain:

- **Problem and outcome:** what was wrong or missing, and what changes for the user.
- **Verification:** exact commands and results — and what you couldn't run (no macOS machine, no WSL…).
- **Risks:** safety, startup latency, compatibility, platform differences.

Keep the diff focused and the discussion kind. Address the change, not the contributor.
