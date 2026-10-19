# The safety model

> `thine` kills processes, deletes and moves files, can run a few `sudo` commands, and executes recipes. It is designed so that **you can't regret using it**: every state change is planned, previewed, logged, and — for your own files — undoable.

This page is the contract. Each invariant below has at least one automated test named `TestS<n>_…`, and [`scripts/trace-invariants.sh`](../scripts/trace-invariants.sh) fails CI if any loses its test. The reasoning is in [ADR-0003](adr/0003-safety-model.md).

## The lifecycle of every change

```text
scan ─► plan ─► preview ─► approve ─► apply ─► log ─► (undo)
                   │           │          ├─ each item re-validated right before acting
                   │           │          ├─ partial  → exit 4
                   │           │          └─ Ctrl-C   → finish current item, exit 130
                   │           └─ declined → nothing changed, exit 0
                   └─ no terminal and no --apply → plan printed (and saved), exit 6
```

## Risk levels

Every action carries one of four risk levels, and the level decides how much confirmation it needs.

| Risk | Meaning | Examples | On a terminal | Without a terminal | Agent mode |
|---|---|---|---|---|---|
| **read-only** | Changes nothing | `port`, `ports`, `whatsup`, plans, read-only recipes | Runs | Runs | Runs |
| **reversible** | Undoable or trivially restartable | Stop your own dev server; quarantine a stale `node_modules` | Runs (dev kills) / asks (cleanup) | Dev kills on explicit ports run; quarantine needs `--apply` | Dev kills on configured groups; otherwise plan only |
| **regenerable** | Not undoable, but rebuilt automatically | npm/pnpm/pip caches, Docker build cache, `.next/` | Preview, then `[y/N]` | `--apply` required | Plan only |
| **destructive** | Neither | Purging the quarantine, destructive recipes | Preview, then type a phrase | Refused without explicit flags | Refused |

## The twelve invariants

