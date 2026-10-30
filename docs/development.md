# Developer diagnostics

Readiness is compiled into the server and runs automatically for ordinary delegate_task requests. It requires no installation flags, separate daemon, scheduler, environment setting or manual smoke test. MCP initialization instructions communicate fallback behavior to clients, including installations without the optional Codex skill.

## Diagnostics and verification

Before creating a worktree, delegate_task probes the CLI with a one-turn READY request in a temporary directory, using plan approval mode. The probe has an 8-second execution budget (or timeoutSec when smaller), plus bounded process cleanup. Success is cached in the server process for 60 seconds; failure for 30 seconds. preflight_failed instructs the primary agent to handle the task directly without retry. Slow but available providers can fail this budget. Cached success cannot guarantee availability for the next task. Git validation occurs before the probe and has its own deadlines.

Run `./bin/gigacode-bridge-mcp --health` for a standalone real readiness check; unlike --check, this calls the configured provider and can consume tokens. Successful readiness does not shorten the subsequent task timeout. --check remains an offline MCP protocol check.

`./bin/gigacode-bridge-mcp --check` performs a real stdio handshake, lists delegate_task and tests rejection of a traversal path without calling the model.

`go test -race -shuffle=on -count=1 ./...` checks real subprocess timeout, output limits, unauthorized worktree changes, patch extraction, unchanged source before acceptance, and explicit patch application in a disposable Git repository. Fixture edits are written by the test; they do not establish successful model execution.

The opt-in live smoke test creates a disposable committed repository, calls delegate_task, checks source isolation, and checks/applies the returned patch only inside the fixture. Run it after building with `GIGACODE_LIVE_TEST=1 go test -run TestLiveMCP -v -count=1 -timeout=5m .`. Set GIGACODE_MCP_BINARY to test a separately installed binary. Earlier fixture attempts returned no changes; a subsequent real documentation delegation succeeded. The fixture test still deliberately fails for an empty diff.

`GIGACODE_MCP_BINARY=/absolute/path/to/binary go test -race -run TestMCPGuards -v -count=1 .` checks timeout through the real CLI and rejection of dirty repositories while preserving a local draft. This check passed on macOS. Local placement avoids synchronizing repositories across hosts; it does not remove the clean-repository restriction. No stash or commit is performed automatically.

If a task fails, check the CLI version, executable path, provider connectivity and authentication on the execution host. GigaCode may return exit code zero even when its JSON contains an API error; the adapter also checks the result payload. Do not accept a result on exit code alone.
