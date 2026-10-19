# ADR-0004: Strict TOML configuration; project files ignored until trusted

Status: accepted
Date: 2026-10-19
Owner / decision authority: maintainer (@YourAKShaw)
Scope / related issue: configuration format, precedence and project trust

## Context and constraints

Teams want to share port groups and recipes per repository, but a cloned repository is untrusted input: a malicious `.thinerc.toml` could redefine `killfb` to run anything the next time the user types it.

## Options and trade-offs

| Option | Correctness/security | Complexity/operations | Cost/time | Compatibility/reversibility |
|---|---|---|---|---|
| **TOML, strict decoding, hash-pinned trust (direnv/mise model)** | Typos fail loudly; untrusted files never execute | Trust store + notice | Low | Re-trust after edits |
| YAML | Indentation and implicit-typing pitfalls | Same | Same | — |
| Trust project config by default | Arbitrary command execution from `git clone` | Simplest | — | — |

## Decision and rationale

Global config lives at `$XDG_CONFIG_HOME/thine/config.toml` (also on macOS). Decoding is strict: unknown keys fail with line, column and key. Precedence is flags > `THINE_*` env > trusted project > global > defaults; lists merge, maps override by name. A project's `.thinerc.toml` (found by walking up to the git root) is ignored — with a one-line notice — until `thine trust` shows it and pins its SHA-256; any edit flips it to "changed" and it is ignored again. Agent policy, forced shortcuts, cleanup roots, retention and the shell greeting are honoured from the global file only, so a project can never loosen them.

## Consequences and risk ownership

Benefits: shareable team config without shared risk; config typos are caught immediately.
Costs / limitations: every pulled change to `.thinerc.toml` requires re-trust.
Residual risks / owners: a user who trusts a malicious file after review (documented warning; recipes are listed before trusting).

## Validation and acceptance

| Claim / acceptance criterion | Method or exact command | Evidence / state | Limitation / owner |
|---|---|---|---|
| Untrusted → trusted → changed lifecycle | `TestS12_UntrustedProjectIgnored` and a manual run | PASS (executed) | — |
| Projects can't loosen agent policy | Same test asserts `agents.policy.cleanup` stays `plan` | PASS (executed) | — |
| Unknown keys rejected | `TestUnknownKeyRejected` | PASS (executed) | — |

## Revisit when

Teams need an organisation-wide trust policy, or users ask for JSON/YAML (a second decoder is cheap).
