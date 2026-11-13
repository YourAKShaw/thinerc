#!/usr/bin/env bash
# Copyright 2026 Ayush Kumar Shaw
# SPDX-License-Identifier: Apache-2.0
# License header check: every tracked .go and .sh file must carry the
# copyright line and the Apache-2.0 SPDX identifier as whole comment
# lines in its first four lines (below the shebang in scripts).
set -eu
cd "$(dirname "$0")/.."
fail=0
while IFS= read -r -d '' f; do
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
done < <(git ls-files -z -- '*.go' '*.sh')
[ "$fail" -eq 0 ] && echo "✔ every source file has a license header"
exit "$fail"
