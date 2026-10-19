package recipes

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/YourAKShaw/thinerc/internal/config"
	"github.com/YourAKShaw/thinerc/internal/platform"
)

// Suggestion is one rc-file definition translated into a recipe.
type Suggestion struct {
	Name     string
	File     string
	Line     int
	Original string
	Recipe   config.Recipe
	Notes    []string
	Replaced string // set when a built-in thine verb already does this better
	Skip     string // why it can't be imported
}

// replacedBy maps hand-rolled helpers to the thine verb that supersedes them.
var replacedBy = map[string]string{
	"killport": "thine kill (kp / killport)", "kill-port": "thine kill", "kill-tcp-port-process": "thine kill",
	"get-tcp-port-data": "thine port (whoport)", "get-tcp-port-pid": "thine port --json", "kill-pid": "thine kill --pid",
	"killfb": "thine kill --group fb (killfb)", "freeup": "thine freeup", "availram": "thine freeup",
	"rameaters": "thine freeup", "cleanup": "thine cleanup", "nuke-nm": "thine nm reset",
}

var (
	aliasRE = regexp.MustCompile(`^\s*alias\s+([A-Za-z0-9_.:+-]+)=(.*)$`)
	funcRE  = regexp.MustCompile(`^\s*(?:function\s+)?([A-Za-z0-9_.:+-]+)\s*\(\)\s*\{?\s*(.*)$`)
	metaRE  = regexp.MustCompile(`[|;&<>$` + "`" + `*?(){}\[\]~!]|\bsudo\b`)
)

// Import parses rc files for aliases and simple functions.
func Import(files []string) []Suggestion {
	var out []Suggestion
	seen := map[string]bool{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		inThine := false
		for i := 0; i < len(lines); i++ {
			line := lines[i]
			if strings.HasPrefix(line, "# >>> thine >>>") {
				inThine = true
			}
			if strings.HasPrefix(line, "# <<< thine <<<") {
				inThine = false
				continue
			}
			trim := strings.TrimSpace(line)
			if inThine || strings.HasPrefix(trim, "#") || trim == "" {
				continue
			}
			var s Suggestion
			if m := aliasRE.FindStringSubmatch(line); m != nil {
				s = fromAlias(m[1], unquote(strings.TrimSpace(m[2])))
			} else if m := funcRE.FindStringSubmatch(line); m != nil && !strings.HasPrefix(trim, "if") {
				body, consumed := functionBody(lines[i:], m[2])
				i += consumed
				s = fromFunction(m[1], body)
			} else {
				continue
			}
			s.File, s.Line, s.Original = f, i+1, trim
			if seen[s.Name] {
				continue
			}
			seen[s.Name] = true
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Skip == "" && out[j].Skip != "" })
	return out
}

func unquote(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && (v[0] == '\'' && v[len(v)-1] == '\'' || v[0] == '"' && v[len(v)-1] == '"') {
		return v[1 : len(v)-1]
	}
	return v
}

// functionBody gathers a function body that may span lines; it returns the
// body and how many extra lines were consumed.
func functionBody(lines []string, firstRest string) (string, int) {
	rest := strings.TrimSpace(firstRest)
	if strings.HasSuffix(rest, "}") {
		return strings.TrimSpace(strings.TrimSuffix(rest, "}")), 0
	}
	var body []string
	if rest != "" && rest != "{" {
		body = append(body, rest)
	}
	depth := 1
	for n := 1; n < len(lines) && n < 60; n++ {
		l := lines[n]
		depth += strings.Count(l, "{") - strings.Count(l, "}")
		if depth <= 0 {
			if t := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(l), "}")); t != "" {
				body = append(body, t)
			}
			return strings.Join(body, "\n"), n
		}
		if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, "#") {
			body = append(body, t)
		}
	}
	return strings.Join(body, "\n"), 0
}

func baseSuggestion(name string) Suggestion {
	s := Suggestion{Name: strings.ToLower(name)}
	if r, ok := replacedBy[s.Name]; ok {
		s.Replaced = r
	}
	if !config.ValidName(s.Name) {
		s.Skip = "name isn't a valid recipe name (lowercase letters, digits, - or _)"
	}
	if platform.Have(name) {
		s.Notes = append(s.Notes, fmt.Sprintf("`%s` also names a program on your PATH (%s) — this alias hides it", name, platform.Which(name)))
	}
	return s
}

// shellStateRE matches commands that only make sense inside the user's own
// shell (a recipe runs in a child process and can't change its parent).
var shellStateRE = regexp.MustCompile(`^\s*(source|\.|cd|export|exec|unset|alias|eval|pushd|popd|set|declare|typeset)\s`)

func fromAlias(name, value string) Suggestion {
	s := baseSuggestion(name)
	if shellStateRE.MatchString(value + " ") {
		s.Skip = "changes your current shell (source/cd/export…) — keep it as a plain alias"
	}
	r := config.Recipe{Description: value, Alias: s.Name, Passthrough: true, Risk: guessRisk(value)}
	if metaRE.MatchString(value) {
		r.Shell = value + ` "$@"`
		r.Passthrough = false
		s.Notes = append(s.Notes, "uses shell syntax, so it becomes an unreviewed `shell` recipe")
	} else {
		r.Steps = []config.Step{{Run: strings.Fields(value)}}
	}
	if strings.Contains(value, "xclip") {
		r.OS = []string{"linux"}
		s.Notes = append(s.Notes, "xclip is Linux-only; add a [recipes."+s.Name+".darwin] override using pbcopy for macOS")
	}
	s.Recipe = r
	return s
}

