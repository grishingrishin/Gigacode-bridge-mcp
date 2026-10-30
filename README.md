# GigaCode Bridge MCP

**MCP stdio server for delegating isolated code changes to GigaCode CLI.**

The server executes tasks in temporary Git worktrees and returns patches for independent review. The MCP client controls task selection, acceptance, and patch application.

[Installation](#installation) · [Usage](#usage) · [Limitations](#limitations) · [Troubleshooting](#troubleshooting) · [Build from source](#build-from-source)

![MCP request and response flow](docs/assets/architecture.svg)

## Requirements

| Component | Requirement |
| --- | --- |
| Operating system | macOS or Linux |
| Git | Available on PATH |
| GigaCode CLI | Installed, authenticated, and connected to its configured provider |
| MCP client | Supports stdio transport |
| Repository | Clean working tree with at least one commit |

Prebuilt binaries do not require Go. The MCP server does not require Node.js, Docker, or a background service.

## Installation

### 1. Download the archive

Open [Releases](https://github.com/grishingrishin/gigacode-bridge-mcp/releases) and download the archive for your platform. The release version replaces `vX.Y.Z` in the filename.

| Platform | Archive filename suffix |
| --- | --- |
| Apple Silicon Mac, M1 or newer | `darwin_arm64.tar.gz` |
| Intel Mac | `darwin_amd64.tar.gz` |
| Linux, Intel or AMD, x86_64 | `linux_amd64.tar.gz` |
| Linux, ARM64, aarch64 | `linux_arm64.tar.gz` |

Use `uname -s -m` to identify the operating system and architecture. Windows is not currently supported.

> **Release availability:** If no release is listed, [build from source](#build-from-source). Access to a private repository is required to download its releases.

Extract the archive into its own directory and open a terminal there. It contains the binary, this guide, and the documentation.

### 2. Install and configure

From the extracted archive directory:

```sh
make setup
```

Select **Codex** for automatic registration and a 900-second tool timeout, or **Other client** for binary installation and manual connection instructions. Restart the client after installation.

| Command | Action |
| --- | --- |
| `make setup` | Install and select a client interactively |
| `make install` | Install the binary only |
| `make check` | Check the installed MCP connection |

Codex CLI must be available on PATH for automatic registration. The installer respects CODEX_HOME and backs up an existing config.toml before modifying it. Other client entries are preserved. The optional skill is installed separately as described below.

The default install directory is `~/.local/bin`. Override it with `INSTALL_DIR=/absolute/path`. From a source checkout, you can install directly from a downloaded archive:

```sh
make setup ARCHIVE=/absolute/path/to/release.tar.gz
```

Only the executable is extracted from the archive. From a source checkout without ARCHIVE, the installer builds locally and requires Go. From an extracted release, no Go installation is required.

If Make is unavailable, run `bash scripts/install.sh choose` from the extracted directory. Installation requires Bash and standard POSIX utilities. It does not require sudo.

For other MCP clients, use stdio, the printed absolute executable path, no arguments, and a tool timeout of 900 seconds. Configuration field names depend on the client.

By default, GigaCode is expected at `~/.gigacode/bin/gigacode`. For another path, see [Custom GigaCode path](#custom-gigacode-path).

## Usage

Example request:

> Add documentation to the exported functions in src/utils.go without changing their behavior. Review the diff before applying it.

To help Codex choose suitable tasks automatically, install the optional skill from the extracted archive:

```sh
mkdir -p "$HOME/.codex/skills"
cp -R skills/delegate-to-gigacode "$HOME/.codex/skills/"
```

If a skill with that name already exists, compare versions first to preserve your customizations. If you use a custom CODEX_HOME, use its skills directory instead.

The client decides when to delegate. The skill helps it choose small tasks; it does not force a tool call for every request. If the executor is unavailable, the primary agent should continue on its own when it can complete the task.

| Suitable for delegation | Retained by the primary agent |
| --- | --- |
| API documentation | Architecture decisions |
| Small mechanical edits | Security-sensitive changes |
| Focused tests | Complex business logic and refactoring |

Every returned diff requires independent review before application.

![Review first, apply second](docs/assets/review-flow.svg)

## Limitations

- A **clean Git repository** with at least one commit is required. The server does not save or stash uncommitted changes; it declines delegation.
- Only the specified **existing tracked files** in one repository can be changed. Creating new files is not supported yet.
- The result is a patch for review. The server neither applies it to the original project nor creates commits.
- A temporary worktree isolates Git edits, but it is not an OS sandbox. Use trusted projects and tasks.
- Local operation has been verified on macOS. Linux binaries have been cross-compiled but not verified on a Linux host.

## Troubleshooting

### Connection verification

Check the MCP connection without calling the model:

```sh
"$HOME/.local/bin/gigacode-bridge-mcp" --check
```

### Custom GigaCode path

```sh
codex mcp add gigacode-bridge-mcp \
  --env GIGACODE_BIN="$(command -v gigacode)" \
  -- "$HOME/.local/bin/gigacode-bridge-mcp"
```

First confirm that `command -v gigacode` returns a path. After updating the registration, check `tool_timeout_sec` and restart the client.

**macOS blocks the app:** binaries are not yet signed or notarized. If you trust the source, allow this specific app through Privacy & Security settings. Do not disable system protection globally.

**No usable diff:** the primary agent should not treat a text report as proof that the task is complete. It checks the result and continues on its own when needed.

## Update and uninstall

To update, download a new archive, close sessions using the MCP server, repeat `make install`, and restart the client. The path stays the same, so registration does not need to change.

Remove the Codex connection:

```sh
codex mcp remove gigacode-bridge-mcp
```

You can then remove the installed binary and the optional skill.

## Build from source

Building requires Go 1.25 or newer:

```sh
git clone https://github.com/grishingrishin/gigacode-bridge-mcp.git
cd gigacode-bridge-mcp
go build -o bin/gigacode-bridge-mcp .
mkdir -p "$HOME/.local/bin"
install -m 755 bin/gigacode-bridge-mcp "$HOME/.local/bin/gigacode-bridge-mcp"
```

Then connect your client using the instructions above.

## Additional documentation

- [Tool contract, SSH, and boundaries](docs/reference.md)
- [Diagnostics and testing](docs/development.md)
- [Release preparation](docs/releasing.md)

Releases include `SHA256SUMS` alongside the archives. To verify all downloads, place all four archives and that file in one directory, then run `shasum -a 256 -c SHA256SUMS` on macOS or `sha256sum -c SHA256SUMS` on Linux. If you downloaded only one archive, compare its checksum with the corresponding line in the file. Checksums verify integrity; they are not a digital signature.
