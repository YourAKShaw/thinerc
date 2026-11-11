package platform

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunDetachedUsesItsOwnProcessGroup(t *testing.T) {
	out, err := Output(context.Background(), 5*time.Second, "sh", "-c", "ps -o pgid= -p $$")
	if err != nil {
		t.Skip("ps unavailable:", err)
	}
	if got, _ := strconv.Atoi(strings.TrimSpace(out)); got != syscall.Getpgrp() {
		t.Fatalf("Run moved the child to group %d", got)
	}
	r, err := RunDetached(context.Background(), 5*time.Second, "sh", "-c", "ps -o pgid= -p $$")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := strconv.Atoi(strings.TrimSpace(string(r.Stdout))); got == syscall.Getpgrp() {
		t.Fatal("RunDetached left the child in the terminal's process group")
	}
}

func TestStreamCancelSendsInterrupt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(300*time.Millisecond, cancel)
	var out bytes.Buffer
	_, err := Stream(ctx, "", &out, "sh", "-c", `trap 'echo interrupted; exit 7' INT; sleep 2 & wait`)
	if err == nil {
		t.Error("a cancelled stream must report an error")
	}
	if !strings.Contains(out.String(), "interrupted") {
		t.Fatalf("child never saw SIGINT, output %q", out.String())
	}
}
