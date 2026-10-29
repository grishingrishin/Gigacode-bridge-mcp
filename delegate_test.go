package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProcessTimeout(t *testing.T) {
	_, err := run(t.Context(), t.TempDir(), "sleep", []string{"10"}, 50*time.Millisecond)
	var typed *serverError
	if !errors.As(err, &typed) || typed.Code != "timeout" {
		t.Fatalf("expected timeout, got %v", err)
	}
}

func TestOutputLimit(t *testing.T) {
	_, err := run(t.Context(), t.TempDir(), "yes", nil, 5*time.Second)
	var typed *serverError
	if !errors.As(err, &typed) || typed.Code != "output_limit" {
		t.Fatalf("expected output_limit, got %v", err)
	}
}

func TestCancelledProcess(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := run(ctx, t.TempDir(), "sleep", []string{"10"}, time.Second)
	var typed *serverError
	if !errors.As(err, &typed) || typed.Code != "cancelled" {
		t.Fatalf("expected cancelled, got %v", err)
	}
}

func TestAPIErrorIsRejected(t *testing.T) {
	_, err := summary(`[{"type":"result","subtype":"success","is_error":false,"result":"[API Error: fetch failed]"}]`)
	if err == nil {
		t.Fatal("API error accepted")
	}
}

func TestSuccessfulSummary(t *testing.T) {
	text, err := summary(`[{"type":"result","subtype":"success","result":"done"}]`)
	if err != nil || text != "done" {
		t.Fatalf("unexpected %q %v", text, err)
	}
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	text, err := git(t.Context(), dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRealGitIsolationAllowlistAndPatch(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, root, "init")
	original := "export function add(a,b){return a+b;}\n"
	write(t, filepath.Join(root, "add.js"), original)
	write(t, filepath.Join(root, "other.txt"), "original\n")
	mustGit(t, root, "add", ".")
	mustGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@localhost", "-c", "commit.gpgsign=false", "commit", "-m", "Fixture")
	in := Input{Task: "Add documentation", Files: []string{filepath.Join(root, "add.js")}, AcceptanceCriteria: []string{"Preserve behavior"}, TimeoutSec: 10}
	if _, _, err = validate(t.Context(), in); err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(mustGit(t, root, "rev-parse", "HEAD"))
	work := filepath.Join(t.TempDir(), "worktree")
	mustGit(t, root, "worktree", "add", "--detach", work, base)
	defer mustGit(t, root, "worktree", "remove", "--force", work)
	write(t, filepath.Join(work, "add.js"), "/** Add numbers. */\n"+original)
	write(t, filepath.Join(work, "other.txt"), "unauthorized\n")
	_, _, err = collect(t.Context(), work, base, []string{"add.js"})
	var typed *serverError
	if !errors.As(err, &typed) || typed.Code != "allowlist_violation" {
		t.Fatalf("expected allowlist rejection: %v", err)
	}
	write(t, filepath.Join(work, "other.txt"), "original\n")
	_, patch, err := collect(t.Context(), work, base, []string{"add.js"})
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filepath.Join(root, "add.js"))
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != original {
		t.Fatal("source modified")
	}
	patchPath := filepath.Join(t.TempDir(), "review.patch")
	write(t, patchPath, patch)
	mustGit(t, root, "apply", "--check", patchPath)
	mustGit(t, root, "apply", patchPath)
	actual, err = os.ReadFile(filepath.Join(root, "add.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(actual), "Add numbers") {
		t.Fatal("patch did not apply")
	}
}
