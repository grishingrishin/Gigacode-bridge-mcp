# GigaCode Bridge MCP

![GigaCode Bridge MCP architecture](docs/assets/architecture.svg)

A small Go MCP server that runs bounded GigaCode CLI tasks in temporary Git worktrees and returns patches for review. Uses stdio, with no HTTP service or database.

## Requirements

- macOS or Linux with Git on PATH. Go 1.25+ is needed only for building.
- An installed, authenticated GigaCode CLI with access to its configured provider.
- An MCP client that supports stdio.
- A clean Git repository with an existing commit.

Local checks have been run on macOS. Linux, remote SSH transport, and live model execution have not yet been verified.

## Install

```sh
git clone https://github.com/grishingrishin/gigacode-bridge-mcp.git
cd gigacode-bridge-mcp
go mod download
go build -o bin/gigacode-bridge-mcp .
go test -race -shuffle=on -count=1 ./...
./bin/gigacode-bridge-mcp --check
```

The default executable is `~/.gigacode/bin/gigacode`, resolved using the server account's home directory. Set `GIGACODE_BIN` to an absolute executable path for a different installation.

The adapter uses these CLI options, confirmed with GigaCode 26.5.17:

```text
--prompt
--approval-mode auto-edit
--output-format json
--max-session-turns 8
--chat-recording=false
```

## Connect locally

For Codex, run this from the installation directory:

```sh
codex mcp add gigacode-bridge-mcp -- "$PWD/bin/gigacode-bridge-mcp"
codex mcp get gigacode-bridge-mcp
```

In the corresponding `[mcp_servers.gigacode-bridge-mcp]` section of the client's configuration, set `tool_timeout_sec = 900` to allow long tasks plus Git setup and cleanup. Restart the client and inspect its MCP tool list. The exposed tool is `delegate_task`.

Other MCP clients can launch `/absolute/path/gigacode-bridge-mcp` using stdio. Standard output is reserved for protocol messages.

## Connect over SSH

Copy the binary built for the execution host and install Git and GigaCode there. Node.js is not required by the MCP server; GigaCode may have its own runtime requirements. Enable key-based SSH authentication and verify its host key through your normal SSH setup.

An example Codex configuration is:

```toml
[mcp_servers.gigacode-bridge-mcp]
command = "ssh"
args = [
  "-T",
  "-o", "BatchMode=yes",
  "-o", "ConnectTimeout=10",
  "mcp-host",
  "/absolute/path/to/gigacode-bridge-mcp"
]
tool_timeout_sec = 900
```

Replace the host alias and executable paths. Use fixed paths without spaces or shell metacharacters in the SSH command. Use an absolute binary path and ensure Git is on the remote PATH. Ensure shell startup scripts do not print banners to stdout.

SSH starts the MCP process; the MCP process starts GigaCode on demand. Neither needs a manually opened interactive session. A process manager or HTTP listener is unnecessary.

The repository must already exist on the execution host. File arguments refer to that host's canonical absolute paths. This server does not synchronize repositories. Before applying a returned patch elsewhere, compare the report's baseCommit with the receiving repository and run `git apply --check`.

## Tool contract

| Parameter | Type | Constraint |
| --- | --- | --- |
| task | string | 1-12,000 characters |
| files | string[] | 1-30 canonical absolute paths |
| acceptanceCriteria | string[] | 1-20 nonempty criteria |
| timeoutSec | integer | 1-600 seconds |

All files must be existing, tracked regular files in one clean repository. Symbolic-link paths, parent traversal, new files and dirty repositories are unsupported. On macOS, resolve aliases such as /tmp to canonical paths before calling.

```json
{
  "task": "Add a concise JSDoc comment to the exported function without changing behavior.",
  "files": ["/absolute/path/project/src/example.js"],
  "acceptanceCriteria": [
    "Document parameters and return value",
    "Preserve the function signature and implementation"
  ],
  "timeoutSec": 120
}
```

A successful response contains:

- status: review_required
- repository and baseCommit
- changedFiles and patch
- summary, applied: false, and a reminder to verify acceptance independently

Errors return an MCP error result with a structured code and no accepted patch. Examples include timeout, output_limit, invalid_path, dirty_repository and allowlist_violation.

## Review workflow

![Review lifecycle](docs/assets/review-flow.svg)

The client reviews every diff, verifies the criteria independently and decides whether to apply the patch. A GigaCode summary is not evidence of correctness. Save an accepted patch and run:

```sh
git apply --check /path/to/review.patch
git apply /path/to/review.patch
```

The server never applies patches to the source checkout or creates commits. A portable optional Codex skill is provided in `skills/delegate-to-gigacode/SKILL.md`; copy its directory into your client's skills directory.

## Boundaries

- Child arguments use os/exec without a shell.
- Combined child output is limited to 1 MiB; each Git command has the same output bound.
- Timeout, cancellation and excess output trigger process-group termination. Subprocess pipe cleanup has a one-second bound.
- Every Git command has a separate 20-second bound. timeoutSec covers the GigaCode process, not total request duration.
- Only one task runs per server process.
- Final tracked, untracked and ignored changes are checked against the allowlist.
- Symbolic-link outputs and changes to HEAD are rejected.
- A detached worktree is removed after execution, including ordinary failures.
- Raw stderr and credential files are not returned. Patches and summaries contain task data and must be handled accordingly.

A worktree is Git isolation, not an OS sandbox. The process has the execution account's filesystem permissions and shares Git metadata. The allowlist detects final changes inside the worktree, not attempted or reverted writes or changes elsewhere. Use trusted repositories and tasks. Crashes or forced server termination can leave worktrees behind; inspect `git worktree list` before manual cleanup.

## Diagnostics and verification

`./bin/gigacode-bridge-mcp --check` performs a real stdio handshake, lists delegate_task and tests rejection of a traversal path without calling the model.

`go test -race -shuffle=on -count=1 ./...` checks real subprocess timeout, output limits, unauthorized worktree changes, patch extraction, unchanged source before acceptance, and explicit patch application in a disposable Git repository. Fixture edits are written by the test; they do not establish successful model execution.

For a live smoke test, use a disposable committed repository, call delegate_task, review its returned diff, verify the original file remains unchanged, and explicitly check and apply the patch. Live inference and the SSH deployment path remain unverified.

If a task fails, check the CLI version, executable path, provider connectivity and authentication on the execution host. GigaCode may return exit code zero even when its JSON contains an API error; the adapter also checks the result payload. Do not accept a result on exit code alone.

## References

- [Official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [Codex MCP configuration](https://developers.openai.com/codex/mcp)

## Cross-compilation

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/gigacode-bridge-mcp-linux-amd64 .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/gigacode-bridge-mcp-linux-arm64 .
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/gigacode-bridge-mcp-darwin-arm64 .
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/gigacode-bridge-mcp-darwin-amd64 .
```

Use the binary matching the host OS and CPU. Windows is not supported by the POSIX process-group implementation. Cross-compilation is not runtime verification.
