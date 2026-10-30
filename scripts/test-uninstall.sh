#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
test_root=$(mktemp -d)
trap 'rm -rf "$test_root"' EXIT
export CODEX_HOME="$test_root/codex"
export INSTALL_DIR="$test_root/bin with spaces"
mkdir -p "$CODEX_HOME"
codex mcp add unrelated -- /usr/bin/true
printf '1\n' | make setup
cp "$CODEX_HOME/config.toml" "$test_root/before.toml"
touch "$INSTALL_DIR/keep-me"
make uninstall
test ! -e "$INSTALL_DIR/gigacode-bridge-mcp"
test -f "$INSTALL_DIR/keep-me"
cmp "$test_root/before.toml" "$CODEX_HOME/config.toml"
make uninstall
make install
printf '1\n' | make teardown
test ! -e "$INSTALL_DIR/gigacode-bridge-mcp"
test -f "$INSTALL_DIR/keep-me"
codex mcp get unrelated >/dev/null
if codex mcp get gigacode-bridge-mcp >/dev/null 2>&1; then
  printf 'MCP registration was not removed\n' >&2
  exit 1
fi
backups=("$CODEX_HOME"/config.toml.before-uninstall.*)
cmp "$test_root/before.toml" "${backups[0]}"
printf '1\n' | make teardown
mkdir -p "$INSTALL_DIR/gigacode-bridge-mcp"
if make uninstall; then
  printf 'Directory removal should have been rejected\n' >&2
  exit 1
fi
test -d "$INSTALL_DIR/gigacode-bridge-mcp"
printf 'PASS: binary-only removal, client deregistration, backups, repeated removal and directory protection\n'
