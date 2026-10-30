#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mode=${1:-choose}
case "$mode" in manual|codex|choose) ;; *) printf 'Unsupported client: %s\n' "$mode" >&2; exit 1;; esac
if [[ "$mode" == choose ]]; then
  printf 'MCP client:\n  1) Codex (automatic configuration)\n  2) Other client (manual configuration)\n'
  read -r -p 'Select [1/2]: ' answer
  case "$answer" in 1) mode=codex;; 2) mode=manual;; *) printf 'Invalid selection\n' >&2; exit 1;; esac
fi
if [[ "$mode" == codex ]]; then command -v codex >/dev/null || { printf 'Codex CLI is required\n' >&2; exit 1; }; fi
destination=${INSTALL_DIR:-$HOME/.local/bin}
case "$destination" in /*) ;; *) printf 'INSTALL_DIR must be absolute\n' >&2; exit 1;; esac
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
if [[ -n "${ARCHIVE:-}" ]]; then
  tar -xOf "$ARCHIVE" gigacode-bridge-mcp > "$stage/gigacode-bridge-mcp"
elif [[ -f ./gigacode-bridge-mcp ]]; then
  cp ./gigacode-bridge-mcp "$stage/gigacode-bridge-mcp"
elif [[ -f go.mod ]]; then
  command -v go >/dev/null || { printf 'Use a release archive or install Go to build from source\n' >&2; exit 1; }
  go build -o "$stage/gigacode-bridge-mcp" .
else
  printf 'No binary or source found\n' >&2; exit 1
fi
chmod 755 "$stage/gigacode-bridge-mcp"
"$stage/gigacode-bridge-mcp" --check
mkdir -p "$destination"
install -m 755 "$stage/gigacode-bridge-mcp" "$destination/gigacode-bridge-mcp.new"
mv "$destination/gigacode-bridge-mcp.new" "$destination/gigacode-bridge-mcp"
if [[ "$mode" == codex ]]; then
  "$destination/gigacode-bridge-mcp" --install-codex
else
  printf 'Command: %s/gigacode-bridge-mcp\nTransport: stdio\nArguments: none\nTool timeout: 180 seconds\n' "$destination"
fi
