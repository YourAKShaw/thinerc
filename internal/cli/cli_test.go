package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/YourAKShaw/thinerc/internal/ui"
)

// TestMain lets the test binary double as a TCP listener helper process:
// THINE_TEST_LISTEN=<port>[,ignore-term] binds the port and sleeps.
func TestMain(m *testing.M) {
	if spec := os.Getenv("THINE_TEST_LISTEN"); spec != "" {
		port, mode, _ := strings.Cut(spec, ",")
		if mode == "ignore-term" {
			signal.Ignore(syscall.SIGTERM)
		}
		ln, err := net.Listen("tcp", "127.0.0.1:"+port)
		if err != nil {
			os.Exit(3)
		}
		defer ln.Close()
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// run executes thine in-process with a throwaway home and captures stdout.
func run(t *testing.T, env map[string]string, args ...string) (string, int) {
	t.Helper()
	t.Setenv("THINE_HOME", t.TempDir())
	t.Setenv("THINE_STATE_DIR", "")
	t.Setenv("THINE_CONFIG", "")
	t.Setenv("NO_COLOR", "1")
	for _, k := range []string{"CLAUDECODE", "CODEX_SANDBOX", "GEMINI_CLI", "THINE_AGENT", "CURSOR_AGENT"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
	r, w, _ := os.Pipe()
	orig := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	code := Execute(args)
	w.Close()
	os.Stdout = orig
	out := <-done
	ui.Init(ui.Options{}) // reset the shared terminal for the next test
	return out, code
}

func freePort(t *testing.T) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func startListener(t *testing.T, port int, mode string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), fmt.Sprintf("THINE_TEST_LISTEN=%d,%s", port, mode))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // its own job, like a dev server
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	for i := 0; i < 100; i++ {
		if c, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err == nil {
			c.Close()
			return cmd
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("helper listener never came up")
	return nil
}

func TestVersionJSON(t *testing.T) {
	out, code := run(t, nil, "version", "--json")
	var v map[string]any
	if code != 0 || json.Unmarshal([]byte(out), &v) != nil || v["schema_version"] != float64(1) {
		t.Fatalf("code %d, out %q", code, out)
	}
}

func TestExitCodes(t *testing.T) {
	cases := []struct {
		args []string
		want int
	}{
		{[]string{"kill"}, ExitUsage},
		{[]string{"kill", "--group", "nope"}, ExitNotFound},
		{[]string{"port", "http"}, ExitUsage},
		{[]string{"no-such-command"}, ExitUsage},
		{[]string{"run", "missing"}, ExitNotFound},
		{[]string{"undo"}, ExitNotFound},
		{[]string{"init", "tcsh"}, ExitUsage},
	}
	for _, c := range cases {
		if _, code := run(t, nil, c.args...); code != c.want {
			t.Errorf("thine %s: exit %d, want %d", strings.Join(c.args, " "), code, c.want)
		}
	}
}

func TestPortFreeAndJSON(t *testing.T) {
	p := freePort(t)
	out, code := run(t, nil, "port", strconv.Itoa(p), "--json")
	var v struct {
		Free []int `json:"free"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &v) != nil || len(v.Free) != 1 || v.Free[0] != p {
		t.Fatalf("code %d out %s", code, out)
	}
}

func TestInitIsSilentScript(t *testing.T) {
	out, code := run(t, nil, "init", "zsh")
	if code != 0 || !strings.Contains(out, "function killfb") || strings.Contains(out, "\x1b") {
		t.Fatalf("init output wrong (code %d):\n%s", code, out)
	}
}

// TestKillFreesPortAndEscalates is the MVP acceptance test: a dev process that
// ignores SIGTERM is escalated to SIGKILL, the port is verified free, exit 0.
func TestKillFreesPortAndEscalates(t *testing.T) {
	p := freePort(t)
	startListener(t, p, "ignore-term")
	out, code := run(t, nil, "kill", strconv.Itoa(p), "--grace", "300ms", "--json")
	var v struct {
		Code    int   `json:"exit_code"`
		Free    []int `json:"ports_free"`
		Targets []struct {
			Tier, Result, Signal string
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("bad json (%v): %s", err, out)
	}
	// The helper is the test binary: not a known dev name, but user-owned and
	// non-TTY, so it is confirm-tier and needs --apply.
	if code != ExitNeedsApply {
		t.Fatalf("confirm-tier target without --apply: exit %d, want %d\n%s", code, ExitNeedsApply, out)
	}
	cfg := filepath.Join(t.TempDir(), "c.toml")
	os.WriteFile(cfg, []byte(fmt.Sprintf("version = 1\n[groups.t]\nports = [%d]\n", p)), 0o644)
	out, code = run(t, nil, "--config", cfg, "kill", "--group", "t", "--grace", "300ms", "--json")
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("bad json (%v): %s", err, out)
	}
	if code != v.Code {
		t.Errorf("process exit %d disagrees with JSON exit_code %d", code, v.Code)
	}
	if code != 0 || len(v.Free) != 1 || len(v.Targets) != 1 || v.Targets[0].Signal != "KILL" || v.Targets[0].Tier != "dev" {
		t.Fatalf("group port should be dev-tier and escalated; code %d\n%s", code, out)
	}
	if c, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", p)); err == nil {
		c.Close()
		t.Fatal("port still accepting connections")
	}
}

func TestS1_AgentModeOnlyPlans(t *testing.T) {
	home := t.TempDir()
	dl := filepath.Join(home, "Downloads")
	os.MkdirAll(dl, 0o755)
	f := filepath.Join(dl, "old.iso")
	os.WriteFile(f, bytes.Repeat([]byte("x"), 1<<20+1), 0o644)
	old := time.Now().Add(-90 * 24 * time.Hour)
	os.Chtimes(f, old, old)
	t.Setenv("THINE_HOME", home)
	_, code := runIn(t, home, map[string]string{"THINE_AGENT": "1"}, "cleanup", "--only", "downloads-installers", "--apply", "--yes")
	if code != ExitNeedsApply {
		t.Fatalf("agent cleanup --apply: exit %d, want %d", code, ExitNeedsApply)
	}
	if _, err := os.Stat(f); err != nil {
		t.Fatal("agent mode changed the filesystem")
	}
	_, code = runIn(t, home, nil, "cleanup", "--only", "downloads-installers")
	if code != ExitNeedsApply {
		t.Fatalf("non-TTY without --apply: exit %d, want %d", code, ExitNeedsApply)
	}
	if _, err := os.Stat(f); err != nil {
		t.Fatal("a plan-only run changed the filesystem")
	}
}

// runIn is run() with a fixed home (so state persists across calls).
func runIn(t *testing.T, home string, env map[string]string, args ...string) (string, int) {
	t.Helper()
	merged := map[string]string{}
	for k, v := range env {
		merged[k] = v
	}
	out, code := func() (string, int) {
		t.Setenv("THINE_HOME", home)
		t.Setenv("NO_COLOR", "1")
		for _, k := range []string{"CLAUDECODE", "THINE_AGENT", "CODEX_SANDBOX"} {
			os.Unsetenv(k)
		}
		for k, v := range merged {
			t.Setenv(k, v)
		}
		r, w, _ := os.Pipe()
		orig := os.Stdout
		os.Stdout = w
		done := make(chan string)
		go func() { b, _ := io.ReadAll(r); done <- string(b) }()
		code := Execute(args)
		w.Close()
		os.Stdout = orig
		ui.Init(ui.Options{})
		return <-done, code
	}()
	return out, code
}
