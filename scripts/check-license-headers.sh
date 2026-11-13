#!/bin/sh
# Copyright 2026 Ayush Kumar Shaw
# SPDX-License-Identifier: Apache-2.0
# License header check: every tracked .go and .sh file must carry the
# copyright line and the Apache-2.0 SPDX identifier in its first lines.
set -eu
cd "$(dirname "$0")/.."
fail=0
for f in $(git ls-files '*.go' '*.sh'); do
	head=$(head -n 4 "$f")
	if ! printf '%s\n' "$head" | grep -q 'Copyright [0-9]\{4\} Ayush Kumar Shaw' ||
		! printf '%s\n' "$head" | grep -q 'SPDX-License-Identifier: Apache-2.0'; then
		echo "✖ $f: missing copyright or SPDX header"
		fail=1
	fi
done
[ "$fail" -eq 0 ] && echo "✔ every source file has a license header"
exit "$fail"
