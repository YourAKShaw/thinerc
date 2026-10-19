package shellinit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/YourAKShaw/thinerc/internal/platform"
)

const (
	beginMarker = "# >>> thine >>>"
	endMarker   = "# <<< thine <<<"
)

// RCFile is the file `thine setup` edits for a shell (plan §7.1).
func RCFile(shell string) string {
	home := platform.Home()
	switch shell {
	case "zsh":
		dir := os.Getenv("ZDOTDIR")
		if dir == "" || os.Getenv("THINE_HOME") != "" {
			dir = home
		}
		return filepath.Join(dir, ".zshrc")
	case "bash":
		if platform.IsDarwin() {
			return filepath.Join(home, ".bash_profile")
		}
		return filepath.Join(home, ".bashrc")
	case "fish":
		return filepath.Join(home, ".config", "fish", "conf.d", "thine.fish")
	}
	return ""
}

// Block is the managed snippet for a shell's rc file.
func Block(shell, bin string) string {
	var line string
	switch shell {
	case "fish":
		line = fmt.Sprintf("%s init fish | source", quoteFish(bin))
	default:
		line = fmt.Sprintf(`eval "$(%s init %s)"`, Quote(bin), shell)
	}
	return fmt.Sprintf("%s (managed by `thine setup`; remove with `thine setup --uninstall`)\n%s\n%s\n", beginMarker, line, endMarker)
}

var blockRE = regexp.MustCompile(`(?s)\n?` + regexp.QuoteMeta(beginMarker) + `.*?` + regexp.QuoteMeta(endMarker) + `\n?`)

// HasBlock reports whether content contains a thine block.
func HasBlock(content string) bool { return blockRE.MatchString(content) }

// BlockIsLast reports whether the block is the last non-blank content, which
// lets the collision guard see every alias the user defined above it.
func BlockIsLast(content string) bool {
	loc := blockRE.FindStringIndex(content)
	return loc != nil && strings.TrimSpace(content[loc[1]:]) == ""
}

// Plan describes an rc edit before it happens.
type Plan struct {
	Path     string
	Before   string
	After    string
	Exists   bool
	Symlink  bool
	NoChange bool
}

// PlanInstall computes the new content: the block is replaced in place if it
// is already last, and otherwise moved to the end of the file.
func PlanInstall(shell, bin string) (Plan, error) {
	p := Plan{Path: RCFile(shell)}
	if p.Path == "" {
		return p, fmt.Errorf("unsupported shell %q", shell)
	}
	block := Block(shell, bin)
	if shell == "fish" {
		block = "# thine — managed by `thine setup`\n" + strings.Split(block, "\n")[1] + "\n"
	}
	if fi, err := os.Lstat(p.Path); err == nil {
		p.Exists = true
		p.Symlink = fi.Mode()&os.ModeSymlink != 0
		b, err := os.ReadFile(p.Path)
		if err != nil {
			return p, err
		}
		p.Before = string(b)
	} else if !errors.Is(err, os.ErrNotExist) {
		return p, err
	}
	if shell == "fish" {
		p.After = block
	} else {
		rest := strings.TrimRight(blockRE.ReplaceAllString(p.Before, "\n"), "\n")
		if rest != "" {
			rest += "\n\n"
		}
		p.After = rest + block
	}
	p.NoChange = p.After == p.Before
	return p, nil
}

// PlanUninstall removes the block (or the fish conf.d file).
func PlanUninstall(shell string) (Plan, error) {
	p := Plan{Path: RCFile(shell)}
	b, err := os.ReadFile(p.Path)
	if errors.Is(err, os.ErrNotExist) {
		p.NoChange = true
		return p, nil
	}
	if err != nil {
		return p, err
	}
	p.Exists, p.Before = true, string(b)
	if shell == "fish" {
		p.After = ""
	} else {
		loc := blockRE.FindStringIndex(p.Before)
		if loc == nil {
			p.NoChange = true
			return p, nil
		}
		before, after := p.Before[:loc[0]], p.Before[loc[1]:]
		// Undo exactly what install added: the blank separator line.
		before = strings.TrimSuffix(before, "\n")
		if after == "" && before != "" {
			before += "\n"
		}
		p.After = before + after
	}
	p.NoChange = p.After == p.Before
	return p, nil
}

// Apply backs the file up and writes atomically (temp file + rename, mode kept).
func (p Plan) Apply() (backup string, err error) {
	if p.NoChange {
		return "", nil
	}
	if p.Symlink {
		return "", fmt.Errorf("%s is a symlink (managed by a dotfile tool?) — add the snippet to its source instead", p.Path)
	}
	if err := os.MkdirAll(filepath.Dir(p.Path), 0o755); err != nil {
		return "", err
	}
	mode := os.FileMode(0o644)
	if p.Exists {
		if fi, err := os.Stat(p.Path); err == nil {
			mode = fi.Mode().Perm()
		}
		backup = fmt.Sprintf("%s.thine-bak-%s", p.Path, time.Now().UTC().Format("20060102T150405Z"))
		if err := os.WriteFile(backup, []byte(p.Before), mode); err != nil {
			return "", err
		}
	}
	if p.After == "" && filepath.Ext(p.Path) == ".fish" {
		return backup, os.Remove(p.Path)
	}
	tmp := p.Path + ".thine-tmp"
	if err := os.WriteFile(tmp, []byte(p.After), mode); err != nil {
		return backup, err
	}
	return backup, os.Rename(tmp, p.Path)
}

// Diff renders a minimal +/- view of the change for confirmation.
func (p Plan) Diff() []string {
	old := map[string]bool{}
	for _, l := range strings.Split(p.Before, "\n") {
		old[l] = true
	}
	nw := map[string]bool{}
	for _, l := range strings.Split(p.After, "\n") {
		nw[l] = true
	}
	var out []string
	for _, l := range strings.Split(p.Before, "\n") {
		if !nw[l] && strings.TrimSpace(l) != "" {
			out = append(out, "- "+l)
		}
	}
	for _, l := range strings.Split(p.After, "\n") {
		if !old[l] && strings.TrimSpace(l) != "" {
			out = append(out, "+ "+l)
		}
	}
	return out
}

// CompletionPath is where `thine setup` installs completions for a shell.
func CompletionPath(shell string) string {
	p := platform.GetPaths()
	home := platform.Home()
	switch shell {
	case "zsh":
		return filepath.Join(p.DataDir, "zsh", "_thine")
	case "bash":
		return filepath.Join(home, ".local", "share", "bash-completion", "completions", "thine")
	case "fish":
		return filepath.Join(home, ".config", "fish", "completions", "thine.fish")
	}
	return ""
}

// Definition is a pre-existing user definition that shadows a shortcut.
type Definition struct {
	Name string
	File string
	Line int
	Text string
}

var defRE = regexp.MustCompile(`^\s*(?:alias\s+([A-Za-z0-9_.:-]+)=|function\s+([A-Za-z0-9_.:-]+)|([A-Za-z0-9_.:-]+)\s*\(\)\s*\{?)`)

// FindDefinitions scans rc files for aliases and functions with the given
// names, so doctor can explain which shortcuts were skipped and why.
func FindDefinitions(files []string, names []string) []Definition {
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	var out []Definition
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		inBlock := false
		for i, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, beginMarker) {
				inBlock = true
			}
			if strings.HasPrefix(line, endMarker) {
				inBlock = false
				continue
			}
			if inBlock || strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			m := defRE.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			name := m[1] + m[2] + m[3]
			if want[name] {
				out = append(out, Definition{Name: name, File: f, Line: i + 1, Text: strings.TrimSpace(line)})
			}
		}
	}
	return out
}