func fromFunction(name, body string) Suggestion {
	s := baseSuggestion(name)
	if shellStateRE.MatchString(body + " ") {
		s.Skip = "inspects or changes your current shell — keep it as a shell function"
	}
	desc := strings.ReplaceAll(body, "\n", "; ")
	if len(desc) > 80 {
		desc = desc[:77] + "…"
	}
	s.Recipe = config.Recipe{Description: desc, Alias: s.Name, Risk: guessRisk(body), Shell: body}
	s.Notes = append(s.Notes, "functions become `shell` recipes; arguments arrive as $1, $2… exactly as before")
	if strings.Contains(body, "[[") || strings.Contains(body, "declare") || strings.Contains(body, "local ") {
		s.Notes = append(s.Notes, "uses bash-only syntax; recipes run with sh — review before enabling")
	}
	if body == "" {
		s.Skip = "empty or too complex to parse"
	}
	return s
}

var (
	readOnlyRE    = regexp.MustCompile(`^(git (status|log|branch( -vv| --sort[^ ]*)?$|shortlog|diff|show|remote -v)|tree|ls|cat|df|du|free|ps|lsof|echo|pwd|curl|whoami|declare -f|type|which)\b`)
	destructiveRE = regexp.MustCompile(`rm -rf|reset --hard|branch -D|kill -9|push (-f|--force)|clean -[a-z]*f|drop|truncate|mkfs|dd `)
)

func guessRisk(cmd string) string {
	c := strings.TrimSpace(cmd)
	switch {
	case destructiveRE.MatchString(c):
		return "destructive"
	case readOnlyRE.MatchString(c):
		return "read-only"
	}
	return "reversible"
}

// TOML renders suggestions as config the user can review and paste.
func TOML(ss []Suggestion) string {
	var b strings.Builder
	for _, s := range ss {
		if s.Skip != "" || s.Replaced != "" {
			continue
		}
		b.WriteString(RecipeTOML(s.Name, s.Recipe, fmt.Sprintf("from %s:%d", platform.Tilde(s.File), s.Line), s.Notes))
		b.WriteString("\n")
	}
	return b.String()
}

// RecipeTOML renders one recipe as a TOML table.
func RecipeTOML(name string, r config.Recipe, origin string, notes []string) string {
	var b strings.Builder
	if origin != "" {
		fmt.Fprintf(&b, "# %s\n", origin)
	}
	for _, n := range notes {
		fmt.Fprintf(&b, "# note: %s\n", n)
	}
	fmt.Fprintf(&b, "[recipes.%s]\n", name)
	fmt.Fprintf(&b, "description = %s\n", tomlString(r.Description))
	fmt.Fprintf(&b, "risk        = %s\n", tomlString(r.Risk))
	if r.Alias != "" {
		fmt.Fprintf(&b, "alias       = %s\n", tomlString(r.Alias))
	}
	if len(r.OS) > 0 {
		fmt.Fprintf(&b, "os          = [%s]\n", joinTOML(r.OS))
	}
	if r.Cwd != "" {
		fmt.Fprintf(&b, "cwd         = %s\n", tomlString(r.Cwd))
	}
	if r.Passthrough {
		b.WriteString("passthrough = true\n")
	}
	if len(r.Params) > 0 {
		var ps []string
		for _, p := range r.Params {
			f := fmt.Sprintf("name = %s, type = %s", tomlString(p.Name), tomlString(p.Type))
			if p.Default != "" {
				f += ", default = " + tomlString(p.Default)
			}
			if p.Required {
				f += ", required = true"
			}
			if len(p.Values) > 0 {
				f += ", values = [" + joinTOML(p.Values) + "]"
			}
			ps = append(ps, "{ "+f+" }")
		}
		fmt.Fprintf(&b, "params      = [%s]\n", strings.Join(ps, ", "))
	}
	if r.Shell != "" {
		fmt.Fprintf(&b, "shell       = %s\n", tomlString(r.Shell))
	}
	if len(r.Args) > 0 {
		fmt.Fprintf(&b, "args        = [%s]\n", joinTOML(r.Args))
	}
	if len(r.Steps) > 0 {
		fmt.Fprintf(&b, "steps       = %s\n", stepsTOML(r.Steps))
	}
	for _, ov := range []struct {
		key string
		o   *config.RecipeOverride
	}{{"darwin", r.Darwin}, {"linux", r.Linux}} {
		if ov.o == nil {
			continue
		}
		if ov.o.Shell != "" {
			fmt.Fprintf(&b, "%-11s = { shell = %s }\n", ov.key, tomlString(ov.o.Shell))
		} else if len(ov.o.Steps) > 0 {
			fmt.Fprintf(&b, "%-11s = { steps = %s }\n", ov.key, stepsTOML(ov.o.Steps))
		}
	}
	return b.String()
}

func stepsTOML(steps []config.Step) string {
	var out []string
	for _, s := range steps {
		f := "run = [" + joinTOML(s.Run) + "]"
		if s.ContinueOnError {
			f += ", continue_on_error = true"
		}
		out = append(out, "{ "+f+" }")
	}
	return "[" + strings.Join(out, ", ") + "]"
}

func joinTOML(in []string) string {
	q := make([]string, len(in))
	for i, s := range in {
		q[i] = tomlString(s)
	}
	return strings.Join(q, ", ")
}

// tomlString prefers a literal string; falls back to an escaped basic string.
func tomlString(s string) string {
	if !strings.ContainsAny(s, "'\n\r") {
		return "'" + s + "'"
	}
	if !strings.Contains(s, "'''") && !strings.HasPrefix(s, "\n") {
		if strings.Contains(s, "\n") {
			return "'''\n" + s + "'''"
		}
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
	return `"` + r.Replace(s) + `"`
}
