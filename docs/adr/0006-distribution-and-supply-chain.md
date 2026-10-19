# ADR-0006: Checksummed, provenance-attested archives; no auto-update, no telemetry

Status: accepted
Date: 2026-10-19
Owner / decision authority: maintainer (@YourAKShaw)
Scope / related issue: release artifacts, install path, network access

## Context and constraints

A tool that kills processes and deletes files must be the most trustworthy binary on the machine. The npm `kill-port` package — the obvious dependency for the alias this project replaces — had a command-injection CVE (CVE-2019-5414) and shipped credential-stealing malware in the Shai-Hulud 2.0 attack (GMS-2025-498).

## Decision and rationale

- Releases are built by `release.yml` on a tag: deterministic `tar.gz` archives for linux/darwin × amd64/arm64, `checksums.txt`, and GitHub build-provenance attestations (OIDC, no long-lived keys). Each archive is verified and executed on a native runner before the release is created.
- `scripts/install.sh` is short enough to read, verifies SHA-256 (and provenance when `gh` is available), installs to `~/.local/bin` without sudo and edits no rc files.
- Three runtime dependencies (cobra, go-toml, go-runewidth) plus `golang.org/x/{sys,term}`; Dependabot proposes updates after a 7-day cooldown; actions are pinned by commit SHA.
- **No network in normal operation.** `internal/netfetch` is the only package allowed to import `net/http` (enforced by a test and a lint script); it serves only the explicit `thine fonts install`, which refuses unpinned downloads and verifies SHA-256. No telemetry, no auto-update, no update checks.

## Consequences and risk ownership

Benefits: verifiable artifacts; tiny supply-chain surface; nothing phones home.
Costs / limitations: no Homebrew tap or package-manager channels yet; binaries are not code-signed for macOS Gatekeeper.
Residual risks / owners: maintainer account takeover (2FA with a hardware key; protected tags).

## Validation and acceptance

| Claim / acceptance criterion | Method or exact command | Evidence / state | Limitation / owner |
|---|---|---|---|
| Only netfetch touches the network | `TestS11_OnlyNetfetchTouchesTheNetwork`, `scripts/check-boundaries.sh` | PASS (executed) | — |
| Pinned font download verified | `thine fonts install victor-mono` in a throwaway home | PASS (executed): checksum matched, 63 files installed | — |
| Release pipeline | `release.yml` | NOT RUN (no tag pushed yet) | First tag validates it |

## Revisit when

Users ask for Homebrew/apt channels or macOS notarisation; GitHub changes attestation tooling.
