#!/bin/sh
# thine installer — short enough to read before you run it.
#
#   curl -fsSL https://raw.githubusercontent.com/YourAKShaw/.thinerc/main/scripts/install.sh | sh
#
# What it does: detects your OS/arch, downloads the release archive and
# checksums.txt from GitHub Releases, verifies the SHA-256 (and the build
# provenance if the GitHub CLI is available), and installs `thine` into
# ~/.local/bin. No sudo, no shell-rc edits — run `thine setup` afterwards.
#
# Environment: THINE_VERSION=v0.1.0 pins a version; THINE_BIN_DIR overrides
# the install directory.
set -eu

repo="YourAKShaw/.thinerc"
bin_dir="${THINE_BIN_DIR:-$HOME/.local/bin}"

say() { printf '\033[1;35m✦\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m✖\033[0m %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"; }

need uname
need tar
if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL --proto '=https' --tlsv1.2 -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -q --https-only -O "$2" "$1"; }
else
	die "need curl or wget"
fi

case "$(uname -s)" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) die "unsupported OS: $(uname -s) (thine supports Linux and macOS)" ;;
esac
case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) die "unsupported architecture: $(uname -m)" ;;
esac

version="${THINE_VERSION:-}"
if [ -z "$version" ]; then
	tmpv="$(mktemp)"
	fetch "https://api.github.com/repos/$repo/releases/latest" "$tmpv" || die "couldn't query the latest release"
	version="$(sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' "$tmpv" | head -n1)"
	rm -f "$tmpv"
	[ -n "$version" ] || die "couldn't determine the latest version; set THINE_VERSION"
fi

archive="thine_${version#v}_${os}_${arch}.tar.gz"
base="https://github.com/$repo/releases/download/$version"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT INT TERM

say "downloading thine $version for $os/$arch"
fetch "$base/$archive" "$work/$archive" || die "download failed: $base/$archive"
fetch "$base/checksums.txt" "$work/checksums.txt" || die "couldn't download checksums.txt"

say "verifying SHA-256"
expected="$(grep " $archive\$" "$work/checksums.txt" | cut -d' ' -f1)"
[ -n "$expected" ] || die "$archive is not listed in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
	actual="$(sha256sum "$work/$archive" | cut -d' ' -f1)"
else
	actual="$(shasum -a 256 "$work/$archive" | cut -d' ' -f1)"
fi
[ "$expected" = "$actual" ] || die "checksum mismatch — refusing to install (expected $expected, got $actual)"

if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
	say "verifying build provenance with gh attestation verify"
	gh attestation verify "$work/$archive" --repo "$repo" >/dev/null || die "provenance verification failed"
fi

tar -xzf "$work/$archive" -C "$work" thine
mkdir -p "$bin_dir"
install -m 0755 "$work/thine" "$bin_dir/thine" 2>/dev/null || { cp "$work/thine" "$bin_dir/thine" && chmod 0755 "$bin_dir/thine"; }

say "installed $bin_dir/thine"
case ":$PATH:" in
	*":$bin_dir:"*) ;;
	*) printf '  add it to your PATH:  export PATH="%s:$PATH"\n' "$bin_dir" ;;
esac
printf '  next:  thine setup   (wires it into your shell)\n'
printf '  then:  thine         (say hello)\n'
