// Copyright 2026 Ayush Kumar Shaw
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

var (
	builtOnce sync.Once
	builtPath string
	builtErr  error
)

// thineBinary builds the real command once, because signal handling lives in
// main and can't be reached through Execute.
func thineBinary(t *testing.T) string {
	t.Helper()
	builtOnce.Do(func() {
		dir, err := os.MkdirTemp("", "thine-e2e")
		if err != nil {
			builtErr = err
			return
		}
		builtPath = filepath.Join(dir, "thine")
		out, err := exec.Command("go", "build", "-o", builtPath, "../../cmd/thine").CombinedOutput()
		if err != nil {
			builtErr = errors.New(string(out))
		}
	})
	if builtErr != nil {
		t.Fatalf("build: %v", builtErr)
	}
	return builtPath
}

type sandbox struct {
	home, bin, log string
	env            []string
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	s := &sandbox{home: t.TempDir(), bin: t.TempDir()}
	s.log = filepath.Join(s.home, "tool.log")
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch k {
		case "CLAUDECODE", "CODEX_SANDBOX", "GEMINI_CLI", "THINE_AGENT", "CURSOR_AGENT", "THINE_STATE_DIR", "THINE_CONFIG", "PATH":
			continue
		}
		s.env = append(s.env, kv)
	}
	s.env = append(s.env, "THINE_HOME="+s.home, "HOME="+s.home, "NO_COLOR=1", "TOOL_LOG="+s.log,
		"XDG_CACHE_HOME="+filepath.Join(s.home, ".cache"), "XDG_STATE_HOME="+filepath.Join(s.home, ".state"),
		"XDG_CONFIG_HOME="+filepath.Join(s.home, ".config"), "XDG_DATA_HOME="+filepath.Join(s.home, ".share"),
		"PATH="+s.bin+string(os.PathListSeparator)+"/usr/bin:/bin")
	return s
}

func (s *sandbox) tool(t *testing.T, name, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(s.bin, name), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// cacheFile writes enough to clear the scanner's 1 MiB noise floor.
func (s *sandbox) cacheFile(t *testing.T, parts ...string) {
	t.Helper()
	p := filepath.Join(append([]string{s.home}, parts...)...)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, bytes.Repeat([]byte("x"), 2<<20), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (s *sandbox) start(t *testing.T, dir string, args ...string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()
	cmd := exec.Command(thineBinary(t), args...)
	cmd.Env, cmd.Dir = s.env, dir
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill() })
	return cmd, &output
}

func (s *sandbox) awaitLog(t *testing.T, want string) {
	t.Helper()
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if b, _ := os.ReadFile(s.log); strings.Contains(string(b), want) {
			return
		}
	}
	t.Fatalf("tool never logged %q", want)
}

func exitCode(t *testing.T, cmd *exec.Cmd, output *bytes.Buffer, within time.Duration) int {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		var ee *exec.ExitError
		if err != nil && !errors.As(err, &ee) {
			t.Fatal(err)
		}
		return cmd.ProcessState.ExitCode()
	case <-time.After(within):
		t.Fatalf("thine still running after %s; output:\n%s", within, output)
		return -1
	}
}

type historyOp struct {
	Verb, Result string
	Items        []struct {
		ID     string `json:"id"`
		Result string `json:"result"`
	}
}

func (s *sandbox) history(t *testing.T) []historyOp {
	t.Helper()
	cmd := exec.Command(thineBinary(t), "history", "--json")
	cmd.Env = s.env
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	var doc struct{ Operations []historyOp }
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("history JSON: %v\n%s", err, out)
	}
	return doc.Operations
}

// cleanupSandbox has two cache categories whose owning tools take a second.
func cleanupSandbox(t *testing.T) *sandbox {
	t.Helper()
	s := newSandbox(t)
	slow := `case "$1" in cache|pm) echo "start $0" >> "$TOOL_LOG"; sleep 1; echo "finish $0" >> "$TOOL_LOG";; esac`
	s.tool(t, "npm", slow)
	s.tool(t, "bun", slow)
	s.cacheFile(t, ".npm", "_cacache", "blob")
	s.cacheFile(t, ".bun", "install", "cache", "blob")
	return s
}

