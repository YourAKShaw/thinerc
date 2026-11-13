// Copyright 2026 Ayush Kumar Shaw
// SPDX-License-Identifier: Apache-2.0

package interrupt

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

func watch(t *testing.T) (context.Context, chan os.Signal, chan struct{}) {
	t.Helper()
	sig := make(chan os.Signal, 2)
	exited := make(chan struct{})
	ctx := Watch(context.Background(), sig, func() { close(exited) })
	return ctx, sig, exited
}

func waitFor(t *testing.T, c <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-c:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func TestSignalOutsideProtectedSectionExitsAtOnce(t *testing.T) {
	ctx, sig, exited := watch(t)
	sig <- syscall.SIGINT
	waitFor(t, exited, "exit")
	if ctx.Err() != nil {
		t.Error("context was cancelled, but nothing was protected")
	}
}

func TestSignalInsideProtectedSectionCancelsThenForceExits(t *testing.T) {
	release := Protect()
	defer release()
	ctx, sig, exited := watch(t)
	sig <- syscall.SIGINT
	waitFor(t, ctx.Done(), "cancellation")
	select {
	case <-exited:
		t.Fatal("the first signal must not exit during protected work")
	case <-time.After(50 * time.Millisecond):
	}
	sig <- syscall.SIGINT
	waitFor(t, exited, "forced exit")
}

func TestReleaseIsIdempotent(t *testing.T) {
	outer := Protect()
	inner := Protect()
	inner()
	inner()
	if depth.Load() != 1 {
		t.Fatalf("depth = %d, want 1", depth.Load())
	}
	outer()
}
