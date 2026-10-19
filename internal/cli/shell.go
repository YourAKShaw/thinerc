package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/YourAKShaw/thinerc/internal/buildinfo"
	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/shellinit"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

// binRef is "thine" when the PATH entry is this very binary, else an absolute
// path, so shortcuts never pick up a different (e.g. npm-shim) thine.
func binRef() string {
	self, err := os.Executable()
	if err != nil {
		return "thine"
	}
	self, _ = filepath.EvalSymlinks(self)
	if p := platform.Which("thine"); p != "" {
		if r, _ := filepath.EvalSymlinks(p); r == self {
			return "thine"
		}
	}
	return self
}

func (a *App) initCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "init <bash|zsh|fish>",
		Short:     "Print shell integration (use: eval \"$(thine init zsh)\")",
		GroupID:   "setup",
		Args:      cobra.ExactArgs(1),
		ValidArgs: shellinit.Shells,
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := args[0]
			script, err := shellinit.Render(shell, shellinit.Build(a.Cfg), shellinit.Options{
				Bin: binRef(), Version: buildinfo.Get().Version,
				Greeting: a.Cfg.Shell.Greeting, ZshCompDir: filepath.Dir(shellinit.CompletionPath("zsh")),
			})
			if err != nil {
				return fail(ExitUsage, "E_USAGE", err.Error(), "thine init zsh")
			}
			_, err = os.Stdout.WriteString(script)
			return err
		},
	}
}

func (a *App) setupCmd() *cobra.Command {
	var shells []string
	var uninstall, yes, all, noCompletions bool
	c := &cobra.Command{
		Use:     "setup",
		Short:   "Wire thine into your shell (idempotent; backs up rc files)",
		GroupID: "setup",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if len(shells) == 0 {
				if all {
					for _, s := range shellinit.Shells {
						if platform.Have(s) {
							shells = append(shells, s)
						}
					}
				} else if s := platform.ShellName(); slices.Contains(shellinit.Shells, s) {
					shells = []string{s}
				} else {
					return fail(ExitUsage, "E_USAGE", fmt.Sprintf("couldn't detect a supported shell from $SHELL (%q)", os.Getenv("SHELL")), "thine setup --shell zsh")
				}
			}
			for _, s := range shells {
				if !slices.Contains(shellinit.Shells, s) {
					return fail(ExitUsage, "E_USAGE", fmt.Sprintf("unsupported shell %q", s), "supported: bash, zsh, fish")
				}
			}
			return a.runSetup(cmd, shells, uninstall, yes, noCompletions)
		},
	}
	f := c.Flags()
	f.StringSliceVar(&shells, "shell", nil, "shell(s) to set up: bash, zsh, fish (default: $SHELL)")
	f.BoolVar(&all, "all", false, "set up every installed supported shell")
	f.BoolVar(&uninstall, "uninstall", false, "remove thine's blocks and completion files")
	f.BoolVarP(&yes, "yes", "y", false, "apply without asking")
	f.BoolVar(&noCompletions, "no-completions", false, "skip installing completion files")
	return c
}

func (a *App) runSetup(cmd *cobra.Command, shells []string, uninstall, yes, noComp bool) error {
	t := a.T
	bin := binRef()
	type change struct {
		shell string
		plan  shellinit.Plan
	}
	var changes []change
	for _, s := range shells {
		var p shellinit.Plan
		var err error
		if uninstall {
			p, err = shellinit.PlanUninstall(s)
		} else {
			p, err = shellinit.PlanInstall(s, bin)
		}
		if err != nil {
			return fail(ExitFailure, "E_IO", err.Error(), "")
		}
		changes = append(changes, change{s, p})
	}

	verb := "Wire thine into"
	if uninstall {
		verb = "Remove thine from"
	}
	pending := 0
	for _, ch := range changes {
		title := fmt.Sprintf("%s %s %s", verb, t.Bold(ch.shell), t.Muted(platform.Tilde(ch.plan.Path)))
		if ch.plan.NoChange {
			t.Step(ui.KindOK, title)
			t.Sub(t.Muted("already up to date"))
			continue
		}
		if ch.plan.Symlink {
			t.Step(ui.KindWarn, title)
			t.Sub(t.Warnc("this rc file is a symlink (chezmoi, stow or yadm?) — add this to its source instead:"))
			for _, l := range strings.Split(strings.TrimSpace(shellinit.Block(ch.shell, bin)), "\n") {
				t.Indent(t.Code(l))
			}
			continue
		}
		pending++
		t.Step(ui.KindNeutral, title)
		for _, d := range ch.plan.Diff() {
			if strings.HasPrefix(d, "+") {
				t.Indent(t.Ok(d))
			} else {
				t.Indent(t.Bad(d))
			}
		}
	}
	if pending == 0 {
		if !uninstall && !noComp {
			a.installCompletions(cmd, shells)
		}
		return nil
	}
	if !yes {
		if !t.Interactive() {
			t.Note("re-run with --yes to apply (no terminal to confirm)")
			return silent(ExitNeedsApply)
		}
		fmt.Fprintln(t.Err)
		if !t.Confirm("Apply these changes? A backup of each file is kept.") {
			t.Note("nothing changed")
			return nil
		}
	}
	for _, ch := range changes {
		if ch.plan.NoChange || ch.plan.Symlink {
			continue
		}
		backup, err := ch.plan.Apply()
		if err != nil {
			return fail(ExitFailure, "E_IO", "couldn't update "+platform.Tilde(ch.plan.Path)+" — "+err.Error(), "")
		}
		msg := fmt.Sprintf("%s updated", platform.Tilde(ch.plan.Path))
		if backup != "" {
			msg += t.Muted(" · backup " + platform.Tilde(backup))
		}
		t.Step(ui.KindOK, msg)
	}
	if uninstall {
		for _, s := range shells {
			if p := shellinit.CompletionPath(s); p != "" {
				if os.Remove(p) == nil {
					t.Sub(t.Muted("removed " + platform.Tilde(p)))
				}
			}
		}
		t.Note("open a new shell to finish — thine's functions disappear with it")
		return nil
	}
	if !noComp {
		a.installCompletions(cmd, shells)
	}
	fmt.Fprintln(t.Out)
	t.Celebrate(t.Icons.Party + " thine is wired in — open a new shell, then try: whoport 3000")
	t.Note("reload now with: exec %s", shells[0])
	return nil
}

func (a *App) installCompletions(cmd *cobra.Command, shells []string) {
	t := a.T
	root := cmd.Root()
	for _, s := range shells {
		var buf bytes.Buffer
		var err error
		switch s {
		case "zsh":
			err = root.GenZshCompletion(&buf)
		case "bash":
			err = root.GenBashCompletionV2(&buf, true)
		case "fish":
			err = root.GenFishCompletion(&buf, true)
		}
		path := shellinit.CompletionPath(s)
		if err != nil || path == "" {
			continue
		}
		if old, rerr := os.ReadFile(path); rerr == nil && bytes.Equal(old, buf.Bytes()) {
			continue
		}
		if os.MkdirAll(filepath.Dir(path), 0o755) == nil && os.WriteFile(path, buf.Bytes(), 0o644) == nil {
			t.Sub(t.Muted("completions → " + platform.Tilde(path)))
		}
	}
}
