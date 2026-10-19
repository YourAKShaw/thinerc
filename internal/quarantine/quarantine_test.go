package quarantine

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func hashTree(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	out := map[string][32]byte{}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			b, _ := os.ReadFile(p)
			rel, _ := filepath.Rel(root, p)
			out[rel] = sha256.Sum256(b)
		}
		return nil
	})
	return out
}

func TestMoveRestoreIsIdentity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("THINE_HOME", home)
	proj := filepath.Join(home, "code", "shop", "node_modules")
	os.MkdirAll(filepath.Join(proj, "left-pad"), 0o755)
	os.WriteFile(filepath.Join(proj, "left-pad", "index.js"), []byte("module.exports = 1"), 0o644)
	os.Symlink("../left-pad", filepath.Join(proj, ".bin-link"))
	before := hashTree(t, proj)

	if _, err := Move("OP1", "cleanup", "nm", proj, 123); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(proj); !os.IsNotExist(err) {
		t.Fatal("original still present after quarantine")
	}
	sums, _ := List()
	if len(sums) != 1 || sums[0].Bytes != 123 || sums[0].Items != 1 {
		t.Fatalf("list = %+v", sums)
	}
	res, err := Restore("OP1", nil)
	if err != nil || len(res) != 1 || res[0].Conflict {
		t.Fatalf("restore: %+v %v", res, err)
	}
	after := hashTree(t, proj)
	if len(after) != len(before) || after["left-pad/index.js"] != before["left-pad/index.js"] {
		t.Fatal("apply ∘ undo is not the identity")
	}
	if l, err := os.Readlink(filepath.Join(proj, ".bin-link")); err != nil || l != "../left-pad" {
		t.Error("symlink not preserved as a link")
	}
	if _, err := Restore("OP1", nil); err != nil {
		t.Errorf("second restore should be a no-op, got %v", err)
	}
}

func TestRestoreNeverOverwrites(t *testing.T) {
	home := t.TempDir()
	t.Setenv("THINE_HOME", home)
	f := filepath.Join(home, "Downloads", "old.iso")
	os.MkdirAll(filepath.Dir(f), 0o755)
	os.WriteFile(f, []byte("old"), 0o644)
	Move("OP2", "cleanup", "iso", f, 3)
	os.WriteFile(f, []byte("new"), 0o644) // user re-downloaded it
	res, err := Restore("OP2", nil)
	if err != nil || !res[0].Conflict || res[0].To != f+".thine-restored-1" {
		t.Fatalf("conflict handling: %+v %v", res, err)
	}
	if b, _ := os.ReadFile(f); string(b) != "new" {
		t.Error("restore overwrote the newer file")
	}
}

func TestPurgeSelected(t *testing.T) {
	home := t.TempDir()
	t.Setenv("THINE_HOME", home)
	for _, op := range []string{"OPA", "OPB"} {
		p := filepath.Join(home, op+".txt")
		os.WriteFile(p, []byte(op), 0o644)
		Move(op, "cleanup", op, p, 10)
	}
	freed, n, err := Purge(func(s Summary) bool { return s.OpID == "OPA" && time.Since(s.Created) >= 0 })
	if err != nil || n != 1 || freed != 10 {
		t.Fatalf("purge: %d %d %v", freed, n, err)
	}
	sums, _ := List()
	if len(sums) != 1 || sums[0].OpID != "OPB" {
		t.Fatalf("remaining = %+v", sums)
	}
}
