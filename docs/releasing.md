# Releasing

Releases are cut from `main` by pushing a `vX.Y.Z` tag. [`release.yml`](../.github/workflows/release.yml) does the rest.

## Checklist

1. `make check` is green locally and CI is green on `main`.
2. Move the **Unreleased** notes in [`CHANGELOG.md`](../CHANGELOG.md) under a new `## [X.Y.Z] - YYYY-MM-DD` heading and update the compare links.
3. Manual pass on a real machine (Linux, and macOS if you have one):
   - `thine setup` and `thine setup --uninstall`
   - `killfb` against a real `pnpm dev` (job scope), and a Docker-published port
   - `cleanup` — read the dry-run plan line by line, apply in a throwaway home, `undo`
   - `freeup` before/after
   - `thine notify --test`
4. Commit (`chore(release): 🔖 release vX.Y.Z` with a filled body and footers), then:

   ```sh
   git tag -s vX.Y.Z -m "thine vX.Y.Z"
   git push origin vX.Y.Z
   ```

## What the pipeline does

1. **tag** — refuses tags without a matching `CHANGELOG.md` section.
2. **ci** — re-runs the full CI workflow on the tagged commit.
3. **build** — deterministic `tar.gz` archives for `linux_amd64`, `linux_arm64`, `darwin_amd64`, `darwin_arm64` (binary + LICENSE + README + CHANGELOG), `install.sh`, and `checksums.txt`; then **build-provenance attestations** via GitHub OIDC.
4. **verify** — each archive is checksum-verified and executed on a native runner.
5. **github-release** — creates the release with notes taken from the changelog.

Users verify downloads with:

```sh
sha256sum --ignore-missing -c checksums.txt          # macOS: shasum -a 256 --ignore-missing -c checksums.txt
gh attestation verify thine_X.Y.Z_linux_amd64.tar.gz --repo YourAKShaw/.thinerc
```

## Bad-release runbook

Releases are immutable — always roll forward.

1. Edit the GitHub release notes: mark it **known bad** and say why.
2. Ship a patch release with the fix and a regression test.
3. If safety is involved (data loss, injection), open a GitHub Security Advisory and add a **Known issues** entry to the changelog.
