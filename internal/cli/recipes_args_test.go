package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const separatorRecipes = `
[recipes.par]
description = "one param"
risk = "read-only"
params = [{ name = "branch", type = "string", required = true }]
steps = [{ run = ["echo", "par:{{branch}}"] }]

[recipes.pass]
description = "forwards"
risk = "read-only"
passthrough = true
steps = [{ run = ["echo", "P"] }]
`

func TestRunConsumesOnlyLeadingSeparator(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(cfg, []byte(separatorRecipes), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"par", "--", "main"}, "par:main"},
		{[]string{"par", "main"}, "par:main"},
		{[]string{"pass", "--", "x", "y"}, "P x y"},
		{[]string{"pass", "x", "--", "y"}, "P x -- y"},
		{[]string{"pass", "--", "--", "x"}, "P -- x"},
	}
	for _, c := range cases {
		out, code := run(t, nil, append([]string{"--config", cfg, "run"}, c.args...)...)
		if code != ExitOK || strings.TrimSpace(out) != c.want {
			t.Errorf("run %v: exit %d, output %q, want %q", c.args, code, out, c.want)
		}
	}
}
