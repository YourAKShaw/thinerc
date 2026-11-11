package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/YourAKShaw/thinerc/internal/platform"
)

// These tests deliberately never pass `--stop all` or a number: the real
// process table is in play, and a broken gate must not stop a developer's
// servers. `--stop none` can't signal anything on any code path, yet still
// reveals a misplaced gate through its exit code.

// fakeSudo puts a logging sudo first on PATH and returns the log's path.
func fakeSudo(t *testing.T) (env map[string]string, log string) {
	t.Helper()
	bin := t.TempDir()
	log = filepath.Join(bin, "sudo.log")
	script := "#!/bin/sh\necho \"$@\" >> '" + log + "'\n"
	if err := os.WriteFile(filepath.Join(bin, "sudo"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return map[string]string{"THINE_AGENT": "1", "PATH": bin + string(os.PathListSeparator) + os.Getenv("PATH")}, log
}

func TestS1_AgentFreeupStopOnlyPlans(t *testing.T) {
	out, code := run(t, map[string]string{"THINE_AGENT": "1"}, "freeup", "--json", "--stop", "none")
	if code != ExitNeedsApply {
		t.Fatalf("agent --stop: exit %d, want %d", code, ExitNeedsApply)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if _, ok := doc["candidates"]; !ok || doc["next"] != "thine freeup --stop none" {
		t.Fatalf("plan is missing candidates or next: %s", out)
	}
	if _, code := run(t, map[string]string{"THINE_AGENT": "1"}, "freeup", "--stop", "none"); code != ExitNeedsApply {
		t.Fatalf("agent --stop, text mode: exit %d, want %d", code, ExitNeedsApply)
	}
}

func TestS1_AgentFreeupReportExitCode(t *testing.T) {
	out, code := run(t, map[string]string{"THINE_AGENT": "1"}, "freeup", "--json")
	var doc struct {
		Candidates []any `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	want := ExitOK
	if len(doc.Candidates) > 0 {
		want = ExitNeedsApply
	}
	if code != want {
		t.Fatalf("exit %d with %d candidates, want %d", code, len(doc.Candidates), want)
	}
}

func TestS1_AgentFreeupNeverDropsCaches(t *testing.T) {
	env, log := fakeSudo(t)
	if _, code := run(t, env, "freeup", "--drop-caches", "--yes"); code != ExitNeedsApply {
		t.Fatalf("exit %d, want %d", code, ExitNeedsApply)
	}
	if _, err := os.Stat(log); err == nil {
		t.Fatal("agent mode reached sudo")
	}
}

func TestAgentFreeupPolicyAllowReachesSudo(t *testing.T) {
	if platform.InContainer() {
		t.Skip("sudo paths are disabled inside containers")
	}
	env, log := fakeSudo(t)
	cfg := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(cfg, []byte("[agents.policy]\nfreeup = \"allow\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, code := run(t, env, "--config", cfg, "freeup", "--drop-caches", "--yes"); code != ExitOK {
		t.Fatalf("exit %d, want 0", code)
	}
	if _, err := os.Stat(log); err != nil {
		t.Fatal("the allow policy should reach sudo")
	}
}

func TestHumanFreeupUnaffected(t *testing.T) {
	if _, code := run(t, nil, "freeup", "--stop", "none"); code != ExitOK {
		t.Fatalf("--stop none: exit %d, want 0", code)
	}
	if _, code := run(t, nil, "freeup", "--json"); code != ExitOK {
		t.Fatalf("--json: exit %d, want 0", code)
	}
}
