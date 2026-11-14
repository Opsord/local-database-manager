//go:build !windows

package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStopDockerEngine_NonWindows_FailsFast(t *testing.T) {
	t.Parallel()
	r := NewRunner("/tmp")
	start := time.Now()
	err := r.stopDockerEngine(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrEngineStartFailed) {
		t.Fatalf("expected ErrEngineStartFailed, got %v", err)
	}
	if !strings.Contains(err.Error(), "auto-stop supported on Windows") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("stopDockerEngine took %v, want immediate failure", elapsed)
	}
}
