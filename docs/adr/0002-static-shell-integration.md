# ADR-0002: Static, collision-guarded shell integration with no hooks

Status: accepted
Date: 2026-10-19
Owner / decision authority: maintainer (@YourAKShaw)
Scope / related issue: how thine enters bash, zsh and fish

## Context and constraints

Shell frameworks are measurably slow (a stock oh-my-zsh config sits at 187% of zsh-bench's first-prompt-lag threshold) and per-prompt hooks add latency to every command. Users already have aliases with the same names thine wants (`killport`, `freeup`), and some manage their rc files with chezmoi or stow.

## Options and trade-offs

| Option | Correctness/security | Complexity/operations | Cost/time | Compatibility/reversibility |
|---|---|---|---|---|
| **`eval "$(thine init zsh)"` emitting guarded one-line functions** | Logic stays in the tested binary | Three small templates | ~2.8 ms per new shell, 0 ms per command | Remove one marked block |
| Sourced shell framework | Quoting bugs; untestable | Grows per shell | Grows per feature | Hard to remove cleanly |
| `precmd`/`preexec` hooks | Runs on every prompt | — | Per-command lag | — |

## Decision and rationale

`thine init <bash|zsh|fish>` prints static code and nothing else (compatible with Powerlevel10k instant prompt). Each shortcut is a one-line function that forwards to `command thine …`, defined **only if the name is free** — the user's own alias wins, and `thine doctor` reports where it is defined. Functions use the `function name { … }` form: the `name() { … }` form is alias-expanded at parse time and broke in real zsh when an alias already existed (caught by the real-shell test). No hooks are installed by default; the opt-in `[shell] greeting` is the only startup output, and it does no scans.

`thine setup` writes a marked block **at the end** of the rc file (so the collision guard sees every alias defined above it), backs the file up, writes atomically, preserves the mode, refuses symlinked rc files (dotfile managers), and `--uninstall` restores the original byte for byte. Completions are installed as lazily-loaded files, not evaluated at startup.

## Consequences and risk ownership

Benefits: zero per-command cost; a user's own definitions are never overridden silently; easy removal.
Costs / limitations: a shortcut name the user already owns is skipped until they `force`, `rename` or remove theirs.
Residual risks / owners: shells not covered (nushell, PowerShell) need their own templates (maintainer, on demand).

## Validation and acceptance

| Claim / acceptance criterion | Method or exact command | Evidence / state | Limitation / owner |
|---|---|---|---|
| Init prints only shell code | `TestInitProducesNoOutput` sources it in bash and zsh | PASS (executed) | fish when installed |
| User aliases survive | `TestInitScriptsInRealShells` with a pre-existing `alias whoport` | PASS (executed) in bash and zsh | — |
| Setup idempotent; uninstall byte-identical | `TestSetupIdempotentAndUninstallRestores`; CI smoke job | PASS (executed) | — |
| Startup cost | `thine bench` | PASS (executed): ≈ 2.8 ms added per new zsh | One machine |

## Revisit when

A feature needs per-prompt behaviour that can be shown to run in ≤ 2 ms, or a widely used shell asks for support.
