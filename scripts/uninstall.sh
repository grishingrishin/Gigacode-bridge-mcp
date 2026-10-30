#!/usr/bin/env bash
set -euo pipefail
mode=${1:-choose}
case "$mode" in manual|choose) ;; *) printf 'Unsupported mode: %s\n' "$mode" >&2; exit 1;; esac
destination=${INSTALL_DIR:-$HOME/.local/bin}
case "$destination" in /*) ;; *) printf 'INSTALL_DIR must be absolute\n' >&2; exit 1;; esac
if [[ "$mode" == choose ]]; then
  printf 'Remove MCP installation:\n  1) Codex connection and binary\n  2) Binary only (configure other clients manually)\n'
  read -r -p 'Select [1/2]: ' answer
  case "$answer" in 1) mode=codex;; 2) mode=manual;; *) printf 'Invalid selection\n' >&2; exit 1;; esac
fi
if [[ "$mode" == codex ]]; then
  command -v codex >/dev/null || { printf 'Codex CLI is required; no files removed\n' >&2; exit 1; }
  config_directory=${CODEX_HOME:-$HOME/.codex}
  if [[ -f "$config_directory/config.toml" ]]; then
    backup=$(mktemp "$config_directory/config.toml.before-uninstall.XXXXXX")
    chmod 600 "$backup"
    cp "$config_directory/config.toml" "$backup"
    codex mcp remove gigacode-bridge-mcp
    printf 'Configuration backup: %s\n' "$backup"
  else
    printf 'No Codex configuration found; skipping deregistration\n'
  fi
fi
binary="$destination/gigacode-bridge-mcp"
if [[ -e "$binary" || -L "$binary" ]]; then
  if [[ -d "$binary" && ! -L "$binary" ]]; then
    printf 'Refusing to remove a directory: %s\n' "$binary" >&2
    exit 1
  fi
  rm -- "$binary"
  printf 'Removed: %s\n' "$binary"
else
  printf 'Binary is already absent: %s\n' "$binary"
fi
if [[ "$mode" == manual ]]; then
  printf 'Client configurations were not changed. Remove their MCP connection manually if needed.\n'
fi
printf 'Restart active clients to stop existing server processes. Optional skills, configuration backups, GigaCode and project files were preserved.\n'
