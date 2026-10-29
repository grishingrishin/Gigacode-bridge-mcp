---
name: delegate-to-gigacode
description: Delegate small isolated coding tasks to a GigaCode MCP server and independently review its returned patch.
---

Use delegate_task for small low-risk edits with exact acceptance criteria. Keep architecture, security, business logic and complex refactors with the primary agent. Do not delegate just to increase tool usage.

Supply task, files, acceptanceCriteria and timeoutSec. Files must be canonical absolute paths on the execution host, referring to existing tracked regular files in one clean Git repository. The tool creates an authorized temporary detached worktree from HEAD. Do not commit or stash user changes merely to satisfy this prerequisite.

Inspect every returned diff. Confirm repository and baseCommit, verify acceptance independently, then check and apply the patch only within the user's authorized scope. Never trust the summary alone or create commits automatically. Allow at most one corrective retry, then handle the task directly or report the blocker.

Remote deployment requires a pre-existing repository on the execution host. The tool does not synchronize files. Worktrees are not OS sandboxes; use trusted code and tasks.
