package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "gigacode-bridge-mcp", Version: "0.2.1"}, &mcp.ServerOptions{
		Instructions: "Review every returned patch independently before applying it. If delegate_task returns preflight_failed, continue the task directly without retrying delegation. Treat this as unavailable or too slow, not a confirmed outage. Routine availability handling is internal: do not narrate diagnostic steps unless the user asks or they block completion. Report task outcomes truthfully and never claim GigaCode made changes when it did not.",
	})
	var busy atomic.Bool
	mcp.AddTool(server, &mcp.Tool{Name: "delegate_task", Description: "Run a small edit in a temporary detached Git worktree. Supply canonical absolute paths to existing tracked files in one clean repository, exact acceptance criteria, and a timeout of 1-120 seconds. Returns a patch for independent review; never applies or commits it."}, func(ctx context.Context, _ *mcp.CallToolRequest, in Input) (*mcp.CallToolResult, Report, error) {
		var report Report
		var err error
		if !busy.CompareAndSwap(false, true) {
			err = fail("busy", "Server is busy")
		} else {
			defer busy.Store(false)
			bounded, cancel := context.WithTimeout(ctx, 150*time.Second)
			defer cancel()
			report, err = delegate(bounded, in)
		}
		if err == nil {
			return nil, report, nil
		}
		report = Report{Status: "failed", Code: "invalid_or_failed_request", Error: "Request failed validation or execution; no patch accepted"}
		var serverErr *serverError
		if errors.As(err, &serverErr) {
			report.Code = serverErr.Code
			report.Error = serverErr.Message
		}
		data, e := json.Marshal(report)
		if e != nil {
			return nil, Report{}, e
		}
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}, StructuredContent: report}, report, nil
	})
	return server
}

func check(ctx context.Context) (err error) {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "gigacode-check", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: exec.CommandContext(ctx, executable)}, nil)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, session.Close()) }()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		return err
	}
	found := false
	for _, tool := range tools.Tools {
		if tool.Name == "delegate_task" {
			found = true
		}
	}
	if !found {
		return errors.New("delegate_task missing")
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "delegate_task", Arguments: Input{Task: "Validate path", Files: []string{"/tmp/../forbidden"}, AcceptanceCriteria: []string{"No edits"}, TimeoutSec: 1}})
	if err != nil {
		return err
	}
	if !result.IsError {
		return errors.New("invalid path accepted")
	}
	fmt.Println("PASS: stdio handshake, delegate_task discovery and invalid-path rejection")
	return nil
}

func main() {
	checkMode := flag.Bool("check", false, "Check MCP stdio and validation without calling GigaCode")
	installMode := flag.Bool("install-codex", false, "Register this binary in Codex and configure its tool timeout")
	healthMode := flag.Bool("health", false, "Probe real GigaCode readiness with an 8-second budget")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *installMode {
		bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := installCodex(bounded); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *healthMode {
		binary, err := executable()
		if err != nil {
			log.Fatal(err)
		}
		if err = preflight(ctx, binary, 8*time.Second); err != nil {
			log.Fatal(err)
		}
		fmt.Println("PASS: GigaCode confirmed readiness")
		return
	}
	if *checkMode {
		bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if err := check(bounded); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := newServer().Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
