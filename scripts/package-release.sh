#!/usr/bin/env bash
set -euo pipefail

version=${1:?Usage: bash scripts/package-release.sh vX.Y.Z}
if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'Expected a version such as v0.2.0\n' >&2
  exit 1
fi
cd "$(dirname "$0")/.."
output="$PWD/dist/$version"
if [[ -e "$output" ]]; then
  printf 'Output already exists: %s\n' "$output" >&2
  exit 1
fi
mkdir -p "$output"
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
mkdir -p "$stage/docs/assets" "$stage/skills/delegate-to-gigacode" "$stage/scripts"
cp Makefile "$stage/Makefile"
cp scripts/install.sh "$stage/scripts/"
cp scripts/uninstall.sh "$stage/scripts/"
cp README.md "$stage/README.md"
cp docs/*.md "$stage/docs/"
cp docs/assets/*.svg "$stage/docs/assets/"
cp skills/delegate-to-gigacode/SKILL.md "$stage/skills/delegate-to-gigacode/"
for target in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64; do
  target_os=${target%/*}
  target_arch=${target#*/}
  CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go build -trimpath -ldflags='-s -w' -o "$stage/gigacode-bridge-mcp" .
  COPYFILE_DISABLE=1 tar -czf "$output/gigacode-bridge-mcp_${version}_${target_os}_${target_arch}.tar.gz" -C "$stage" gigacode-bridge-mcp Makefile scripts README.md docs skills
done
cd "$output"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum ./*.tar.gz > SHA256SUMS
else
  shasum -a 256 ./*.tar.gz > SHA256SUMS
fi
printf 'Release assets ready: %s\n' "$output"