| ID | Invariant | Enforced in | Test |
|---|---|---|---|
| **S1** | No state change without a plan you can see. With no terminal and no `--apply`, thine prints the plan and exits 6. The one exception: stopping your own dev processes on ports you *named* — the command line is the plan, so `killfb && pnpm dev` works in scripts. | `cli` decision points | `TestS1_AgentModeOnlyPlans` |
| **S2** | File targets are canonical, strictly inside an allowlisted root, outside the deny-list (`~/.ssh`, keychains, `/etc`, `/usr`, thine's own state…), never `.git`, and never reached through a symlink. A symlink item is handled as a link. | `cleanup.Guard` — the single choke point | `TestS2_GuardRejectsEscapes`, `FuzzGuard` |
| **S3** | Each item is re-validated **immediately** before acting: files by device + inode + mode + mtime, processes by PID + start time. A mismatch means *skip and report*, never "best effort". | `cleanup.Revalidate`, `procs.Signal` | `TestS3_ApplySkipsChangedItems`, `TestS3_SignalRefusesReusedIdentity` |
| **S4** | Signals go only to planned identities. Never PID 1, never thine's own ancestors (your shell, terminal, editor), never system daemons, never another user's process without `--sudo`. | `procs.Policy.Classify`, `procs.Signal` | `TestS4_ClassifyProtectsSystemAndAncestors` |
| **S5** | Every subprocess runs from an argv array with a deadline — no shell interpolation. Shell recipes are explicit, labelled, and receive parameters as positional arguments. | `platform.Run`, `recipes.Run` | `TestS5_ShellRecipeParamsArePositional` |
| **S6** | `sudo` runs only argv prefixes from an exhaustive registry, only on explicit flags, after printing the exact command. thine refuses to run as root. | `platform.Sudo` | `TestS6_SudoRegistryIsExhaustive` |
| **S7** | Write-ahead logging: an fsynced `intent` record before any action, an `outcome` record after. A crash leaves evidence `thine history` shows as *incomplete*. | `oplog.Append` | `TestS7_IntentWithoutOutcomeIsIncomplete` |
| **S8** | Untrusted text — process names, command lines, file names, agent payloads — is stripped of escape sequences and control characters, and length-bounded, before it reaches your terminal, a notification or a log. | `ui.Sanitize` | `TestS8_SanitizeStripsEscapesAndControls` |
| **S9** | One apply at a time per user: an exclusive lock on the state directory. Read verbs take no lock. | `platform.AcquireLock` | `TestS9_LockIsExclusive` |
| **S10** | Ctrl-C during an apply finishes the current (atomic) item, records a partial result, and stops. | `cleanup.Apply` | `TestS10_ApplyStopsAfterCancel` |
| **S11** | No network I/O except the explicit `thine fonts install`, which verifies a pinned SHA-256. Only `internal/netfetch` may import `net/http`. No telemetry, no update checks. | import boundary | `TestS11_OnlyNetfetchTouchesTheNetwork`, `scripts/check-boundaries.sh` |
| **S12** | A project's `.thinerc.toml` is ignored until `thine trust` pins its SHA-256; any edit requires re-trust. Projects can never loosen agent policy. | `config.Load`, `config.Trust` | `TestS12_UntrustedProjectIgnored` |

## Processes: who thine will and won't stop

| Tier | Who | What thine does |
|---|---|---|
| **protected** | PID 1, thine's ancestors, desktop/system daemons (`systemd`, `gnome-shell`, `WindowServer`…), `sshd` carrying your session | Refuses (exit 5) and says why |
| **confirm** | Databases, Docker, IDEs, browsers, `java`, anything not recognised as a dev runtime, other users' processes | Shows the plan and asks; no automatic SIGKILL; `--sudo` for other users |
| **dev** | Your own `node`, `bun`, `deno`, `python`, `vite`, `next-server`, `tsx`, `uvicorn`… — or anything holding a port from a configured group | SIGTERM, a visible grace period (3 s), then SIGKILL if it's still alive |

**Job scope.** A dev server is usually the child of a runner (`pnpm dev`, `nodemon`, `concurrently`). For groups (`killfb`) thine stops the whole process group — but only if the group leader isn't a shell, terminal or IDE, it isn't thine's own group, and every member is yours. The plan lists every member first.

**Docker.** If a port is held by `docker-proxy` (or `vpnkit`, `gvproxy`…), killing the proxy is wrong. thine maps the port to its container and plans `docker stop <container>`, which runs with `--docker` or your confirmation.

## Files: what cleanup touches

- **Caches** are cleaned with the owning tool's own command (`npm cache clean --force`, `pnpm store prune`, `go clean -cache`, `docker builder prune -f`…) — never by deleting a tool's directories behind its back.
- **Your files** (stale `node_modules`, old installers in `~/Downloads`) are **quarantined**: moved by an atomic same-filesystem rename into `~/.local/share/thine/quarantine/<op-id>/`, with a manifest. `thine undo` restores them; if the original path is taken, the item lands at `<path>.thine-restored-N` — nothing is ever overwritten. After 7 days (configurable) the next `cleanup --apply` lists and purges them.
- **Artifacts** qualify only if a marker sits beside them (`package.json` for `node_modules`, `Cargo.toml` for `target/`), the project has been idle longer than `stale_after` (30 days), and git tracks nothing inside them.
- **Sizes are honest**: allocated bytes, with hardlinks deduplicated. pnpm and Bun hardlink packages from a global store, so a naive size can overstate savings by 100%; thine shows that part as "shared with a store" and never counts it as freed.
- Items on another filesystem than the quarantine are skipped (an atomic move is impossible), not deleted.

## Agent mode

AI agents call thine too. When `THINE_AGENT=1`, or an agent's environment marker is present (`CLAUDECODE`, `CODEX_SANDBOX`, `GEMINI_CLI`, `CURSOR_AGENT`):

- read-only verbs, `--json` output and dev-tier kills on configured groups work normally;
- cleanup, freeup, `nm reset`, non-read-only recipes and confirm-tier kills only **plan** (exit 6), and cleanup plans are saved;
- a human applies a saved plan with `thine apply <plan-id>`; `apply`, `trust` and `trash purge` refuse agent callers.

Agent mode is **defence-in-depth, not a security boundary**: a compromised agent can unset variables or run `rm` itself. The agent's own sandbox is the real boundary. thine's job is to make the safe path the easy path.

## When something goes wrong

```sh
thine history          # every operation, with its result and whether it's undoable
thine trash ls         # what's in quarantine, and when it expires
thine undo             # restore the most recent undoable operation
thine undo 01JB8Q4M    # …or a specific one, by ID prefix
```

The operations log lives at `~/.local/state/thine/ops.jsonl` (mode 0600). Credentials in command lines are redacted before they're written. If thine ever deletes, moves or kills something it shouldn't have, please [report it](../.github/SECURITY.md) — data-loss bugs block releases.
