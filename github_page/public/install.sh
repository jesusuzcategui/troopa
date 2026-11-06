#!/bin/sh
# Troopa installer: downloads the latest release for this system, checks its
# SHA-256 against checksums.txt and installs the binary.
#   curl -fsSL https://jesusuzcategui.github.io/troopa/install.sh | sh
# Options (environment variables):
#   TROOPA_VERSION=v0.2.0          install a specific version instead of the latest
#   TROOPA_INSTALL_DIR=/usr/local/bin   where to put the binary (default ~/.local/bin)
set -eu

REPO="jesusuzcategui/troopa"
INSTALL_DIR="${TROOPA_INSTALL_DIR:-$HOME/.local/bin}"

fail() { printf 'troopa install: %s\n' "$1" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || fail "$1 is required"; }

need uname
need tar
if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL "$1"; }
  fetch_to() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -qO- "$1"; }
  fetch_to() { wget -qO "$2" "$1"; }
else
  fail "curl or wget is required"
fi

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "unsupported system $(uname -s); on Windows download the zip from https://github.com/$REPO/releases" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "unsupported architecture $(uname -m)" ;;
esac

tag="${TROOPA_VERSION:-}"
if [ -z "$tag" ]; then
  tag=$( (fetch "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null || true) | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)
  if [ -z "$tag" ]; then
    # Only pre-releases (betas) so far: take the newest one.
    tag=$( (fetch "https://api.github.com/repos/$REPO/releases?per_page=1" 2>/dev/null || true) | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)
  fi
  [ -n "$tag" ] || fail "could not find a release"
fi
version="${tag#v}"
asset="troopa_${version}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$tag"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

printf 'Downloading troopa %s for %s/%s…\n' "$tag" "$os" "$arch"
fetch_to "$base/$asset" "$tmp/$asset" || fail "download failed: $base/$asset"
fetch_to "$base/checksums.txt" "$tmp/checksums.txt" || fail "could not download checksums.txt"

expected=$(grep " $asset\$" "$tmp/checksums.txt" | cut -d ' ' -f 1)
[ -n "$expected" ] || fail "$asset is not listed in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$asset" | cut -d ' ' -f 1)
else
  actual=$(shasum -a 256 "$tmp/$asset" | cut -d ' ' -f 1)
fi
[ "$expected" = "$actual" ] || fail "checksum mismatch for $asset"

tar -xzf "$tmp/$asset" -C "$tmp"
mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/troopa" "$INSTALL_DIR/troopa" 2>/dev/null || { cp "$tmp/troopa" "$INSTALL_DIR/troopa" && chmod 0755 "$INSTALL_DIR/troopa"; }

printf 'Installed %s\n' "$INSTALL_DIR/troopa"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) printf 'Add %s to your PATH to run troopa from anywhere.\n' "$INSTALL_DIR" ;;
esac
