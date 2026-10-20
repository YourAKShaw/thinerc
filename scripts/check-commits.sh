#!/bin/sh
# Commit message check: fail when a commit breaks the convention in .github/CONTRIBUTING.md.
#   - header: <type>(<scope>)[!]: <gitmoji> <summary of at most 72 characters>
#   - a blank line, then a body explaining why
#   - footers: Breaking-Changes: none (…) or BREAKING CHANGE: … with "!", plus Refs:
#   - any Co-Authored-By trailer uses exactly that key casing
# Usage: scripts/check-commits.sh <revision range or commit>   (merge and Dependabot commits are skipped)
set -eu
cd "$(dirname "$0")/.."
[ $# -eq 1 ] || { echo "usage: $0 <base..head | commit>" >&2; exit 2; }
case "$1" in
	*..*) commits=$(git rev-list --no-merges "$1") ;;
	*) commits=$(git rev-list --no-merges -n 1 "$1") ;;
esac
types='feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert'
fail=0

for sha in $commits; do
	# Merged Dependabot pull requests reach main with Dependabot as author.
	[ "$(git log -1 --format=%an "$sha")" != "dependabot[bot]" ] || continue
	msg=$(git log -1 --format=%B "$sha")
	header=$(printf '%s\n' "$msg" | sed -n 1p)
	problems=""
	add() { problems="$problems    $1
"; }

	# Byte-wise matching: any non-ASCII run after the colon counts as a gitmoji.
	if ! printf '%s\n' "$header" | LC_ALL=C grep -Eq "^($types)\([a-z0-9][a-z0-9._/-]*\)!?: (:[a-z0-9_+-]+:|[^ -~]+) [^ ]"; then
		add "header must be '<type>(<scope>): <gitmoji> <summary>' with type one of: $(echo "$types" | tr '|' ' ')"
	else
		summary=$(printf '%s\n' "$header" | LC_ALL=C sed -E 's/^[^:]*: (:[a-z0-9_+-]+:|[^ -~]+) //')
		length=$(printf '%s' "$summary" | LC_ALL=C.UTF-8 wc -m | tr -d ' ')
		[ "$length" -le 72 ] || add "summary is $length characters; keep it to 72 or fewer"
	fi

	[ -z "$(printf '%s\n' "$msg" | sed -n 2p)" ] || add "leave one blank line after the header"
	[ -n "$(printf '%s\n' "$msg" | sed -n '3,$p' | grep -Ev '^([A-Za-z-]+|BREAKING CHANGE): ' | grep -v '^[[:space:]]*$' || true)" ] ||
		add "add a body that explains why the change was made"

	breaking=$(printf '%s\n' "$msg" | grep -c '^BREAKING CHANGE: ' || true)
	compatible=$(printf '%s\n' "$msg" | grep -c '^Breaking-Changes: none (.*)$' || true)
	bang=$(printf '%s\n' "$header" | grep -c '^[^:]*!: ' || true)
	[ $((breaking + compatible)) -eq 1 ] ||
		add "declare exactly one of 'Breaking-Changes: none (<why>)' or 'BREAKING CHANGE: <impact>'"
	[ "$breaking" -eq "$bang" ] || add "use '!' after the scope if and only if there is a BREAKING CHANGE footer"
	printf '%s\n' "$msg" | grep -q '^Refs: .' || add "add a 'Refs:' footer naming the doc, ADR or #issue"
	if printf '%s\n' "$msg" | grep -i '^co-authored-by:' | grep -Evq '^Co-Authored-By: [^<>]+ <[^<>@ ]+@[^<> ]+>$'; then
		add "write co-author trailers as 'Co-Authored-By: Name <email>'"
	fi

	if [ -n "$problems" ]; then
		echo "✖ $(git log -1 --format='%h %s' "$sha")"; printf '%s' "$problems"; fail=1
	fi
done

[ "$fail" -eq 0 ] && echo "✔ commit messages follow the convention"
exit "$fail"
