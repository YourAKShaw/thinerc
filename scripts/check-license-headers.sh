#!/usr/bin/env bash
# Copyright 2026 Ayush Kumar Shaw
# SPDX-License-Identifier: Apache-2.0
# License header check: every tracked .go and .sh file must carry the
# copyright line and the Apache-2.0 SPDX identifier as whole comment
# lines in its first four lines (below the shebang in scripts).
set -eu
cd "$(dirname "$0")/.."
fail=0
checked=0
# Tracked paths are relative, so a leading '/' can never be a real file.
sentinel='/git-ls-files-failed'
while IFS= read -r -d '' f; do
	if [ "$f" = "$sentinel" ]; then
		echo "✖ git ls-files failed; no headers were checked"
		fail=1
		continue
	fi
	checked=$((checked + 1))
	case "$f" in
	*.go) c='//' ;;
	*) c='#' ;;
	esac
	head=$(head -n 4 -- "$f")
	if ! printf '%s\n' "$head" | grep -qxE "$c Copyright [0-9]{4} Ayush Kumar Shaw" ||
		! printf '%s\n' "$head" | grep -qxF "$c SPDX-License-Identifier: Apache-2.0"; then
		echo "✖ $f: missing copyright or SPDX header"
		fail=1
	fi
done < <(git ls-files -z -- '*.go' '*.sh' || printf '%s\0' "$sentinel")
if [ "$checked" -eq 0 ] && [ "$fail" -eq 0 ]; then
	echo "✖ no tracked .go or .sh files found; refusing to pass"
	fail=1
fi
[ "$fail" -eq 0 ] && echo "✔ every source file has a license header"
exit "$fail"
