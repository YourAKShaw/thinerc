# ADR-0007: Vendor-neutral agent glue; thine never calls a model

Status: accepted
Date: 2026-10-19
Owner / decision authority: maintainer (@YourAKShaw)
Scope / related issue: Claude Code, Codex CLI, Gemini CLI and other coding agents

## Context and constraints

Developers run several coding agents and hand-roll the same glue for each: "notify me when it's done", "which agents are running where", "keep CLAUDE.md and AGENTS.md consistent". Agents also call thine's verbs themselves — and may be prompt-injected.

## Decision and rationale

- thine **never calls an LLM** and ships no chat client.
- `thine notify` is one notifier for every agent (Claude Code `Stop`/`Notification` hooks via stdin JSON, Codex `notify` via its argument) and for scripts. Payloads are capped at 64 KiB, sanitized, coalesced within 3 s, passed to `osascript`/`notify-send`/a WSL toast **as argv only**, and the command always exits 0 as a hook so it can never block an agent.
- `thine ai doctor` reports installed agents, versions, hook status and context files; `thine ai hooks` prints ready-to-paste config. thine does not edit agent configuration files in this version — users stay in control of them.
- **Agent mode** (`THINE_AGENT=1`, `CLAUDECODE`, `CODEX_SANDBOX`, `GEMINI_CLI`, `CURSOR_AGENT`): destructive and regenerable actions only produce plans; read-only verbs and dev-tier kills on explicit targets work; `thine apply <plan-id>` and `thine trust` refuse agent callers.

## Consequences and risk ownership

Benefits: one hook for every agent; safe defaults when an agent drives the terminal.
Costs / limitations: hook installation is manual (copy-paste) until each vendor's format is stable enough to edit structurally.
Residual risks / owners: environment markers change between agent releases (re-verify per release).

## Validation and acceptance

| Claim / acceptance criterion | Method or exact command | Evidence / state | Limitation / owner |
|---|---|---|---|
| Agent mode only plans | `TestS1_AgentModeOnlyPlans`; running `cleanup --apply --yes` inside Claude Code saved a plan and exited 6 | PASS (executed) | — |
| Hook never fails the agent | `notify --from claude` with an escape-laden payload → exit 0 | PASS (executed) | — |

## Revisit when

Agents converge on a shared hook standard, or users ask thine to write agent configs (then: structural merge, backup, diff, byte-identical uninstall).
