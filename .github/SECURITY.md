# Security policy

`thine` signals processes, deletes and moves files, can run a small set of `sudo` commands, and executes user-defined recipes. A tool like that has to be the most trustworthy binary on your machine, so security and data-safety reports are our top priority. Thank you for helping keep users safe.

## Supported versions

| Version | Supported |
| :--- | :---: |
| Latest `0.x` release | ✓ |
| Older releases | ✗ — please upgrade |

## Reporting a vulnerability

**Please do not open a public issue, discussion or pull request for a vulnerability.**

Report it privately through GitHub: open the repository's **Security** tab and choose **Report a vulnerability** ([direct link](https://github.com/YourAKShaw/.thinerc/security/advisories/new)).

Please include:

- The affected version (`thine version --json`) and platform (`thine doctor --json` output, sanitized).
- What an attacker controls (a cloned repository's `.thinerc.toml`, a file or process name, an agent hook payload, a recipe parameter…) and what they gain.
- Minimal steps to reproduce, using a **throwaway** directory (`THINE_HOME=$(mktemp -d)` isolates thine's state and home).
- Any suggested fix or mitigation.

What to expect (targets for a project maintained in spare time):

- Acknowledgement within 3 business days.
- An initial assessment and severity within 7 days.
- A fix or mitigation plan agreed with you, followed by a release and a GitHub Security Advisory crediting you unless you prefer otherwise.

Please give us reasonable time to release a fix before disclosing publicly.

## In scope

Anything that breaks the guarantees in the [safety model](../docs/safety.md), for example:

- A file outside an allowlisted root being deleted, moved or followed through a symlink (S2), or an item changed since planning being acted on (S3).
- A process signalled that thine classifies as protected — PID 1, thine's own ancestors, another user's process without `--sudo` — or a recycled PID being signalled (S3/S4).
- Command or argument injection through a recipe parameter, file name, process name, notification payload or config value (S5, T5, T6).
- `sudo` used for anything outside the registered call sites (S6).
- Terminal-escape injection reaching your terminal from untrusted text (S8).
- An untrusted or edited `.thinerc.toml` being honoured without `thine trust` (S12), or a project config loosening agent policy.
- Any network access outside the explicit `thine fonts install` (S11), or secrets written to the ops log.
- Supply-chain issues in release artifacts, checksums or provenance.

## Out of scope

- Actions a user explicitly approves after an accurate preview (for example, applying a cleanup plan that lists the item).
- A compromised AI agent bypassing thine entirely (unsetting `THINE_AGENT`, running `rm` directly). Agent mode is defence-in-depth; the agent's own sandbox is the security boundary. This is documented, not a vulnerability.
- Recipes you wrote or trusted yourself doing what they say.
- Vulnerabilities in your shell, terminal emulator, or the tools thine calls (npm, docker, git…).

## Data-loss bugs

Even when not exploitable, a confirmed case of thine deleting, moving or killing something it shouldn't have is treated with security priority: it blocks releases until a fix and a regression test ship. If you were affected, check `thine history` and `thine trash ls` — quarantined items are restorable with `thine undo <op-id>` during the retention period (7 days by default).
