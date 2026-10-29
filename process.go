package main

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type serverError struct {
	Code    string
	Message string
}

func (e *serverError) Error() string  { return e.Message }
func fail(code, message string) error { return &serverError{code, message} }

type capture struct {
	mu       sync.Mutex
	stdout   bytes.Buffer
	total    int
	exceeded bool
	cancel   context.CancelFunc
}
type outputWriter struct {
	capture *capture
	stdout  bool
}

func (w outputWriter) Write(p []byte) (int, error) {
	c := w.capture
	c.mu.Lock()
	defer c.mu.Unlock()
	c.total += len(p)
	if c.total > 1024*1024 {
		c.exceeded = true
		c.cancel()
		return len(p), nil
	}
	if w.stdout {
		return c.stdout.Write(p)
	}
	return len(p), nil
}

func run(ctx context.Context, dir, binary string, args []string, timeout time.Duration) (string, error) {
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	childCtx, stop := context.WithCancel(bounded)
	defer stop()
	cmd := exec.CommandContext(childCtx, binary, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	cmd.WaitDelay = time.Second
	output := &capture{cancel: stop}
	cmd.Stdout = outputWriter{output, true}
	cmd.Stderr = outputWriter{output, false}
	err := cmd.Run()
	if output.exceeded {
		return "", fail("output_limit", "Process output exceeded 1 MiB")
	}
	if errors.Is(bounded.Err(), context.DeadlineExceeded) {
		return "", fail("timeout", "Process timed out")
	}
	if ctx.Err() != nil {
		return "", fail("cancelled", "Request cancelled")
	}
	if err != nil {
		return "", fail("process_failed", "Process failed; check executable, connectivity and authentication")
	}
	return output.stdout.String(), nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	return run(ctx, dir, "git", append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false"}, args...), 20*time.Second)
}
