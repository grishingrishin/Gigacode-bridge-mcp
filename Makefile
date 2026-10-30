.PHONY: help install setup check
.DEFAULT_GOAL := help

help:
	@printf '%s\n' 'make install        Install the binary only' 'make setup          Install and choose Codex or manual configuration' 'make check          Check the installed MCP connection' 'Optional: ARCHIVE=/path/to/release.tar.gz INSTALL_DIR=/absolute/path'

install:
	@bash scripts/install.sh manual

setup:
	@bash scripts/install.sh choose

check:
	@"$${INSTALL_DIR:-$$HOME/.local/bin}/gigacode-bridge-mcp" --check

export ARCHIVE
export INSTALL_DIR