func TestS10_SIGINTDuringCleanupFinishesCurrentItem(t *testing.T) {
	s := cleanupSandbox(t)
	cmd, output := s.start(t, s.home, "cleanup", "--apply", "--yes", "--only", "npm,bun")
	s.awaitLog(t, "start")
	cmd.Process.Signal(syscall.SIGINT)
	if code := exitCode(t, cmd, output, 15*time.Second); code != ExitInterrupted {
		t.Fatalf("exit %d, want %d; output:\n%s", code, ExitInterrupted, output)
	}
	log, _ := os.ReadFile(s.log)
	if strings.Count(string(log), "start") != 1 || strings.Count(string(log), "finish") != 1 {
		t.Fatalf("exactly one tool should start and finish, log:\n%s", log)
	}
	ops := s.history(t)
	if len(ops) != 1 || ops[0].Result != "aborted" || len(ops[0].Items) != 2 {
		t.Fatalf("history = %+v", ops)
	}
	if ops[0].Items[0].Result != "done" || ops[0].Items[1].Result != "skipped" {
		t.Fatalf("items = %+v", ops[0].Items)
	}
}

func TestSecondSIGINTForceExits(t *testing.T) {
	s := newSandbox(t)
	s.tool(t, "npm", `case "$1" in cache) echo start >> "$TOOL_LOG"; sleep 5;; esac`)
	s.cacheFile(t, ".npm", "_cacache", "blob")
	cmd, output := s.start(t, s.home, "cleanup", "--apply", "--yes", "--only", "npm")
	s.awaitLog(t, "start")
	cmd.Process.Signal(syscall.SIGINT)
	time.Sleep(100 * time.Millisecond)
	cmd.Process.Signal(syscall.SIGINT)
	if code := exitCode(t, cmd, output, 2*time.Second); code != ExitInterrupted {
		t.Fatalf("exit %d, want %d; output:\n%s", code, ExitInterrupted, output)
	}
}

func TestSIGINTDuringReadOnlyScanExitsAtOnce(t *testing.T) {
	s := newSandbox(t)
	s.tool(t, "pnpm", `echo start >> "$TOOL_LOG"; sleep 5`)
	cmd, output := s.start(t, s.home, "cleanup", "--only", "pnpm")
	s.awaitLog(t, "start")
	cmd.Process.Signal(syscall.SIGINT)
	if code := exitCode(t, cmd, output, time.Second); code != ExitInterrupted {
		t.Fatalf("exit %d, want %d; output:\n%s", code, ExitInterrupted, output)
	}
}

func TestS10_SIGINTDuringNMResetRestoresTree(t *testing.T) {
	s := newSandbox(t)
	project := filepath.Join(s.home, "app")
	if err := os.MkdirAll(filepath.Join(project, "node_modules", "old"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"package.json", "package-lock.json"} {
		if err := os.WriteFile(filepath.Join(project, f), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s.tool(t, "npm", `mkdir -p node_modules/partial; echo start >> "$TOOL_LOG"; exec sleep 30`)
	cmd, output := s.start(t, project, "nm", "reset", "--yes")
	s.awaitLog(t, "start")
	cmd.Process.Signal(syscall.SIGINT)
	if code := exitCode(t, cmd, output, 15*time.Second); code != ExitInterrupted {
		t.Fatalf("exit %d, want %d; output:\n%s", code, ExitInterrupted, output)
	}
	if _, err := os.Stat(filepath.Join(project, "node_modules", "old")); err != nil {
		t.Errorf("old node_modules not restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, "node_modules", "partial")); err == nil {
		t.Error("partial install left in place")
	}
	ops := s.history(t)
	if len(ops) != 1 || ops[0].Result != "aborted" {
		t.Fatalf("history = %+v", ops)
	}
}
