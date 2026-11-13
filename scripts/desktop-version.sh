#!/bin/sh
# Writes a release version into the desktop build assets, so the .app bundle,
# the .deb and the Windows installer report the version of the tag they were
# built from instead of the placeholder in build/config.yml.
#
#   scripts/desktop-version.sh v0.2.0
#
# Pre-release suffixes are dropped (v0.2.0-beta.1 becomes 0.2.0): Windows and
# macOS only accept numbers here. The file names of the artifacts keep the
# full tag.
set -eu

[ $# -eq 1 ] || { printf 'usage: %s <version>\n' "$0" >&2; exit 1; }

version=${1#v}      # v0.2.0        -> 0.2.0
version=${version%%-*}  # 0.2.0-beta.1  -> 0.2.0
case "$version" in
  [0-9]*.[0-9]*.[0-9]*) ;;
  *) printf 'desktop-version: %s is not a major.minor.patch version\n' "$1" >&2; exit 1 ;;
esac

cd "$(dirname "$0")/.."

# sed -i is not portable between GNU and BSD, so write next to the file.
edit() {
  file=$1
  shift
  sed "$@" "$file" > "$file.tmp"
  mv "$file.tmp" "$file"
}

# What the wails templates are rendered from, for anyone regenerating them.
edit build/config.yml -e "s|^  version: .*|  version: \"$version\"|"

# macOS: the <string> under each of the two version keys.
awk -v v="$version" '
  prev ~ /<key>CFBundleVersion<\/key>|<key>CFBundleShortVersionString<\/key>/ {
    sub(/<string>[^<]*<\/string>/, "<string>" v "</string>")
  }
  { print; prev = $0 }
' build/darwin/Info.plist > build/darwin/Info.plist.tmp
mv build/darwin/Info.plist.tmp build/darwin/Info.plist

# Windows: the .exe resources, the manifest and the installer.
edit build/windows/info.json \
  -e "s|\"file_version\": \".*\"|\"file_version\": \"$version\"|" \
  -e "s|\"ProductVersion\": \".*\"|\"ProductVersion\": \"$version\"|"
edit build/windows/wails.exe.manifest \
  -e "s|\(name=\"dev.troopa.desktop\" version=\)\"[^\"]*\"|\1\"$version\"|"
edit build/windows/nsis/wails_tools.nsh \
  -e "s|^\( *!define INFO_PRODUCTVERSION \).*|\1\"$version\"|"

# Linux: the .deb.
edit build/linux/nfpm/nfpm.yaml -e "s|^version: \".*\"|version: \"$version\"|"

printf 'desktop build assets set to version %s\n' "$version"
