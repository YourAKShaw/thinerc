package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/YourAKShaw/thinerc/internal/recipes"
)

// shortcutConfig builds a config through the real import path, plus every
// starter recipe that takes parameters.
func shortcutConfig(t *testing.T, s *sandbox) string {
	t.Helper()
	rc := filepath.Join(s.home, "rc")
	body := "zzfn() { echo \"args:$1:$2\"; }\nalias zzal='echo al$EMPTY'\n"
	if err := os.WriteFile(rc, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var cfg strings.Builder
	cfg.WriteString(recipes.TOML(recipes.Import([]string{rc})))
	cfg.WriteString("\n[recipes.zzsep]\ndescription = 'forwards'\nrisk = 'read-only'\nalias = 'zzsep'\npassthrough = true\nsteps = [{ run = ['echo', 'P'] }]\n\n")
	var parameterised []string
	for _, st := range recipes.Starter() {
		if len(st.Recipe.Params) > 0 {
			parameterised = append(parameterised, st.Name)
			cfg.WriteString(recipes.RecipeTOML(st.Name, st.Recipe, "", nil) + "\n")
		}
	}
	sort.Strings(parameterised)
	// A new parameterised starter must get a case in the test below.
	if want := []string{"biggest", "gco-remote"}; !slices.Equal(parameterised, want) {
		t.Fatalf("starter recipes with params = %v, want %v", parameterised, want)
	}
	path := filepath.Join(s.home, "config.toml")
	if err := os.WriteFile(path, []byte(cfg.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGeneratedShortcutsForwardArgsInRealShells(t *testing.T) {
	calls := `zzfn 'two words' b; zzal x y; zzsep -- x y; gcor feature/x; biggest 'my dir'; biggest`
	loaders := map[string]string{
		"bash": `eval "$('%BIN%' init bash)"; `,
		"zsh":  `eval "$('%BIN%' init zsh)"; `,
		"fish": `'%BIN%' init fish | source; `,
	}
	for shell, loader := range loaders {
		t.Run(shell, func(t *testing.T) {
			path, err := exec.LookPath(shell)
			if err != nil {
				t.Skipf("%s not installed", shell)
			}
			s := newSandbox(t)
			s.tool(t, "git", `echo "$*" >> "$TOOL_LOG"`)
			s.env = append(s.env, "THINE_CONFIG="+shortcutConfig(t, s))
			s.cacheFile(t, "my dir", "sub", "blob")

			cmd := exec.Command(path, "-c", strings.ReplaceAll(loader, "%BIN%", thineBinary(t))+calls)
			cmd.Env, cmd.Dir = s.env, s.home
			raw, err := cmd.CombinedOutput()
			out := string(raw)
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			for _, want := range []string{"args:two words:b", "al x y", "P -- x y", "my dir/sub"} {
				if !strings.Contains(out, want) {
					t.Errorf("output lacks %q:\n%s", want, out)
				}
			}
			log, err := os.ReadFile(s.log)
			if err != nil {
				t.Fatal(err)
			}
			want := "fetch origin feature/x:feature/x\ncheckout feature/x\nbranch --set-upstream-to origin/feature/x\n"
			if string(log) != want {
				t.Errorf("git log = %q, want %q", log, want)
			}
		})
	}
}
