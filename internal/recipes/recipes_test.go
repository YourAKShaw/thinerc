package recipes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/YourAKShaw/thinerc/internal/config"
)

func TestBindSubstitutesOnlyInsideArgv(t *testing.T) {
	r := config.Recipe{Risk: "reversible",
		Params: []config.Param{{Name: "env", Type: "enum", Values: []string{"dev", "test"}, Default: "dev"}, {Name: "port", Type: "port", Default: "3000"}},
		Steps:  []config.Step{{Run: []string{"echo", "app_{{env}}", "{{ port }}"}}}}
	res, extra, err := Bind("x", r, map[string]string{"env": "test"}, []string{"8080", "leftover"}, "/p")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Steps[0], " ") != "echo app_test 8080" || len(extra) != 1 {
		t.Fatalf("steps = %v extra = %v", res.Steps, extra)
	}
	if _, _, err := Bind("x", r, map[string]string{"env": "prod"}, nil, ""); err == nil {
		t.Error("enum violation accepted")
	}
	if _, _, err := Bind("x", r, map[string]string{"port": "99999"}, nil, ""); err == nil {
		t.Error("invalid port accepted")
	}
	if _, _, err := Bind("x", r, map[string]string{"nope": "1"}, nil, ""); err == nil {
		t.Error("unknown param accepted")
	}
}

func TestS5_ShellRecipeParamsArePositional(t *testing.T) {
	dir := t.TempDir()
	pwned := filepath.Join(dir, "pwned")
	r := config.Recipe{Risk: "read-only", Params: []config.Param{{Name: "name", Type: "string"}},
		Shell: `printf '%s' "$1" > /dev/null`, Args: []string{"{{name}}"}}
	res, _, err := Bind("x", r, map[string]string{"name": "$(touch " + pwned + ")"}, nil, dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Shell, "touch") {
		t.Fatal("parameter was spliced into the script text")
	}
	out := Run(context.Background(), res, nil)
	if out[0].Err != nil {
		t.Fatalf("run: %v", out[0].Err)
	}
	if _, err := os.Stat(pwned); err == nil {
		t.Fatal("command injection: the parameter was executed")
	}
}

func TestRunStopsAtFirstFailure(t *testing.T) {
	r := config.Recipe{Risk: "read-only", Steps: []config.Step{{Run: []string{"false"}}, {Run: []string{"true"}}}}
	res, _, _ := Bind("x", r, nil, nil, "")
	out := Run(context.Background(), res, nil)
	if len(out) != 1 || out[0].Err == nil {
		t.Fatalf("want stop after the failing step, got %+v", out)
	}
	r.Steps[0].ContinueOnError = true
	res, _, _ = Bind("x", r, nil, nil, "")
	if out := Run(context.Background(), res, nil); len(out) != 2 {
		t.Fatalf("continue_on_error ignored: %+v", out)
	}
}

const sampleRC = `# aliases
alias ni="npm i"
alias reload-zshrc="source ~/.zshrc"
fundef() {
    declare -f $1
}
alias gpa="git push -u origin HEAD"
alias gr-hard="git reset --hard HEAD~1"
alias treenc="tree -I node_modules | xclip -selection clipboard"
alias gh="git log --oneline"
alias freeup='sudo -n /usr/local/sbin/freeup-cache'
alias BadName="echo x"
killport() { kill -9 $(lsof -t -i:$1); }
gb-da-except() {
    git branch | grep -v $1 | xargs git branch -D
}
`

func TestImportFromRC(t *testing.T) {
	f := filepath.Join(t.TempDir(), ".bashrc")
	os.WriteFile(f, []byte(sampleRC), 0o644)
	ss := Import([]string{f})
	by := map[string]Suggestion{}
	for _, s := range ss {
		by[s.Name] = s
	}
	if r := by["gpa"].Recipe; len(r.Steps) != 1 || strings.Join(r.Steps[0].Run, " ") != "git push -u origin HEAD" || r.Risk != "reversible" {
		t.Errorf("gpa = %+v", r)
	}
	if by["fundef"].Skip == "" {
		t.Error("declare -f can't see the user's functions from a recipe")
	}
	if by["reload-zshrc"].Skip == "" {
		t.Error("source aliases can't be recipes")
	}
	if by["gr-hard"].Recipe.Risk != "destructive" {
		t.Error("reset --hard must be destructive")
	}
	if r := by["treenc"].Recipe; r.Shell == "" || r.OS[0] != "linux" {
		t.Errorf("pipe alias should be a linux shell recipe: %+v", r)
	}
	if by["freeup"].Replaced == "" || by["killport"].Replaced == "" {
		t.Error("freeup/killport should be marked as replaced by thine verbs")
	}
	if by["badname"].Skip != "" {
		t.Errorf("uppercase names are lowercased, not skipped: %+v", by["badname"])
	}
	if !strings.Contains(by["gb-da-except"].Recipe.Shell, "xargs git branch -D") || by["gb-da-except"].Recipe.Risk != "destructive" {
		t.Errorf("multi-line function: %+v", by["gb-da-except"].Recipe)
	}

	// The generated TOML must parse back through the strict config decoder.
	doc := "version = 1\n" + TOML(ss)
	var c config.Config
	d := toml.NewDecoder(strings.NewReader(doc))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		t.Fatalf("generated TOML is invalid: %v\n%s", err, doc)
	}
	if c.Recipes["gpa"].Alias != "gpa" || c.Recipes["killport"].Risk != "" {
		t.Errorf("round trip lost data (or imported a replaced helper): %+v", c.Recipes)
	}
	if err := (&config.Config{Version: 1, Recipes: c.Recipes}).Validate(); err != nil && !strings.Contains(err.Error(), "ui.icons") {
		t.Errorf("imported recipes fail validation: %v", err)
	}
}

func TestStarterRecipesValidate(t *testing.T) {
	c := config.Defaults()
	var doc strings.Builder
	doc.WriteString("version = 1\n")
	for _, s := range Starter() {
		c.Recipes[s.Name] = s.Recipe
		doc.WriteString(RecipeTOML(s.Name, s.Recipe, "", nil))
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	var back config.Config
	d := toml.NewDecoder(strings.NewReader(doc.String()))
	d.DisallowUnknownFields()
	if err := d.Decode(&back); err != nil {
		t.Fatalf("starter TOML invalid: %v\n%s", err, doc.String())
	}
	for _, s := range Starter() {
		got, want := back.Recipes[s.Name], s.Recipe
		if show(got) != show(want) {
			t.Errorf("%s did not round-trip:\n got %+v\nwant %+v", s.Name, got, want)
		}
	}
}

// show renders a recipe with its overrides dereferenced, for comparison.
func show(r config.Recipe) string {
	d, l := r.Darwin, r.Linux
	r.Darwin, r.Linux = nil, nil
	out := fmt.Sprintf("%+v", r)
	if d != nil {
		out += fmt.Sprintf(" darwin=%+v", *d)
	}
	if l != nil {
		out += fmt.Sprintf(" linux=%+v", *l)
	}
	return out
}
