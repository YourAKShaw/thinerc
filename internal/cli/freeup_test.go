package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/procs"
)

// Tests that read the real process table never pass `--stop all` or a number,
// because a broken gate must not stop a developer's servers. `--stop none` can't signal anything on any code path, yet still
// reveals a misplaced gate through its exit code.

// fakeSudo puts a logging sudo first on PATH and returns the log's path.
func fakeSudo(t *testing.T) (env map[string]string, log string) {
	t.Helper()
	bin := t.TempDir()
	log = filepath.Join(bin, "sudo.log")
	script := "#!/bin/sh\necho \"$@\" >> '" + log + "'\n"
	if err := os.WriteFile(filepath.Join(bin, "sudo"), []byte(script), 0o700); err != nil {
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

func TestFreeupNextNeverEchoesRawSelection(t *testing.T) {
	long := "1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22"
	cases := []struct {
		stop string
		drop bool
		n    int
		want string
	}{
		{"", false, 3, "thine freeup --stop all"},
		{"", true, 3, "thine freeup --drop-caches"},
		{"none", false, 3, "thine freeup --stop none"},
		{"all", true, 3, "thine freeup --stop all --drop-caches"},
		{"1, 3-4", false, 5, "thine freeup --stop 1,3,4"},
		{long, false, 30, "thine freeup --stop " + long},
		{"1;rm -rf ~", false, 3, "thine freeup"},
		{"$(reboot)", true, 3, "thine freeup --drop-caches"},
	}
	for _, c := range cases {
		if got := freeupNext(c.stop, c.drop, c.n); got != c.want {
			t.Errorf("freeupNext(%q, %v, %d) = %q, want %q", c.stop, c.drop, c.n, got, c.want)
		}
	}
}

// syntheticCandidate replaces the process table with one dev server that can't
// exist, and returns a count of the stops requested.
func syntheticCandidate(t *testing.T) *int {
	t.Helper()
	origProcs, origStop := freeupProcs, freeupStop
	t.Cleanup(func() { freeupProcs, freeupStop = origProcs, origStop })
	fake := procs.Proc{Identity: procs.Identity{PID: 99999999}, UID: procs.Self(), Name: "node", RSS: 1 << 30}
	freeupProcs = func() ([]procs.Proc, error) { return []procs.Proc{fake}, nil }
	stops := 0
	freeupStop = func(*App, *killTarget, time.Duration, killOpts) { stops++ }
	return &stops
}

func TestS1_AgentFreeupStopAllSignalsNothing(t *testing.T) {
	stops := syntheticCandidate(t)
	for _, sel := range []string{"all", "1"} {
		if _, code := run(t, map[string]string{"THINE_AGENT": "1"}, "freeup", "--stop", sel); code != ExitNeedsApply {
			t.Fatalf("--stop %s: exit %d, want %d", sel, code, ExitNeedsApply)
		}
	}
	if *stops != 0 {
		t.Fatalf("agent mode requested %d stops", *stops)
	}
}

func TestS1_AgentFreeupWithCandidatesExitsSix(t *testing.T) {
	stops := syntheticCandidate(t)
	out, code := run(t, map[string]string{"THINE_AGENT": "1"}, "freeup", "--json")
	var doc struct {
		Candidates []any  `json:"candidates"`
		Next       string `json:"next"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if code != ExitNeedsApply || len(doc.Candidates) != 1 || doc.Next != "thine freeup --stop all" {
		t.Fatalf("exit %d, doc %+v", code, doc)
	}
	if *stops != 0 {
		t.Fatalf("agent mode requested %d stops", *stops)
	}
}

// Proves the synthetic candidate is one freeup would really stop.
func TestHumanFreeupStopAllStopsCandidate(t *testing.T) {
	stops := syntheticCandidate(t)
	if _, code := run(t, nil, "freeup", "--stop", "all"); code != ExitOK {
		t.Fatalf("exit %d, want 0", code)
	}
	if *stops != 1 {
		t.Fatalf("stops = %d, want 1", *stops)
	}
}
