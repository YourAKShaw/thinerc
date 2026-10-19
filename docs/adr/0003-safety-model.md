# ADR-0003: Plan → preview → apply → log → undo for every state change

Status: accepted
Date: 2026-10-19
Owner / decision authority: maintainer (@YourAKShaw)
Scope / related issue: process signalling, file deletion, quarantine, sudo, agent callers

## Context and constraints

The hand-rolled helpers thine replaces run `kill -9 $(lsof -t -i:$1)` and `rm -rf` globs with no preview and no undo. A cleanup tool that deletes something valuable once loses its users' trust permanently. thine is also called by AI agents that may be prompt-injected.

## Options and trade-offs

| Option | Correctness/security | Complexity/operations | Cost/time | Compatibility/reversibility |
|---|---|---|---|---|
| **Plan objects, identity/fingerprint re-validation, quarantine, write-ahead log** | Defends against TOCTOU, PID reuse, symlink tricks, crashes | One choke point per hazard | Moderate | Undo for user files |
| Confirm prompt, then `rm` / `kill -9` | No undo; vulnerable to PID reuse and races | Simple | Low | None |
| OS trash APIs | Platform-specific; restore semantics vary | Several backends | Moderate | Partial |

## Decision and rationale

- **Processes** are addressed by identity = PID + start time. On Linux the signal goes through `pidfd_open` → start-time check → `pidfd_send_signal` (race-free); macOS checks the start time immediately before `kill(2)`. SIGTERM first; SIGKILL after the grace period only for dev-tier processes or with `--force`. PID 1, thine's ancestors and system daemons are never signalled; other users' processes need `--sudo`.
- **Files** pass a single `Guard` (canonical parent, strictly inside an allowlisted root, outside a deny-list, never `.git`, never following symlinks) and are re-fingerprinted (device, inode, mode, mtime) immediately before acting. User files are **quarantined** by same-filesystem atomic rename with a manifest, restorable with `thine undo` for 7 days; caches are cleaned by their owning tool's own command; cache directories are emptied through `os.Root`.
- **Every apply** takes an exclusive lock, writes an fsynced `intent` record before acting and an `outcome` record after.
- **Without a TTY**, nothing state-changing happens without `--apply` (exit 6), except stopping dev-tier processes on explicitly named ports — there the command line is the plan, so `killfb && pnpm dev` works in scripts.
- **Agents** (detected via `THINE_AGENT`, `CLAUDECODE`, `CODEX_SANDBOX`, …) only plan; a human applies with `thine apply <plan-id>`.

The twelve resulting invariants are listed in [docs/safety.md](../safety.md); each has a `TestS<n>_…` test and `scripts/trace-invariants.sh` fails CI if one is missing.

## Consequences and risk ownership

Benefits: undo for user files; crash evidence in `thine history`; no PID-reuse kills; scripts and agents get safe defaults.
Costs / limitations: quarantined bytes are freed only after retention (the plan says so); cross-filesystem items are skipped rather than deleted.
Residual risks / owners: a compromised agent can bypass thine entirely — agent mode is defence-in-depth, not a security boundary (documented in SECURITY.md).

## Validation and acceptance

| Claim / acceptance criterion | Method or exact command | Evidence / state | Limitation / owner |
|---|---|---|---|
| Guard never admits a path outside roots | `go test -fuzz=FuzzGuard` (random symlink trees) | PASS (executed): 36k execs, 0 escapes | Fuzz time bounded |
| PID reuse is refused | `TestS3_SignalRefusesReusedIdentity` (live process, wrong start time) | PASS (executed) on Linux | macOS has a microsecond window |
| apply ∘ undo = identity | `TestMoveRestoreIsIdentity` (content hashes, symlinks preserved) | PASS (executed) | — |
| Changed items are skipped | `TestS3_ApplySkipsChangedItems` | PASS (executed) | — |
| Agents only plan | `TestS1_AgentModeOnlyPlans`; manual run inside Claude Code | PASS (executed) | — |

## Revisit when

A data-loss incident is reported (forces review), or OS-native trash APIs offer atomic restore on both platforms.
