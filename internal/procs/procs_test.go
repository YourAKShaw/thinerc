package procs

import (
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestParseStatWithHostileComm(t *testing.T) {
	line := "4123 (my (evil) proc) S 4100 4101 4101 0 -1 4194304 1 0 0 0 0 0 0 0 20 0 1 0 45958097 8646656 435 18446744073709551615"
	st, err := parseStat(line)
	if err != nil {
		t.Fatal(err)
	}
	if st.Comm != "my (evil) proc" || st.PPID != 4100 || st.PGID != 4101 || st.StartTime != 45958097 || st.RSSPages != 435 {
		t.Fatalf("parsed %+v", st)
	}
	if _, err := parseStat("garbage"); err == nil {
		t.Error("garbage accepted")
	}
}

func TestParsePSRow(t *testing.T) {
	p, err := parsePSRow("  4123   4100  4101   501  81234 Mon Oct 19 21:55:01 2026 /usr/local/bin/node")
	if err != nil {
		t.Fatal(err)
	}
	if p.PID != 4123 || p.PGID != 4101 || p.UID != 501 || p.RSS != 81234*1024 || p.Name != "node" {
		t.Fatalf("parsed %+v", p)
	}
	if p.Started.Year() != 2026 || p.StartTime == 0 {
		t.Errorf("start time not parsed: %v", p.Started)
	}
	p, err = parsePSRow("1 0 1 0 100 Mon Oct  5 09:01:02 2026 /Applications/Google Chrome.app/Contents/MacOS/Google Chrome")
	if err != nil || p.Name != "Google Chrome" {
		t.Fatalf("padded day / spaced path: %+v %v", p, err)
	}
}

func TestS4_ClassifyProtectsSystemAndAncestors(t *testing.T) {
	pol := Policy{DevNames: []string{"node", "python"}, Confirm: []string{"postgres"}, Ancestors: map[int]bool{777: true}, SelfUID: 1000, SelfPGID: 50}
	cases := []struct {
		p    Proc
		want Tier
	}{
		{Proc{Identity: Identity{PID: 1}, Name: "systemd"}, TierProtected},
		{Proc{Identity: Identity{PID: 777}, Name: "zsh", UID: 1000}, TierProtected},
		{Proc{Identity: Identity{PID: 900}, PGID: 50, Name: "node", UID: 1000}, TierProtected},
		{Proc{Identity: Identity{PID: 901}, Name: "gnome-shell", UID: 1000}, TierProtected},
		{Proc{Identity: Identity{PID: 902}, Name: "node", UID: 0, User: "root"}, TierConfirm},
		{Proc{Identity: Identity{PID: 903}, Name: "postgres", UID: 1000}, TierConfirm},
		{Proc{Identity: Identity{PID: 904}, Name: "node", UID: 1000}, TierDev},
		{Proc{Identity: Identity{PID: 905}, Name: "python3.12", UID: 1000}, TierDev},
		{Proc{Identity: Identity{PID: 906}, Name: "pythonista", UID: 1000}, TierConfirm},
		{Proc{Identity: Identity{PID: 907}, Name: "mystery", UID: 1000}, TierConfirm},
	}
	for _, c := range cases {
		if got, why := pol.Classify(c.p, false); got != c.want {
			t.Errorf("%s(%d): got %s (%s), want %s", c.p.Name, c.p.PID, got, why, c.want)
		}
	}
	if got, _ := pol.Classify(Proc{Identity: Identity{PID: 908}, Name: "mystery", UID: 1000}, true); got != TierDev {
		t.Error("group port should mark an unknown user process as dev")
	}
}

func TestJobScope(t *testing.T) {
	pol := Policy{Ancestors: map[int]bool{}, SelfUID: 1000, SelfPGID: 1}
	runner := Proc{Identity: Identity{PID: 100}, PGID: 100, UID: 1000, Name: "pnpm"}
	child := Proc{Identity: Identity{PID: 101}, PGID: 100, UID: 1000, Name: "node"}
	all := []Proc{runner, child, {Identity: Identity{PID: 5}, PGID: 5, UID: 1000, Name: "zsh"}}
	if got := pol.JobScope(child, all); len(got) != 2 {
		t.Errorf("job scope should include runner and child, got %d", len(got))
	}
	shellLed := []Proc{{Identity: Identity{PID: 200}, PGID: 200, UID: 1000, Name: "bash"}, {Identity: Identity{PID: 201}, PGID: 200, UID: 1000, Name: "node"}}
	if got := pol.JobScope(shellLed[1], shellLed); len(got) != 1 {
		t.Error("a shell-led group must fall back to listener scope")
	}
}

func TestS3_SignalRefusesReusedIdentity(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uses /proc")
	}
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skip("sleep unavailable")
	}
	defer cmd.Process.Kill()
	p, err := Lookup(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	stale := Identity{PID: p.PID, StartTime: p.StartTime + 1}
	if err := Signal(stale, syscall.SIGTERM); err != ErrIdentityChanged {
		t.Fatalf("stale identity: got %v, want ErrIdentityChanged", err)
	}
	if !Alive(p.Identity) {
		t.Fatal("process should still be alive after refused signal")
	}
	if err := Signal(p.Identity, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	cmd.Wait()
	if Alive(p.Identity) {
		t.Error("process survived SIGTERM")
	}
	if err := Signal(Identity{PID: 1}, syscall.SIGTERM); err != ErrPermission {
		t.Errorf("PID 1 must be refused, got %v", err)
	}
}

func TestSelfLookup(t *testing.T) {
	p, err := Lookup(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if p.UID != os.Getuid() || p.Uptime() > time.Hour || len(Ancestors()) == 0 {
		t.Errorf("unexpected self: %+v", p)
	}
}
