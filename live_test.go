package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestLiveMCP(t *testing.T) {
	if os.Getenv("GIGACODE_LIVE_TEST") != "1" {
		t.Skip("opt-in real GigaCode test")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, root, "init")
	original := "export function add(a, b) { return a + b; }\n"
	file := filepath.Join(root, "add.js")
	write(t, file, original)
	mustGit(t, root, "add", "add.js")
	mustGit(t, root, "-c", "user.name=Smoke Test", "-c", "user.email=smoke@localhost", "-c", "commit.gpgsign=false", "commit", "-m", "Fixture")
	ctx, cancel := context.WithTimeout(t.Context(), 240*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "live-check", Version: "1"}, nil)
	binary := os.Getenv("GIGACODE_MCP_BINARY")
	if binary == "" {
		binary, err = filepath.Abs("bin/gigacode-bridge-mcp")
		if err != nil {
			t.Fatal(err)
		}
	}
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: exec.CommandContext(ctx, binary)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Error(err)
		}
	}()
	input := Input{Task: "Use read_file and edit tools now to modify add.js on disk. Add JSDoc for numeric parameters and return. Preserve the existing function byte-for-byte. Actually edit the file, do not describe a plan.", Files: []string{file}, AcceptanceCriteria: []string{"JSDoc documents both numeric parameters and numeric return", "Function implementation and signature unchanged"}, TimeoutSec: 120}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "delegate_task", Arguments: input})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var report Report
	if err = json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("MCP failed: %s %s", report.Code, report.Error)
	}
	if report.Status != "review_required" || report.Applied || !strings.Contains(report.Patch, "@param") || !strings.Contains(report.Patch, "@returns") {
		t.Fatalf("Unexpected report: %s", data)
	}
	actual, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != original {
		t.Fatal("source changed before acceptance")
	}
	if strings.TrimSpace(mustGit(t, root, "status", "--porcelain")) != "" {
		t.Fatal("source repository dirty")
	}
	t.Logf("REVIEW PATCH:\n%s", report.Patch)
	patch := filepath.Join(t.TempDir(), "review.patch")
	write(t, patch, report.Patch)
	mustGit(t, root, "apply", "--check", patch)
	mustGit(t, root, "apply", patch)
	actual, err = os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(actual), original) {
		t.Fatal("function implementation changed")
	}
	mustGit(t, root, "restore", "add.js")
	input.TimeoutSec = 1
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "delegate_task", Arguments: input})
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if !result.IsError || report.Code != "timeout" {
		t.Fatalf("Expected timeout: %s", data)
	}
	if strings.TrimSpace(mustGit(t, root, "status", "--porcelain")) != "" {
		t.Fatal("timeout changed source")
	}
	write(t, file, original+"// local draft\n")
	input.TimeoutSec = 120
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "delegate_task", Arguments: input})
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if !result.IsError || report.Code != "dirty_repository" {
		t.Fatalf("Expected dirty rejection: %s", data)
	}
	actual, err = os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != original+"// local draft\n" {
		t.Fatal("local draft changed")
	}
	t.Log("PASS: real MCP generation, isolation, patch application, timeout, dirty repository preservation")
}
