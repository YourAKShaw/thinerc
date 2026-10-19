// Package recipes runs typed, declarative user verbs — "much higher-level
// aliases" with parameters, risk labels and per-OS variants (plan §7.9).
// Parameters are validated and substituted only inside argv elements; a
// `shell` recipe receives them as positional arguments, never spliced into
// the script text.
package recipes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/YourAKShaw/thinerc/internal/config"
	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/ports"
)

// Resolved is a recipe ready to run on this OS.
type Resolved struct {
	Name     string
	Recipe   config.Recipe
	Steps    [][]string // argv per step (for step recipes)
	Shell    string     // script (for shell recipes)
	Args     []string   // positional args for the shell script
	Dir      string
	Env      []string
	Reviewed bool // false for shell recipes: shown as "unreviewed"
}

var placeholder = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_-]+)\s*\}\}`)

// Bind validates params (from --param k=v, then positionals in declared
// order, then defaults) and resolves the recipe for this OS.
func Bind(name string, r config.Recipe, kv map[string]string, positional []string, projectRoot string) (Resolved, []string, error) {
	if len(r.OS) > 0 && !slices.Contains(r.OS, platform.GOOS()) {
		return Resolved{}, nil, fmt.Errorf("recipe %q is not available on %s (os = %v)", name, platform.GOOS(), r.OS)
	}
	values := map[string]string{}
	pos := positional
	for _, p := range r.Params {
		v, ok := kv[p.Name]
		if !ok && len(pos) > 0 {
			v, pos, ok = pos[0], pos[1:], true
		}
		if !ok {
			if p.Required && p.Default == "" {
				return Resolved{}, nil, fmt.Errorf("missing required parameter %q", p.Name)
			}
			v = p.Default
		}
		if err := validate(p, v, projectRoot); err != nil {
			return Resolved{}, nil, fmt.Errorf("parameter %q: %w", p.Name, err)
		}
		values[p.Name] = v
	}
	for k := range kv {
		if !slices.ContainsFunc(r.Params, func(p config.Param) bool { return p.Name == k }) {
			return Resolved{}, nil, fmt.Errorf("unknown parameter %q", k)
		}
	}
	steps, shell, args := r.Steps, r.Shell, r.Args
	var ov *config.RecipeOverride
	if platform.IsDarwin() {
		ov = r.Darwin
	} else if platform.IsLinux() {
		ov = r.Linux
	}
	if ov != nil {
		if len(ov.Steps) > 0 || ov.Shell != "" {
			steps, shell, args = ov.Steps, ov.Shell, ov.Args
		}
	}
	res := Resolved{Name: name, Recipe: r, Shell: shell, Reviewed: shell == ""}
	sub := func(s string) string {
		return placeholder.ReplaceAllStringFunc(s, func(m string) string {
			key := placeholder.FindStringSubmatch(m)[1]
			if v, ok := values[key]; ok {
				return v
			}
			return m
		})
	}
	for _, st := range steps {
		argv := make([]string, len(st.Run))
		for i, a := range st.Run {
			argv[i] = sub(a)
		}
		res.Steps = append(res.Steps, argv)
	}
	for _, a := range args {
		res.Args = append(res.Args, sub(a))
	}
	extra := pos
	if r.Passthrough {
		if shell != "" {
			res.Args = append(res.Args, extra...)
		} else if len(res.Steps) > 0 {
			res.Steps[len(res.Steps)-1] = append(res.Steps[len(res.Steps)-1], extra...)
		}
		extra = nil
	}
	switch r.Cwd {
	case "project":
		res.Dir = projectRoot
	case "home":
		res.Dir = platform.Home()
	}
	for k, v := range r.Env {
		res.Env = append(res.Env, k+"="+sub(v))
	}
	return res, extra, nil
}

func validate(p config.Param, v, projectRoot string) error {
	switch p.Type {
	case "int":
		if _, err := strconv.Atoi(v); err != nil {
			return fmt.Errorf("%q is not an integer", v)
		}
	case "port":
		if _, err := ports.ParsePort(v); err != nil {
			return err
		}
	case "enum":
		if !slices.Contains(p.Values, v) {
			return fmt.Errorf("%q is not one of %s", v, strings.Join(p.Values, ", "))
		}
	case "path":
		if strings.ContainsRune(v, 0) {
			return fmt.Errorf("invalid path")
		}
	case "string":
		if strings.ContainsAny(v, "\x00\n") {
			return fmt.Errorf("must be a single line")
		}
	}
	return nil
}

// Commands renders what will run, for previews and plans.
func (r Resolved) Commands() []string {
	if r.Shell != "" {
		s := "sh -c " + strconv.Quote(r.Shell)
		if len(r.Args) > 0 {
			s += " -- " + strings.Join(quoteAll(r.Args), " ")
		}
		return []string{s}
	}
	out := make([]string, len(r.Steps))
	for i, st := range r.Steps {
		out[i] = strings.Join(quoteAll(st), " ")
	}
	return out
}

func quoteAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		if s == "" || strings.ContainsAny(s, " \t\"'$`\\|&;<>()*?") {
			out[i] = strconv.Quote(s)
		} else {
			out[i] = s
		}
	}
	return out
}

// StepResult reports one executed step.
type StepResult struct {
	Command string
	Code    int
	Err     error
}

// Run executes the recipe attached to the terminal. Steps stop at the first
// failure unless the step sets continue_on_error.
func Run(ctx context.Context, r Resolved, onStep func(i int, cmd string)) []StepResult {
	var out []StepResult
	if r.Shell != "" {
		// Positional args, never interpolated: sh -c '<script>' thine-recipe "$1" "$2"…
		argv := append([]string{"sh", "-c", r.Shell, "thine-recipe"}, r.Args...)
		if onStep != nil {
			onStep(0, r.Commands()[0])
		}
		code, err := platform.Interactive(ctx, r.Dir, r.Env, argv...)
		return append(out, StepResult{Command: r.Commands()[0], Code: code, Err: exitErr(code, err)})
	}
	for i, st := range r.Steps {
		cmd := strings.Join(quoteAll(st), " ")
		if onStep != nil {
			onStep(i, cmd)
		}
		code, err := platform.Interactive(ctx, r.Dir, r.Env, st...)
		res := StepResult{Command: cmd, Code: code, Err: exitErr(code, err)}
		out = append(out, res)
		cont := i < len(r.Recipe.Steps) && r.Recipe.Steps[i].ContinueOnError
		if res.Err != nil && !cont {
			break
		}
	}
	return out
}

func exitErr(code int, err error) error {
	if err == nil && code == 0 {
		return nil
	}
	if err != nil && code <= 0 {
		return err
	}
	return fmt.Errorf("exited %d", code)
}

// ProjectRoot is the git root (or cwd) used for cwd = "project".
func ProjectRoot(cwd string) string {
	for d := cwd; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return cwd
		}
	}
}
