package recipes

import "github.com/YourAKShaw/thinerc/internal/config"

// Starter is a curated pack of everyday developer verbs, inspired by the
// aliases developers keep rewriting in their .bashrc — now typed, risk
// labelled and cross-platform.
func Starter() []struct {
	Name   string
	Recipe config.Recipe
} {
	type nr = struct {
		Name   string
		Recipe config.Recipe
	}
	return []nr{
		{"gpa", config.Recipe{Description: "Push the current branch and set its upstream", Risk: "reversible", Alias: "gpa",
			Passthrough: true, Steps: []config.Step{{Run: []string{"git", "push", "-u", "origin", "HEAD"}}}}},
		{"gpff", config.Recipe{Description: "Pull only if it fast-forwards (never creates a merge commit)", Risk: "reversible", Alias: "gpff",
			Passthrough: true, Steps: []config.Step{{Run: []string{"git", "pull", "--ff-only"}}}}},
		{"gbr", config.Recipe{Description: "Branches sorted by most recent commit", Risk: "read-only", Alias: "gbr",
			Steps: []config.Step{{Run: []string{"git", "branch", "--sort=-committerdate", "--format=%(HEAD) %(refname:short) · %(committerdate:relative) · %(subject)"}}}}},
		{"glast", config.Recipe{Description: "The last commit, one line", Risk: "read-only", Alias: "glast",
			Steps: []config.Step{{Run: []string{"git", "log", "-1", "--oneline"}}}}},
		{"gwho", config.Recipe{Description: "Authors by commit count", Risk: "read-only", Alias: "gwho",
			Steps: []config.Step{{Run: []string{"git", "shortlog", "-sn", "--all", "--no-merges"}}}}},
		{"gtrack", config.Recipe{Description: "Which remote branch each local branch tracks", Risk: "read-only", Alias: "gtrack",
			Steps: []config.Step{{Run: []string{"git", "branch", "-vv"}}}}},
		{"gco-remote", config.Recipe{Description: "Fetch a remote branch, check it out and track it", Risk: "reversible", Alias: "gcor",
			Params: []config.Param{{Name: "branch", Type: "string", Required: true}},
			Steps: []config.Step{{Run: []string{"git", "fetch", "origin", "{{branch}}:{{branch}}"}}, {Run: []string{"git", "checkout", "{{branch}}"}},
				{Run: []string{"git", "branch", "--set-upstream-to", "origin/{{branch}}"}}}}},
		{"gprune-merged", config.Recipe{Description: "Delete local branches already merged into the current one", Risk: "destructive", Alias: "gprune",
			Shell: `git branch --merged | grep -vE '^\*|^[[:space:]]*(main|master|develop)$' | xargs -r git branch -d`}},
		{"treen", config.Recipe{Description: "Directory tree without node_modules", Risk: "read-only", Alias: "treen", Passthrough: true,
			Steps: []config.Step{{Run: []string{"tree", "-I", "node_modules|.git|dist|.next"}}}}},
		{"cpwd", config.Recipe{Description: "Copy the current directory path to the clipboard", Risk: "read-only", Alias: "cpwd",
			Shell: `pwd | tr -d '\n' | (wl-copy 2>/dev/null || xclip -selection clipboard)`, Darwin: &config.RecipeOverride{Shell: `pwd | tr -d '\n' | pbcopy`}}},
		{"myip", config.Recipe{Description: "Your public IP address (asks ipinfo.io)", Risk: "read-only", Alias: "myip",
			Steps: []config.Step{{Run: []string{"curl", "-fsS", "https://ipinfo.io/ip"}}}}},
		{"diskfree", config.Recipe{Description: "Free space on every mounted disk", Risk: "read-only", Alias: "diskfree",
			Steps:  []config.Step{{Run: []string{"df", "-h", "-x", "tmpfs", "-x", "devtmpfs", "-x", "squashfs", "-x", "overlay"}}},
			Darwin: &config.RecipeOverride{Steps: []config.Step{{Run: []string{"df", "-h"}}}}}},
		{"biggest", config.Recipe{Description: "The 20 largest directories under a path", Risk: "read-only", Alias: "biggest",
			Params: []config.Param{{Name: "path", Type: "path", Default: "."}},
			Shell:  `du -xh -d 2 "$1" 2>/dev/null | sort -rh | head -20`, Args: []string{"{{path}}"}}},
	}
}
