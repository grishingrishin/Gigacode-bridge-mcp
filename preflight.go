package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var readiness struct {
	sync.Mutex
	binary string
	until  time.Time
	err    error
}

func executable() (string, error) {
	if binary := os.Getenv("GIGACODE_BIN"); binary != "" {
		return binary, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gigacode", "bin", "gigacode"), nil
}

func probe(ctx context.Context, binary string, budget time.Duration) (err error) {
	dir, err := os.MkdirTemp("", "gigacode-preflight-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(dir)) }()
	raw, err := run(ctx, dir, binary, []string{"--prompt", "Reply with exactly READY. Do not call any tools or change files.", "--approval-mode", "plan", "--output-format", "json", "--max-session-turns", "1", "--chat-recording=false"}, budget)
	if ctx.Err() != nil {
		return fail("cancelled", "Request cancelled")
	}
	if err != nil {
		return fail("preflight_failed", "Delegation is unavailable; continue the task directly without retrying")
	}
	answer, err := summary(raw)
	if err != nil || strings.TrimSpace(answer) != "READY" {
		return fail("preflight_failed", "Delegation is unavailable; continue the task directly without retrying")
	}
	return nil
}

func preflight(ctx context.Context, binary string, budget time.Duration) error {
	readiness.Lock()
	defer readiness.Unlock()
	if ctx.Err() != nil {
		return fail("cancelled", "Request cancelled")
	}
	if readiness.binary == binary && time.Now().Before(readiness.until) {
		return readiness.err
	}
	err := probe(ctx, binary, budget)
	if ctx.Err() != nil {
		return err
	}
	ttl := time.Minute
	if err != nil {
		ttl = 30 * time.Second
	}
	readiness.binary = binary
	readiness.err = err
	readiness.until = time.Now().Add(ttl)
	return err
}
