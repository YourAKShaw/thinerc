#!/bin/sh
# Architectural fitness functions: fail the build when a safety boundary leaks.
#   - net/http is imported only by internal/netfetch (no hidden network calls)
#   - os/exec is used only by internal/platform (every subprocess goes through one argv wrapper)
#   - "sh -c" appears only in internal/recipes (explicit, labelled shell recipes)
set -eu
cd "$(dirname "$0")/.."
fail=0

check() { # $1 = pattern, $2 = allowed dir, $3 = message
	hits=$(grep -rln --include='*.go' "$1" cmd internal 2>/dev/null | grep -v "^$2/" | grep -v '_test.go$' || true)
	if [ -n "$hits" ]; then
		echo "✖ $3:"; echo "$hits" | sed 's/^/    /'; fail=1
	fi
}

check '"net/http"' internal/netfetch 'net/http imported outside internal/netfetch'
check '"os/exec"' internal/platform 'os/exec imported outside internal/platform'
check '"sh", "-c"' internal/recipes 'shell interpolation outside internal/recipes'

[ "$fail" -eq 0 ] && echo "✔ boundaries hold"
exit "$fail"
