package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestProbeMissingExecutable(t *testing.T) {
	err := probe(t.Context(), filepath.Join(t.TempDir(), "missing"), time.Second)
	var typed *serverError
	if !errors.As(err, &typed) || typed.Code != "preflight_failed" {
		t.Fatalf("expected preflight failure: %v", err)
	}
}

func TestCancelledProbe(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := probe(ctx, "missing", time.Second)
	var typed *serverError
	if !errors.As(err, &typed) || typed.Code != "cancelled" {
		t.Fatalf("expected cancellation: %v", err)
	}
}

func TestCachedReadiness(t *testing.T) {
	readiness.Lock()
	readiness.binary = "cached-test"
	readiness.until = time.Now().Add(time.Minute)
	readiness.err = nil
	readiness.Unlock()
	defer func() { readiness.Lock(); readiness.until = time.Time{}; readiness.Unlock() }()
	if err := preflight(t.Context(), "cached-test", time.Second); err != nil {
		t.Fatalf("cached success should avoid launching missing executable: %v", err)
	}
	readiness.Lock()
	readiness.until = time.Now().Add(-time.Second)
	readiness.Unlock()
	if err := preflight(t.Context(), "cached-test", time.Second); err == nil {
		t.Fatal("expired cache must probe executable")
	}
}
