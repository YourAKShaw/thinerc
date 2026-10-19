## What changed and why?

<!-- Describe the problem and the resulting behaviour. Link related issues; use "Closes #123" only when this PR resolves the issue. -->

## Verification

<!-- List commands actually run and their results (for example `make check`, a manual `killfb` against a real dev server, a run on macOS). Explain what was not run and why (no macOS machine, no WSL, no Docker…). -->

## Risks or compatibility

<!-- Effects on safety (what could be signalled, moved, deleted or executed), shell startup latency, config files, JSON output contracts, or Linux/macOS differences. Write "None identified" when appropriate. -->

## Safety self-review (required for the `safety` label)

<!-- Delete this section if the PR touches none of: procs, cleanup, quarantine, platform, netfetch, recipes, trust. -->

1. What could this delete, move, signal or execute that it shouldn't?
2. What happens if it's interrupted halfway, or run twice at once?
3. What does an agent (or a malicious repository) get to do with it?
4. How does the user see — and undo — what happened?

## Checklist

- [ ] The change is focused, and every commit follows Conventional Commits with a gitmoji, a body and filled footers (see CONTRIBUTING.md).
- [ ] I ran `make check` and recorded the result above.
- [ ] Tests cover the new behaviour, including the failure path; safety invariants still trace (`scripts/trace-invariants.sh`).
- [ ] `--json` output changes are additive, or the break is documented in the changelog.
- [ ] User-visible changes are documented (README or `docs/`) and listed under **Unreleased** in `CHANGELOG.md`.
- [ ] No real secrets, private paths or personal data appear in code, tests, fixtures or this description; verification claims reflect checks actually performed.
