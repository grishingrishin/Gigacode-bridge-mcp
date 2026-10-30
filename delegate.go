package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type Input struct {
	Task               string   `json:"task"`
	Files              []string `json:"files"`
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
	TimeoutSec         int      `json:"timeoutSec"`
}
type Report struct {
	Status       string   `json:"status"`
	Repository   string   `json:"repository,omitempty"`
	BaseCommit   string   `json:"baseCommit,omitempty"`
	ChangedFiles []string `json:"changedFiles,omitempty"`
	Patch        string   `json:"patch,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	Applied      bool     `json:"applied"`
	Checks       string   `json:"checks,omitempty"`
	Code         string   `json:"code,omitempty"`
	Error        string   `json:"error,omitempty"`
}

func validText(s string, limit int) bool {
	return strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= limit
}

func validate(ctx context.Context, in Input) (string, []string, error) {
	if !validText(in.Task, 12000) || len(in.Files) < 1 || len(in.Files) > 30 || len(in.AcceptanceCriteria) < 1 || len(in.AcceptanceCriteria) > 20 || in.TimeoutSec < 1 || in.TimeoutSec > 120 {
		return "", nil, fail("invalid_input", "Invalid task, files, criteria or timeout")
	}
	for _, c := range in.AcceptanceCriteria {
		if !validText(c, 2000) {
			return "", nil, fail("invalid_input", "Invalid acceptance criterion")
		}
	}
	for _, f := range in.Files {
		if !filepath.IsAbs(f) || filepath.Clean(f) != f || len(f) > 4096 {
			return "", nil, fail("invalid_path", "Use canonical absolute paths")
		}
		resolved, err := filepath.EvalSymlinks(f)
		if err != nil || resolved != f {
			return "", nil, fail("invalid_path", "File is missing or path contains a symbolic link")
		}
		info, err := os.Lstat(f)
		if err != nil || !info.Mode().IsRegular() {
			return "", nil, fail("invalid_path", "Only regular files are allowed")
		}
	}
	root, err := git(ctx, filepath.Dir(in.Files[0]), "rev-parse", "--show-toplevel")
	if err != nil {
		return "", nil, err
	}
	root = strings.TrimSpace(root)
	allowed := make([]string, 0, len(in.Files))
	for _, f := range in.Files {
		rel, err := filepath.Rel(root, f)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") || slices.Contains(strings.Split(rel, "/"), ".git") {
			return "", nil, fail("invalid_path", "Files must belong to one repository")
		}
		if _, err = git(ctx, root, "ls-files", "--error-unmatch", "--", rel); err != nil {
			return "", nil, fail("invalid_path", "Only tracked files are supported")
		}
		allowed = append(allowed, rel)
	}
	status, err := git(ctx, root, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return "", nil, err
	}
	if status != "" {
		return "", nil, fail("dirty_repository", "Repository must be clean")
	}
	return root, allowed, nil
}
func names(s string) []string { return strings.FieldsFunc(s, func(r rune) bool { return r == 0 }) }
func allowlist(changed, allowed []string) error {
	for _, f := range changed {
		if !slices.Contains(allowed, f) {
			return fail("allowlist_violation", "Unauthorized file: "+f)
		}
	}
	return nil
}

func collect(ctx context.Context, work, base string, allowed []string) ([]string, string, error) {
	raw, err := git(ctx, work, "diff", "--name-only", "-z", base, "--")
	if err != nil {
		return nil, "", err
	}
	changed := names(raw)
	others, err := git(ctx, work, "ls-files", "--others", "-z")
	if err != nil {
		return nil, "", err
	}
	if err = allowlist(append(slices.Clone(changed), names(others)...), allowed); err != nil {
		return nil, "", err
	}
	for _, f := range changed {
		info, e := os.Lstat(filepath.Join(work, f))
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return nil, "", e
		}
		if !info.Mode().IsRegular() {
			return nil, "", fail("invalid_output", "Only regular file outputs are accepted")
		}
	}
	head, err := git(ctx, work, "rev-parse", "HEAD")
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(head) != base {
		return nil, "", fail("unexpected_commit", "GigaCode changed HEAD")
	}
	patch, err := git(ctx, work, "diff", "--binary", "--no-ext-diff", "--no-textconv", base, "--")
	return changed, patch, err
}

func summary(raw string) (string, error) {
	var events []struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &events); err != nil {
		return "", fail("execution_failed", "Invalid GigaCode JSON")
	}
	lower := strings.ToLower(raw)
	for _, marker := range []string{"api error", "fetch failed", "connection error"} {
		if strings.Contains(lower, marker) {
			return "", fail("execution_failed", "GigaCode reported an API error")
		}
	}
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		if e.Type == "result" {
			if e.IsError || e.Subtype != "success" {
				break
			}
			return e.Result, nil
		}
	}
	return "", fail("execution_failed", "GigaCode returned no successful result")
}

func delegate(ctx context.Context, in Input) (report Report, err error) {
	root, allowed, err := validate(ctx, in)
	if err != nil {
		return report, err
	}
	binary, err := executable()
	if err != nil {
		return report, err
	}
	if err = preflight(ctx, binary, min(8*time.Second, time.Duration(in.TimeoutSec)*time.Second)); err != nil {
		return report, err
	}
	base, err := git(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return report, err
	}
	base = strings.TrimSpace(base)
	temp, err := os.MkdirTemp("", "gigacode-bridge-mcp-")
	if err != nil {
		return report, err
	}
	work := filepath.Join(temp, "worktree")
	added := false
	defer func() {
		cleanup := context.WithoutCancel(ctx)
		if added {
			if _, e := git(cleanup, root, "worktree", "remove", "--force", work); e != nil {
				err = errors.Join(err, fail("cleanup_failed", "Worktree cleanup failed: "+work))
				return
			}
		}
		err = errors.Join(err, os.RemoveAll(temp))
	}()
	if _, err = git(ctx, root, "worktree", "add", "--detach", work, base); err != nil {
		return report, err
	}
	added = true
	details, err := json.Marshal(struct {
		Task     string
		Files    []string
		Criteria []string
	}{in.Task, allowed, in.AcceptanceCriteria})
	if err != nil {
		return report, err
	}
	prompt := "Perform one small task in the current worktree. Modify only the listed files. Do not commit, stage, change Git metadata, access other directories or run shell commands. Report edits and checks briefly. Task specification: " + string(details)
	raw, err := run(ctx, work, binary, []string{"--prompt", prompt, "--approval-mode", "auto-edit", "--output-format", "json", "--max-session-turns", "8", "--chat-recording=false"}, time.Duration(in.TimeoutSec)*time.Second)
	if err != nil {
		return report, err
	}
	changed, patch, err := collect(ctx, work, base, allowed)
	if err != nil {
		return report, err
	}
	text, err := summary(raw)
	if err != nil {
		return report, err
	}
	if len(changed) == 0 || patch == "" {
		return report, fail("no_changes", "GigaCode returned without changing files; no patch to review")
	}
	return Report{Status: "review_required", Repository: root, BaseCommit: base, ChangedFiles: changed, Patch: patch, Summary: text, Checks: "Acceptance criteria must be verified independently by the client"}, nil
}
