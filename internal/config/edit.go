package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Starter is the annotated config written by `thine config init`.
const Starter = `# thine configuration — https://github.com/YourAKShaw/.thinerc/blob/main/docs/configuration.md
# Validate with: thine config validate
version = 1

[ui]
theme      = "thine"   # thine | catppuccin-mocha | catppuccin-latte | tokyo-night | dracula | nord | gruvbox | rose-pine | synthwave
icons      = "emoji"   # emoji | nerd (needs a Nerd Font: thine fonts) | ascii
animations = true      # spinners, splash, confetti (always off for pipes, CI and NO_COLOR)
splash     = true      # animated logo when you run bare "thine"

[shell]
greeting = false       # one-line hello in every new interactive shell (costs ~5 ms)

[shortcuts]
disable = []           # e.g. ["cleanup"] if you already own that name
rename  = {}           # e.g. { kp = "kport" }
force   = []           # define even if you already have an alias with that name

[kill]
grace = "3s"           # SIGTERM → wait → SIGKILL (dev processes only)
scope = "auto"         # auto | listener | job

[groups.fb]            # killfb → frees the frontend + backend ports
ports = [3000, 3001]
alias = "killfb"

# [groups.db]
# ports = [5432, 6379]
# alias = "killdb"

[cleanup]
stale_after = "30d"    # node_modules / build dirs idle longer than this are candidates
retention   = "7d"     # quarantined items are restorable with "thine undo" for this long
# roots = ["~/code", "~/projects"]

# A recipe is a typed, reviewable alias. Run with: thine run <name>  (or its alias)
# [recipes.gpa]
# description = "Push the current branch and set upstream"
# risk        = "reversible"
# alias       = "gpa"
# steps       = [{ run = ["git", "push", "-u", "origin", "HEAD"] }]
`

// WriteStarter creates the global config if it does not exist.
func WriteStarter(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(Starter), 0o644)
}

// SetKey sets `key = value` inside [section] with a line-level edit so the
// user's comments and layout survive. value must already be a TOML literal.
func SetKey(path, section, key, value string) error {
	if _, err := WriteStarter(path); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	header := "[" + section + "]"
	in, sectionAt, insertAt := false, -1, -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") {
			if in {
				break
			}
			if t == header || strings.HasPrefix(t, header+" ") || strings.HasPrefix(t, header+"#") {
				in, sectionAt, insertAt = true, i, i+1
			}
			continue
		}
		if !in {
			continue
		}
		if t != "" && !strings.HasPrefix(t, "#") {
			insertAt = i + 1
		}
		if k, _, ok := strings.Cut(t, "="); ok && strings.TrimSpace(k) == key {
			comment := ""
			if idx := strings.Index(l, "#"); idx > 0 && strings.Count(l[:idx], `"`)%2 == 0 {
				comment = "   " + strings.TrimSpace(l[idx:])
			}
			lines[i] = fmt.Sprintf("%s = %s%s", key, value, comment)
			return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
		}
	}
	line := fmt.Sprintf("%s = %s", key, value)
	if sectionAt < 0 {
		lines = append(lines, "", header, line)
	} else {
		lines = append(lines[:insertAt], append([]string{line}, lines[insertAt:]...)...)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}
