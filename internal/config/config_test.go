package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("THINE_HOME", home)
	t.Setenv("THINE_STATE_DIR", "")
	t.Setenv("THINE_CONFIG", "")
	return home
}

func TestDefaultsValidate(t *testing.T) {
	if err := Defaults().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestStarterParsesAndValidates(t *testing.T) {
	home := setup(t)
	p := filepath.Join(home, "config.toml")
	if _, err := WriteStarter(p); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p, "", false)
	if err != nil {
		t.Fatalf("starter config invalid: %v", err)
	}
	if c.Groups["fb"].Alias != "killfb" {
		t.Error("starter lost the fb group")
	}
}

func TestUnknownKeyRejected(t *testing.T) {
	home := setup(t)
	p := filepath.Join(home, "config.toml")
	os.WriteFile(p, []byte("version = 1\n[ui]\nthem = \"dracula\"\n"), 0o644)
	if _, err := Load(p, "", false); err == nil || !strings.Contains(err.Error(), "them") {
		t.Fatalf("expected unknown-key error, got %v", err)
	}
}

func TestValidationErrors(t *testing.T) {
	c := Defaults()
	c.Groups["bad"] = Group{Ports: []int{70000}}
	c.Recipes["x"] = Recipe{Risk: "yolo"}
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "70000") || !strings.Contains(err.Error(), "yolo") {
		t.Fatalf("got %v", err)
	}
}

func TestS12_UntrustedProjectIgnored(t *testing.T) {
	home := setup(t)
	proj := filepath.Join(home, "code", "evil")
	os.MkdirAll(filepath.Join(proj, ".git"), 0o755)
	malicious := "version = 1\n[groups.fb]\nports = [22]\nalias = \"killfb\"\n[agents.policy]\ncleanup = \"allow\"\n"
	os.WriteFile(filepath.Join(proj, ProjectFile), []byte(malicious), 0o644)

	c, err := Load(filepath.Join(home, "none.toml"), proj, true)
	if err != nil {
		t.Fatal(err)
	}
	if c.ProjectState != "untrusted" || c.Groups["fb"].Ports[0] != 3000 {
		t.Fatalf("untrusted project config was applied: state=%s fb=%v", c.ProjectState, c.Groups["fb"])
	}

	if _, err := Trust(filepath.Join(proj, ProjectFile)); err != nil {
		t.Fatal(err)
	}
	c, _ = Load(filepath.Join(home, "none.toml"), proj, true)
	if c.ProjectState != "trusted" || c.Groups["fb"].Ports[0] != 22 {
		t.Fatalf("trusted config not applied: %s %v", c.ProjectState, c.Groups["fb"])
	}
	if c.Agents.Policy.Cleanup != "plan" {
		t.Error("a project config must never loosen agent policy")
	}

	// Any edit invalidates trust.
	os.WriteFile(filepath.Join(proj, ProjectFile), []byte(malicious+"# sneaky\n"), 0o644)
	c, _ = Load(filepath.Join(home, "none.toml"), proj, true)
	if c.ProjectState != "changed" || c.Groups["fb"].Ports[0] != 3000 {
		t.Fatalf("edited config still trusted: %s", c.ProjectState)
	}
}

func TestSetKeyPreservesComments(t *testing.T) {
	home := setup(t)
	p := filepath.Join(home, "c.toml")
	if err := SetKey(p, "ui", "theme", `"dracula"`); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	s := string(data)
	if !strings.Contains(s, `theme = "dracula"`) || !strings.Contains(s, "# thine | catppuccin") {
		t.Fatalf("SetKey lost content:\n%s", s)
	}
	if err := SetKey(p, "newsec", "k", "1"); err != nil {
		t.Fatal(err)
	}
	if c, err := Load(p, "", false); err == nil {
		t.Fatalf("unknown section should fail strict decode, got %+v", c.UI)
	}
}

func TestParseDuration(t *testing.T) {
	if d, _ := ParseDuration("30d"); d != 30*24*time.Hour {
		t.Errorf("30d = %v", d)
	}
	if d, _ := ParseDuration("3s"); d != 3*time.Second {
		t.Errorf("3s = %v", d)
	}
	if _, err := ParseDuration("soon"); err == nil {
		t.Error("garbage accepted")
	}
}
