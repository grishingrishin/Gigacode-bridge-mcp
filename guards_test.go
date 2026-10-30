package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPGuards(t *testing.T) {
	binary := os.Getenv("GIGACODE_MCP_BINARY")
	if binary == "" {
		t.Skip("set GIGACODE_MCP_BINARY to test installed server")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, root, "init")
	file := filepath.Join(root, "example.js")
	write(t, file, "export const value = 1;\n")
	mustGit(t, root, "add", ".")
	mustGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@localhost", "-c", "commit.gpgsign=false", "commit", "-m", "Fixture")
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "guards", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: exec.CommandContext(ctx, binary)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Error(err)
		}
	}()
	input := Input{Task: "Add a documentation comment", Files: []string{file}, AcceptanceCriteria: []string{"No behavior change"}, TimeoutSec: 1}
	for _, code := range []string{"preflight_failed", "dirty_repository"} {
		if code == "dirty_repository" {
			write(t, file, "// draft\nexport const value = 2;\n")
		}
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
		if !result.IsError || report.Code != code {
			t.Fatalf("expected %s got %s", code, data)
		}
	}
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "// draft\nexport const value = 2;\n" {
		t.Fatal("draft modified")
	}
	t.Log("PASS: MCP readiness deadline and preservation of uncommitted changes")
}
