#!/bin/sh
# Traceability fitness function: every safety invariant S1–S12 from
# docs/safety.md must have at least one test named TestS<n>_…
set -eu
cd "$(dirname "$0")/.."
missing=0
for n in 1 2 3 4 5 6 7 8 9 10 11 12; do
	if grep -rqs --include='*_test.go' "func TestS${n}_" internal; then
		printf '  ✔ S%-2s %s\n' "$n" "$(grep -rho --include='*_test.go' "func TestS${n}_[A-Za-z0-9_]*" internal | head -1 | sed 's/func //')"
	else
		printf '  ✖ S%-2s has no test\n' "$n"
		missing=1
	fi
done
exit "$missing"
