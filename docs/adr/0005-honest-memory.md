# ADR-0005: `freeup` reports available memory and reclaims by stopping real consumers

Status: accepted
Date: 2026-10-19
Owner / decision authority: maintainer (@YourAKShaw)
Scope / related issue: memory reporting and the `freeup` verb

## Context and constraints

The `freeup` alias that started this project ran `sudo … drop_caches` and appeared to free 1–2 GB on an 8 GB laptop. The Linux kernel documentation says dropping caches "can cause performance problems" and is not recommended outside testing; on macOS `purge` only flushes the file cache. The "freed" memory was page cache the kernel already hands back on demand.

## Options and trade-offs

| Option | Correctness/security | Complexity/operations | Cost/time | Compatibility/reversibility |
|---|---|---|---|---|
| **Report MemAvailable + pressure; stop real consumers; cache drop opt-in with a before/after** | Honest numbers | Per-OS readers | Low | — |
| Drop caches by default to show a big number | Misleading; slows the next file reads | Simple | — | — |

## Decision and rationale

`freeup` shows total, **available**, PSI pressure (Linux) or `memory_pressure` (macOS), swap and page cache; groups top consumers by application using PSS where possible; and offers to stop the user's own dev servers and language servers through the same TERM → grace → KILL engine as `kill`, then reports the *measured* change. `--drop-caches` explains why it's cosmetic, prints the exact `sudo` argv, confirms, and shows page-cache and available deltas side by side — which usually teaches the lesson on its own.

## Consequences and risk ownership

Benefits: real headroom, an honest number, and a teaching moment.
Costs / limitations: the number reclaimed is smaller than the old alias claimed — by design (see the FAQ in the troubleshooting guide).

## Validation and acceptance

| Claim / acceptance criterion | Method or exact command | Evidence / state | Limitation / owner |
|---|---|---|---|
| Parsers correct | `TestSnapshotFromMeminfo`, `TestParsePSI`, `TestParseVMStatAndSwap` | PASS (executed) | macOS values from fixtures |
| Report on a real 8 GB machine | `thine freeup` | PASS (executed): 1.7 GiB available of 7.1 GiB, browser grouped ×33 | — |

## Revisit when

Never for the default — this is a product-integrity decision.
