package shellinit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YourAKShaw/thinerc/internal/config"
)

func shortcuts() []Shortcut {
	c := config.Defaults()
	c.Recipes["gpa"] = config.Recipe{Risk: "reversible", Alias: "gpa", Steps: []config.Step{{Run: []string{"git", "push"}}}}
	c.Shortcuts.Disable = []string{"cleanup"}
	c.Shortcuts.Rename = map[string]string{"kp": "kport"}
	c.Shortcuts.Force = []string{"killfb"}
	return Build(c)
}

func TestBuildAppliesOverrides(t *testing.T) {
	names := map[string]Shortcut{}
	for _, s := range shortcuts() {
		names[s.Name] = s
	}
	if _, ok := names["cleanup"]; ok {
		t.Error("disabled shortcut emitted")
	}
	if _, ok := names["kport"]; !ok {
		t.Error("rename not applied")
	}
	if !names["killfb"].Force || strings.Join(names["killfb"].Args, " ") != "kill --group fb" {
		t.Errorf("killfb = %+v", names["killfb"])
	}
	if strings.Join(names["gpa"].Args, " ") != "run gpa --" {
		t.Errorf("recipe alias = %+v", names["gpa"])
	}
}

// runShell sources the init script in a real shell (skipped if absent) with a
// pre-existing user alias, and reports what each name resolves to.
func runShell(t *testing.T, shell, script string) string {
	t.Helper()
	if _, err := exec.LookPath(shell); err != nil {
		t.Skipf("%s not installed", shell)
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "init")
	os.WriteFile(f, []byte(script), 0o644)
	var probe string
	switch shell {
	case "fish":
		probe = "function whoport; echo mine; end; source " + f + "; type -t whoport; type -t ports; type -t killfb"
	case "zsh":
		probe = "alias whoport='echo mine'; source " + f + " 2>&1; whence -w whoport ports killfb"
	default:
		probe = "shopt -s expand_aliases; alias whoport='echo mine'; source " + f + " 2>&1; type -t whoport ports killfb"
	}
	out, err := exec.Command(shell, "-c", probe).CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\n%s\n--- script ---\n%s", shell, err, out, script)
	}
	return string(out)
}

func TestInitScriptsInRealShells(t *testing.T) {
	sc := shortcuts()
	for _, shell := range Shells {
		t.Run(shell, func(t *testing.T) {
			script, err := Render(shell, sc, Options{Bin: "/opt/my tools/thine", Version: "test"})
			if err != nil {
				t.Fatal(err)
			}
			out := runShell(t, shell, script)
			// The user's own whoport must survive (collision guard).
			if shell == "fish" {
				if !strings.HasPrefix(out, "function\nfunction\nfunction") {
					t.Errorf("fish types: %q", out)
				}
			} else if !strings.Contains(out, "alias") || strings.Count(out, "function") != 2 {
				t.Errorf("%s: want user alias kept and 2 functions, got %q", shell, out)
			}
		})
	}
}

func TestInitProducesNoOutput(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		if _, err := exec.LookPath(shell); err != nil {
			continue
		}
		script, _ := Render(shell, shortcuts(), Options{Bin: "thine", Version: "test"})
		out, err := exec.Command(shell, "-c", script).CombinedOutput()
		if err != nil || len(out) != 0 {
			t.Errorf("%s init printed %q (err %v); init must be silent", shell, out, err)
		}
	}
}

func TestSetupIdempotentAndUninstallRestores(t *testing.T) {
	home := t.TempDir()
	t.Setenv("THINE_HOME", home)
	original := "# my zshrc\nalias ll='ls -la'\n"
	rc := filepath.Join(home, ".zshrc")
	os.WriteFile(rc, []byte(original), 0o600)

	p, err := PlanInstall("zsh", "thine")
	if err != nil {
		t.Fatal(err)
	}
	backup, err := p.Apply()
	if err != nil || backup == "" {
		t.Fatalf("apply: %v (backup %q)", err, backup)
	}
	once, _ := os.ReadFile(rc)
	if !HasBlock(string(once)) || !BlockIsLast(string(once)) {
		t.Fatalf("block missing or not last:\n%s", once)
	}
	if fi, _ := os.Stat(rc); fi.Mode().Perm() != 0o600 {
		t.Error("file mode not preserved")
	}
	p2, _ := PlanInstall("zsh", "thine")
	if !p2.NoChange {
		t.Fatalf("second install should be a no-op, diff: %v", p2.Diff())
	}
	// User adds aliases after the block: re-running setup moves the block last.
	os.WriteFile(rc, append(once, []byte("alias gs='git status'\n")...), 0o600)
	p3, _ := PlanInstall("zsh", "thine")
	p3.Apply()
	moved, _ := os.ReadFile(rc)
	if !BlockIsLast(string(moved)) || strings.Count(string(moved), beginMarker) != 1 {
		t.Fatalf("block not moved to end:\n%s", moved)
	}
	// Uninstall from the first state restores the original byte for byte.
	os.WriteFile(rc, once, 0o600)
	u, _ := PlanUninstall("zsh")
	if _, err := u.Apply(); err != nil {
		t.Fatal(err)
	}
	back, _ := os.ReadFile(rc)
	if string(back) != original {
		t.Fatalf("uninstall not byte-identical:\n%q\nwant\n%q", back, original)
	}
}

func TestFindDefinitions(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, ".bashrc")
	os.WriteFile(f, []byte("alias freeup='sudo x'\nkillport() { kill -9 $(lsof -t -i:$1); }\n# alias kp=x\nfunction whatsup { :; }\n"), 0o644)
	defs := FindDefinitions([]string{f}, []string{"freeup", "killport", "kp", "whatsup", "ports"})
	got := map[string]int{}
	for _, d := range defs {
		got[d.Name] = d.Line
	}
	if got["freeup"] != 1 || got["killport"] != 2 || got["whatsup"] != 4 || got["kp"] != 0 {
		t.Errorf("definitions = %v", got)
	}
}
