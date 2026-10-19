package cleanup

import (
	"context"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tempHome(t *testing.T) string {
	t.Helper()
	h, _ := filepath.EvalSymlinks(t.TempDir())
	t.Setenv("THINE_HOME", h)
	return h
}

func TestS2_GuardRejectsEscapes(t *testing.T) {
	home := tempHome(t)
	root := filepath.Join(home, "code")
	os.MkdirAll(filepath.Join(root, "app", "node_modules"), 0o755)
	os.MkdirAll(filepath.Join(home, "secret"), 0o755)
	os.Symlink(filepath.Join(home, "secret"), filepath.Join(root, "link"))
	os.MkdirAll(filepath.Join(root, "app", ".git"), 0o755)

	if err := Guard(filepath.Join(root, "app", "node_modules"), []string{root}); err != nil {
		t.Errorf("legit path refused: %v", err)
	}
	bad := []string{
		root,                                      // the root itself
		filepath.Join(root, "..", "secret"),       // dot-dot
		filepath.Join(root, "link", "x"),          // through a symlinked parent
		filepath.Join(home, ".ssh", "id_ed25519"), // deny-list
		filepath.Join(root, "app", ".git"),        // git metadata
		"/etc/passwd",
		"relative/path",
	}
	for _, b := range bad {
		if err := Guard(b, []string{root}); err == nil {
			t.Errorf("Guard admitted %s", b)
		}
	}
}

// FuzzGuard builds random trees full of symlinks and asserts that no admitted
// path canonicalises outside the root.
func FuzzGuard(f *testing.F) {
	f.Add(uint64(1), "a/b/../c")
	f.Add(uint64(7), "link/../../etc")
	f.Fuzz(func(t *testing.T, seed uint64, rel string) {
		home := tempHome(t)
		root := filepath.Join(home, "code")
		os.MkdirAll(root, 0o755)
		outside := filepath.Join(home, "outside")
		os.MkdirAll(outside, 0o755)
		r := rand.New(rand.NewPCG(seed, seed))
		for i := 0; i < 6; i++ {
			d := filepath.Join(root, string(rune('a'+r.IntN(4))), string(rune('a'+r.IntN(4))))
			os.MkdirAll(d, 0o755)
			if r.IntN(2) == 0 {
				os.Symlink([]string{outside, "/", "..", home}[r.IntN(4)], filepath.Join(d, "link"))
			}
		}
		p := filepath.Join(root, rel)
		if Guard(p, []string{root}) != nil {
			return
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(p))
		if err != nil {
			t.Fatalf("admitted a path with unresolvable parent: %s", p)
		}
		canon := filepath.Join(parent, filepath.Base(p))
		if !within(canon, root) || canon == root {
			t.Fatalf("Guard admitted %s → %s outside %s", p, canon, root)
		}
	})
}

func TestDirSizeDedupesHardlinks(t *testing.T) {
	home := tempHome(t)
	store := filepath.Join(home, "store")
	nm := filepath.Join(home, "proj", "node_modules")
	os.MkdirAll(store, 0o755)
	os.MkdirAll(nm, 0o755)
	data := make([]byte, 64<<10)
	os.WriteFile(filepath.Join(store, "pkg.js"), data, 0o644)
	os.Link(filepath.Join(store, "pkg.js"), filepath.Join(nm, "pkg.js")) // shared with the store
	os.WriteFile(filepath.Join(nm, "own.js"), data, 0o644)
	os.Link(filepath.Join(nm, "own.js"), filepath.Join(nm, "own-twin.js")) // both links inside

	s, err := DirSize(context.Background(), nm)
	if err != nil {
		t.Fatal(err)
	}
	if s.Shared < 64<<10 || s.Bytes < 64<<10 || s.Bytes > 80<<10+8<<10 {
		t.Errorf("size = %+v; want ~64 KiB own (counted once) and ~64 KiB shared", s)
	}
}

func TestArtifactRules(t *testing.T) {
	home := tempHome(t)
	root := filepath.Join(home, "code")
	old := time.Now().Add(-90 * 24 * time.Hour)
	mk := func(proj string, marker bool, stale bool) string {
		p := filepath.Join(root, proj)
		nm := filepath.Join(p, "node_modules", "dep")
		os.MkdirAll(nm, 0o755)
		os.WriteFile(filepath.Join(nm, "big.bin"), make([]byte, 2<<20), 0o644)
		if marker {
			os.WriteFile(filepath.Join(p, "package.json"), []byte("{}"), 0o644)
		}
		if stale {
			ents, _ := os.ReadDir(p)
			for _, e := range ents {
				os.Chtimes(filepath.Join(p, e.Name()), old, old)
			}
		}
		return p
	}
	mk("stale", true, true)
	mk("fresh", true, false)
	mk("nomarker", false, true)
	plan := Scan(context.Background(), Options{Only: []string{"node-modules"}, Roots: []string{root}, StaleAfter: 30 * 24 * time.Hour})
	if len(plan.Items) != 1 || !strings.HasSuffix(plan.Items[0].Title, "stale") {
		t.Fatalf("want only the stale, marked project; got %+v", plan.Items)
	}
	if plan.Items[0].IdleDays < 89 || !plan.Items[0].Undoable {
		t.Errorf("item = %+v", plan.Items[0])
	}
}

func TestS3_ApplySkipsChangedItems(t *testing.T) {
	home := tempHome(t)
	dl := filepath.Join(home, "Downloads")
	os.MkdirAll(dl, 0o755)
	old := time.Now().Add(-60 * 24 * time.Hour)
	for _, n := range []string{"a.iso", "b.deb", "notes.zip"} {
		p := filepath.Join(dl, n)
		os.WriteFile(p, make([]byte, 4096), 0o644)
		os.Chtimes(p, old, old)
	}
	plan := Scan(context.Background(), Options{Only: []string{"downloads-installers"}, StaleAfter: 30 * 24 * time.Hour})
	if len(plan.Items) != 2 {
		t.Fatalf("want 2 installers (.zip excluded), got %+v", plan.Items)
	}
	// The user touches a.iso between plan and apply.
	os.Chtimes(filepath.Join(dl, "a.iso"), time.Now(), time.Now())
	outs := Apply(context.Background(), "OPX", plan, nil, Hooks{})
	results := map[string]string{}
	for _, o := range outs {
		results[filepath.Base(o.Path)] = o.Result
	}
	if results["a.iso"] != "skipped" || results["b.deb"] != "done" {
		t.Fatalf("results = %v", results)
	}
	if _, err := os.Stat(filepath.Join(dl, "a.iso")); err != nil {
		t.Error("changed item was touched")
	}
}

func TestParseHumanSize(t *testing.T) {
	cases := map[string]int64{"2.1GB": 2_100_000_000, "512MB": 512_000_000, "101.5M": 106430464, "0B": 0, "junk": 0}
	for in, want := range cases {
		if got := ParseHumanSize(in); got != want {
			t.Errorf("%s → %d, want %d", in, got, want)
		}
	}
}
