package platform

import (
	"path/filepath"
	"testing"
)

func TestS6_SudoRegistryIsExhaustive(t *testing.T) {
	if len(SudoCallSites) != 6 {
		t.Fatalf("sudo call sites changed (%d); update plan §6.8 and this test together", len(SudoCallSites))
	}
	allowed := [][]string{{"sysctl", "-w", "vm.drop_caches=1"}, {"journalctl", "--vacuum-time=7d"}, {"kill", "-TERM", "42"}}
	for _, a := range allowed {
		if !sudoAllowed(a) {
			t.Errorf("%v should be allowed", a)
		}
	}
	for _, a := range [][]string{{"rm", "-rf", "/"}, {"sh", "-c", "id"}, {"sysctl", "-w", "vm.swappiness=1"}, {}} {
		if sudoAllowed(a) {
			t.Errorf("%v must be refused", a)
		}
	}
}

func TestPathsHonourTHINE_HOME(t *testing.T) {
	home := t.TempDir()
	t.Setenv("THINE_HOME", home)
	t.Setenv("THINE_CONFIG", "")
	t.Setenv("THINE_STATE_DIR", "")
	p := GetPaths()
	if p.ConfigFile != filepath.Join(home, ".config/thine/config.toml") || p.StateDir != filepath.Join(home, ".local/state/thine") {
		t.Errorf("unexpected paths: %+v", p)
	}
	if Tilde(filepath.Join(home, "code")) != "~/code" || ExpandHome("~/x") != filepath.Join(home, "x") {
		t.Error("tilde round-trip failed")
	}
}

func TestS9_LockIsExclusive(t *testing.T) {
	dir := t.TempDir()
	l, err := AcquireLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireLock(dir); err != ErrLocked {
		t.Errorf("second lock: got %v, want ErrLocked", err)
	}
	l.Release()
	l2, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	l2.Release()
}
