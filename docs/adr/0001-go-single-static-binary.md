# ADR-0001: One static Go binary that emits shell glue

Status: accepted
Date: 2026-10-19
Owner / decision authority: maintainer (@YourAKShaw)
Scope / related issue: implementation language, runtime and distribution shape of `thine`

## Context and constraints

`thine` sits on the hot path twice: every interactive shell evaluates `thine init` at startup, and developers fire one-word verbs (`kp`, `killfb`, `whatsup`) dozens of times a day. It also performs R3 operations — signalling processes, deleting files, running `sudo` — so correctness and testability matter as much as speed. It must run on Linux first and macOS from the same codebase. The owner's strongest language is TypeScript; the owner asked for "Rust or maybe Go — you decide".

| Driver | Known / estimated / unknown | Constraint or evidence |
|---|---|---|
| Startup budget | Known | zsh-bench: command lag < 10 ms "feels like 0"; target ≤ 5 ms for `thine init` |
| Process start cost | Measured by third parties, re-measured here | Node ≈ 40 ms per process; compiled Go ≈ 2 ms |
| OS primitives without cgo | Known need | pidfd (Linux), sysctl/statfs, flock — all in `golang.org/x/sys` |
| Cross-compilation | Known need | linux/darwin × amd64/arm64 from one machine, no native toolchains |
| Team | Known | One maintainer; learning cost matters |

Non-goals: Windows-native support in v0.1 (WSL works through the Linux build); a plugin runtime.

## Options and trade-offs

| Option | Correctness/security | Complexity/operations | Cost/time | Compatibility/reversibility |
|---|---|---|---|---|
| **Go, `CGO_ENABLED=0`** | Strong typing, explicit errors, race detector and fuzzing in the standard library | Gentle ramp-up for a TypeScript engineer; trivial cross-compiles | Low | Static binaries, ~10 MB |
| Rust | Strongest guarantees; Starship/zoxide/atuin precedent | Steeper ramp-up; cross-compiling macOS from Linux is harder | Medium–high | Equally good artifacts |
| TypeScript on Node | Good with strict types | Fastest to write | ~40 ms per invocation breaks the hot-path budget; npm supply-chain exposure (`kill-port` shipped malware in Shai-Hulud 2.0) | Needs a Node runtime |
| Pure shell framework | Weak (quoting, no types); bash/zsh/fish are three languages | Low to start, high to maintain | Startup grows with features | No cross-shell story |

## Decision and rationale

Write `thine` in Go 1.27 as a single static binary (`CGO_ENABLED=0`). The shell layer is static glue emitted by `thine init <shell>` — one-line functions that forward to the binary (see [ADR-0002](0002-static-shell-integration.md)) — the pattern Starship, zoxide and mise use. Go meets the latency budget, gives cgo-free access to every primitive the safety model needs (`pidfd_open`/`pidfd_send_signal`, `flock`, `statfs`, `os.Root`), and cross-compiles all four targets from one CI job. Rust would also satisfy the constraints; Go wins on time-to-value and on the existence of a detailed Go implementation plan.

## Consequences and risk ownership

Benefits: measured cold start 2.2 ms (p50) and `init` 2.2 ms on the reference 8 GB laptop; ~10 MB binaries; one codebase for Linux and macOS with build-tagged adapters (`*_linux.go`, `*_darwin.go`).
Costs / limitations: contributors need Go; macOS adapters shell out to `ps`/`lsof` once per verb (no `/proc`), costing tens of milliseconds there.
Residual risks / owners: macOS behaviour is verified in CI runners, not on a daily-driver Mac (maintainer).

## Transition and recovery

The public contract is the CLI, its exit codes and `--json` schemas; everything lives under `internal/`, so a rewrite (for example in Rust) could replace the binary without changing users' configs or shell blocks.

## Validation and acceptance

| Claim / acceptance criterion | Method or exact command | Evidence / state | Limitation / owner |
|---|---|---|---|
| `thine init` ≤ 5 ms p95 | `thine bench -n 30` | PASS (executed): 2.2 ms p50 / 2.6 ms p95 on Linux x64 | One machine |
| Cold start ≤ 5 ms p95 | `thine bench` | PASS (executed): 2.2 ms p50 / 3.0 ms p95 | One machine |
| All targets build | `make cross` | PASS (executed): 4 binaries, 9.8–10.8 MB | macOS runs happen in CI |
| No cgo | `CGO_ENABLED=0` in Makefile and CI | PASS (executed) | — |

## Revisit when

A required OS API needs cgo, a contributor pool fluent in Rust appears, or Windows-native support becomes a priority that Go's `x/sys/windows` can't serve.
