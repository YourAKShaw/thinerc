package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/YourAKShaw/thinerc/internal/config"
	"github.com/YourAKShaw/thinerc/internal/oplog"
	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/recipes"
	"github.com/YourAKShaw/thinerc/internal/shellinit"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

func (a *App) riskBadgeRecipe(r string) string {
	t := a.T
	switch r {
	case "read-only":
		return t.Badge("read-only", t.Pal.Accent)
	case "reversible":
		return t.Badge("reversible", t.Pal.Success)
	case "regenerable":
		return t.Badge("regenerable", t.Pal.Info)
	}
	return t.Badge("destructive", t.Pal.Error)
}

func (a *App) runCmd() *cobra.Command {
	var params []string
	var yes, dryRun bool
	c := &cobra.Command{
		Use:     "run <recipe> [args...] [-- extra args]",
		Short:   "Run one of your recipes (typed, risk-labelled aliases)",
		GroupID: "recipes",
		Args:    cobra.MinimumNArgs(1),
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			var names []string
			for n, r := range a.Cfg.Recipes {
				names = append(names, n+"\t"+r.Description)
			}
			sort.Strings(names)
			return names, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			r, ok := a.Cfg.Recipes[name]
			if !ok {
				hint := "thine recipes ls"
				if a.Cfg.ProjectState == "untrusted" || a.Cfg.ProjectState == "changed" {
					hint = "this project's .thinerc.toml isn't trusted — review it, then `thine trust`"
				}
				return fail(ExitNotFound, "E_NOT_FOUND", fmt.Sprintf("no recipe named %q", name), hint)
			}
			kv := map[string]string{}
			for _, p := range params {
				k, v, ok := strings.Cut(p, "=")
				if !ok {
					return fail(ExitUsage, "E_USAGE", fmt.Sprintf("--param %q must look like key=value", p), "")
				}
				kv[k] = v
			}
			cwd, _ := os.Getwd()
			res, extra, err := recipes.Bind(name, r, kv, args[1:], recipes.ProjectRoot(cwd))
			if err != nil {
				return fail(ExitUsage, "E_USAGE", err.Error(), "thine recipes show "+name)
			}
			if len(extra) > 0 {
				hint := "set passthrough = true on the recipe to forward them"
				if strings.HasPrefix(extra[0], "-") {
					hint = "thine's own flags go before the recipe name: thine run --param k=v " + name
				}
				return fail(ExitUsage, "E_USAGE", fmt.Sprintf("unexpected arguments: %s", strings.Join(extra, " ")), hint)
			}
			return a.runRecipe(cmd, res, yes, dryRun)
		},
	}
	c.Flags().SetInterspersed(false) // everything after the recipe name belongs to the recipe
	c.Flags().StringArrayVarP(&params, "param", "p", nil, "set a parameter: --param env=test")
	c.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation for regenerable/destructive recipes")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "show what would run")
	return c
}

func (a *App) runRecipe(cmd *cobra.Command, res recipes.Resolved, yes, dryRun bool) error {
	t := a.T
	risk := res.Recipe.Risk
	head := fmt.Sprintf("%s %s", t.Bold(res.Name), a.riskBadgeRecipe(risk))
	if !res.Reviewed {
		head += " " + t.Badge("shell", t.Pal.Warning)
	}
	needsGate := risk == "regenerable" || risk == "destructive"
	quiet := !needsGate && !dryRun
	if !quiet {
		t.StepTo(t.Err, ui.KindNeutral, head)
		for _, c := range res.Commands() {
			t.SubTo(t.Err, t.Code(c))
		}
	}
	if dryRun {
		return nil
	}
	if a.isAgent() && risk != "read-only" && a.Cfg.Agents.Policy.Recipes != "allow" {
		return fail(ExitNeedsApply, "E_AGENT_POLICY", "agent mode: only read-only recipes run directly", "ask the user to run `thine run "+res.Name+"`")
	}
	switch {
	case risk == "destructive" && !yes:
		if !t.ConfirmTyped("This recipe is destructive and can't be undone.", "run "+res.Name) {
			return silent(ExitNeedsApply)
		}
	case risk == "regenerable" && !yes:
		if !t.Confirm("Run it?") {
			return silent(ExitNeedsApply)
		}
	}
	var opID string
	if risk != "read-only" {
		opID = oplog.NewID()
		oplog.Append(oplog.Record{OpID: opID, Phase: "intent", Verb: "run:" + res.Name, Argv: a.argv, Caller: a.Caller,
			Items: []oplog.Item{{ID: res.Name, Action: "exec", Target: strings.Join(res.Commands(), " && ")}}})
	}
	results := recipes.Run(cmd.Context(), res, func(i int, c string) {
		if len(res.Commands()) > 1 {
			t.StepTo(t.Err, ui.KindInfo, t.Muted(fmt.Sprintf("[%d/%d] ", i+1, len(res.Commands())))+t.Code(c))
		}
	})
	last := results[len(results)-1]
	if opID != "" {
		result := "completed"
		if last.Err != nil {
			result = "failed"
		}
		oplog.Append(oplog.Record{OpID: opID, Phase: "outcome", Verb: "run:" + res.Name, Caller: a.Caller, Result: result})
	}
	if last.Err != nil {
		code := last.Code
		if code <= 0 {
			code = ExitFailure
		}
		if !quiet || len(results) > 1 {
			t.StepTo(t.Err, ui.KindFail, t.Bad(res.Name+" failed")+t.Muted(" — "+last.Command+": "+last.Err.Error()))
		}
		return silent(code)
	}
	return nil
}

func (a *App) recipesCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "recipes",
		Aliases: []string{"recipe"},
		Short:   "List, inspect and install recipes",
		GroupID: "recipes",
		Args:    cobra.NoArgs,
		RunE:    func(*cobra.Command, []string) error { return a.recipesList() },
	}
	c.AddCommand(&cobra.Command{
		Use: "ls", Short: "List your recipes", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error { return a.recipesList() },
	})
	c.AddCommand(&cobra.Command{
		Use: "show <recipe>", Short: "Show a recipe as TOML and what it runs on this OS", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			r, ok := a.Cfg.Recipes[args[0]]
			if !ok {
				return fail(ExitNotFound, "E_NOT_FOUND", fmt.Sprintf("no recipe named %q", args[0]), "thine recipes ls")
			}
			fmt.Fprint(a.T.Out, a.T.Text(recipes.RecipeTOML(args[0], r, "", nil)))
			if res, _, err := recipes.Bind(args[0], r, nil, nil, "."); err == nil {
				a.T.Step(ui.KindInfo, a.T.Muted("on "+platform.GOOS()+" this runs:"))
				for _, cmd := range res.Commands() {
					a.T.Sub(a.T.Code(cmd))
				}
			}
			return nil
		},
	})
	var write bool
	starter := &cobra.Command{
		Use: "starter", Short: "A curated pack of everyday git and shell recipes (add with --write)", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			var b strings.Builder
			t := a.T
			t.Step(ui.KindNeutral, t.Bold("Starter recipes")+t.Muted(" · typed, risk-labelled versions of aliases everyone hand-rolls"))
			for _, s := range recipes.Starter() {
				state := ""
				if _, exists := a.Cfg.Recipes[s.Name]; exists {
					state = t.Muted(" (already yours)")
				} else {
					b.WriteString(recipes.RecipeTOML(s.Name, s.Recipe, "thine starter pack", nil) + "\n")
				}
				fmt.Fprintf(t.Out, "  %s %s %s%s\n", ui.PadRight(t.Accent(t.Bold(s.Recipe.Alias)), 10), ui.PadRight(a.riskBadgeRecipe(s.Recipe.Risk), 15), t.Text(s.Recipe.Description), state)
			}
			if !write {
				t.Sub(t.Muted("add them all with ") + t.Code("thine recipes starter --write"))
				return nil
			}
			return a.appendToConfig(b.String(), "starter recipes")
		},
	}
	starter.Flags().BoolVar(&write, "write", false, "append the pack to your global config (after confirmation)")
	c.AddCommand(starter)
	return c
}

func (a *App) recipesList() error {
	t := a.T
	names := make([]string, 0, len(a.Cfg.Recipes))
	for n := range a.Cfg.Recipes {
		names = append(names, n)
	}
	sort.Strings(names)
	if a.json() {
		rows := []map[string]any{}
		for _, n := range names {
			r := a.Cfg.Recipes[n]
			rows = append(rows, map[string]any{"name": n, "alias": r.Alias, "risk": r.Risk, "description": r.Description, "reviewed": r.Shell == ""})
		}
		return a.emitJSON(map[string]any{"recipes": rows})
	}
	if len(names) == 0 {
		t.Step(ui.KindInfo, t.Muted("no recipes yet"))
		t.Sub(t.Muted("start with ") + t.Code("thine recipes starter") + t.Muted(" or turn your aliases into recipes: ") + t.Code("thine import"))
		return nil
	}
	t.Step(ui.KindNeutral, t.Bold(plural(len(names), "recipe", "recipes")))
	for _, n := range names {
		r := a.Cfg.Recipes[n]
		alias := ""
		if r.Alias != "" && r.Alias != n {
			alias = t.Muted(" (" + r.Alias + ")")
		}
		shell := ""
		if r.Shell != "" {
			shell = " " + t.Badge("shell", t.Pal.Warning)
		}
		fmt.Fprintf(t.Out, "  %s %s %s%s\n", ui.PadRight(t.Accent(t.Bold(n))+alias, 22), ui.PadRight(a.riskBadgeRecipe(r.Risk), 15), t.Text(ui.Truncate(ui.Sanitize(r.Description, 200), max(20, t.Width()-42))), shell)
	}
	return nil
}

func (a *App) importCmd() *cobra.Command {
	var write, toStdout bool
	c := &cobra.Command{
		Use:     "import [rc-file...]",
		Short:   "Turn the aliases and functions in your rc files into typed recipes",
		GroupID: "recipes",
		Example: "  thine import\n  thine import ~/.bashrc --write\n  thine import --toml > recipes.toml",
		RunE: func(_ *cobra.Command, args []string) error {
			files := args
			if len(files) == 0 {
				h := platform.Home()
				for _, f := range []string{".bashrc", ".bash_aliases", ".zshrc", ".zsh_aliases", ".bash_profile"} {
					if _, err := os.Stat(filepath.Join(h, f)); err == nil {
						files = append(files, filepath.Join(h, f))
					}
				}
			}
			ss := recipes.Import(files)
			var fresh []recipes.Suggestion
			for _, s := range ss {
				if _, exists := a.Cfg.Recipes[s.Name]; !exists {
					fresh = append(fresh, s)
				}
			}
			toml := recipes.TOML(fresh)
			if toStdout || a.json() {
				fmt.Fprint(a.T.Out, toml)
				return nil
			}
			a.printImport(ss, files)
			if !write {
				if toml != "" {
					a.T.Sub(a.T.Muted("review the TOML with ") + a.T.Code("thine import --toml") + a.T.Muted(", or add it with ") + a.T.Code("thine import --write"))
				}
				return nil
			}
			if toml == "" {
				a.T.Note("nothing new to import")
				return nil
			}
			return a.appendToConfig(toml, "imported recipes")
		},
	}
	c.Flags().BoolVar(&write, "write", false, "append the suggested recipes to your global config (after confirmation)")
	c.Flags().BoolVar(&toStdout, "toml", false, "print the suggested TOML only")
	return c
}

func (a *App) printImport(ss []recipes.Suggestion, files []string) {
	t := a.T
	short := make([]string, len(files))
	for i, f := range files {
		short[i] = platform.Tilde(f)
	}
	t.Step(ui.KindNeutral, t.Bold(fmt.Sprintf("Found %s", plural(len(ss), "definition", "definitions")))+t.Muted(" in "+strings.Join(short, ", ")))
	for _, s := range ss {
		name := ui.PadRight(t.Accent(t.Bold(s.Name)), 22)
		src := t.Muted(fmt.Sprintf("%s:%d", filepath.Base(s.File), s.Line))
		switch {
		case s.Replaced != "":
			fmt.Fprintf(t.Out, "  %s %s %s %s\n", name, ui.PadRight(t.Badge("built in", t.Pal.Brand[0]), 15), t.Text("→ "+s.Replaced), src)
		case s.Skip != "":
			fmt.Fprintf(t.Out, "  %s %s %s %s\n", name, ui.PadRight(t.Badge("skipped", t.Pal.Subtle), 15), t.Muted(s.Skip), src)
		default:
			body := s.Recipe.Shell
			if body == "" && len(s.Recipe.Steps) > 0 {
				body = strings.Join(s.Recipe.Steps[0].Run, " ")
			}
			fmt.Fprintf(t.Out, "  %s %s %s %s\n", name, ui.PadRight(a.riskBadgeRecipe(s.Recipe.Risk), 15),
				t.Text(ui.Truncate(strings.ReplaceAll(ui.Sanitize(body, 300), "\n", "; "), max(20, t.Width()-60))), src)
		}
		if s.Replaced != "" || s.Skip != "" {
			continue
		}
		for _, n := range s.Notes {
			fmt.Fprintf(t.Out, "  %s %s\n", ui.Repeat(" ", 22), t.Muted("↳ "+n))
		}
	}
}

// appendToConfig adds TOML to the global config with a backup, then validates;
// an invalid result is rolled back so the user's config never breaks.
func (a *App) appendToConfig(snippet, what string) error {
	t := a.T
	path := a.Cfg.GlobalPath
	if _, err := config.WriteStarter(path); err != nil {
		return fail(ExitFailure, "E_IO", err.Error(), "")
	}
	before, _ := os.ReadFile(path)
	if !t.Interactive() {
		t.Note("no terminal to confirm — nothing written")
		return silent(ExitNeedsApply)
	}
	if !t.Confirm(fmt.Sprintf("Append %s to %s?", what, platform.Tilde(path))) {
		return nil
	}
	backup := path + ".thine-bak"
	os.WriteFile(backup, before, 0o644)
	content := strings.TrimRight(string(before), "\n") + "\n\n" + snippet
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fail(ExitFailure, "E_IO", err.Error(), "")
	}
	if _, err := config.Load(path, "", false); err != nil {
		os.WriteFile(path, before, 0o644)
		return fail(ExitFailure, "E_CONFIG_INVALID", "the result didn't validate, so your config was left unchanged — "+err.Error(), "")
	}
	t.Step(ui.KindOK, t.Ok("added")+" "+what+t.Muted(" · backup "+platform.Tilde(backup)))
	t.Sub(t.Muted("open a new shell (or `exec $SHELL`) to get the aliases"))
	return nil
}

// ── trust ──────────────────────────────────────────────────────────────────

func (a *App) trustCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:     "trust [path]",
		Short:   "Review and trust a project's .thinerc.toml (pinned by SHA-256)",
		GroupID: "setup",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			t := a.T
			path, err := a.projectFileArg(args)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return fail(ExitNotFound, "E_NOT_FOUND", err.Error(), "")
			}
			var probe config.Config
			if err := probeDecode(data, &probe); err != nil {
				return fail(ExitUsage, "E_CONFIG_INVALID", platform.Tilde(path)+" is invalid — "+err.Error(), "fix it before trusting it")
			}
			t.Step(ui.KindWarn, t.Icons.Lock+" "+t.Bold("Review "+platform.Tilde(path)))
			lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
			for i, l := range lines {
				if i == 80 {
					t.Indent(t.Muted(fmt.Sprintf("… %d more lines", len(lines)-80)))
					break
				}
				fmt.Fprintf(t.Out, "  %s %s\n", t.Subtle(fmt.Sprintf("%3d│", i+1)), t.Text(ui.Sanitize(l, 300)))
			}
			var summary []string
			for n, r := range probe.Recipes {
				s := n + " (" + r.Risk
				if r.Shell != "" {
					s += ", shell"
				}
				summary = append(summary, s+")")
			}
			sort.Strings(summary)
			if len(summary) > 0 {
				t.Sub(t.Warnc("defines recipes that run commands: ") + t.Text(strings.Join(summary, ", ")))
			}
			if a.isAgent() {
				return fail(ExitRefused, "E_AGENT_POLICY", "trusting project config is a human decision", "")
			}
			if !yes {
				if !t.Interactive() {
					return silent(ExitNeedsApply)
				}
				if !t.Confirm("Trust this exact file? Any later edit will need re-trust.") {
					return nil
				}
			}
			e, err := config.Trust(path)
			if err != nil {
				return fail(ExitFailure, "E_IO", err.Error(), "")
			}
			t.Step(ui.KindOK, t.Ok("trusted")+" "+platform.Tilde(e.Path)+t.Muted(" · sha256 "+e.SHA256[:12]+"…"))
			return nil
		},
	}
	c.Flags().BoolVarP(&yes, "yes", "y", false, "trust without asking (after reading it yourself)")
	c.AddCommand(&cobra.Command{
		Use: "ls", Short: "List trusted project configs", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			list, err := config.TrustList()
			if err != nil {
				return fail(ExitFailure, "E_IO", err.Error(), "")
			}
			if a.json() {
				return a.emitJSON(map[string]any{"trusted": list})
			}
			if len(list) == 0 {
				a.T.Step(ui.KindInfo, a.T.Muted("no trusted project configs"))
			}
			for _, e := range list {
				state, _ := config.TrustState(e.Path)
				mark := a.T.Ok("✓")
				if state != "trusted" {
					mark = a.T.Warnc("! " + state)
				}
				fmt.Fprintf(a.T.Out, "  %s %s %s\n", mark, a.T.Text(platform.Tilde(e.Path)), a.T.Muted(e.TrustedAt.Local().Format("Jan 2 2006")))
			}
			return nil
		},
	})
	return c
}

func (a *App) untrustCmd() *cobra.Command {
	return &cobra.Command{
		Use: "untrust [path]", Short: "Forget trust for a project's .thinerc.toml", GroupID: "setup", Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			path, err := a.projectFileArg(args)
			if err != nil {
				return err
			}
			ok, err := config.Untrust(path)
			if err != nil {
				return fail(ExitFailure, "E_IO", err.Error(), "")
			}
			if !ok {
				a.T.Step(ui.KindInfo, a.T.Muted(platform.Tilde(path)+" wasn't trusted"))
				return nil
			}
			a.T.Step(ui.KindOK, "untrusted "+platform.Tilde(path))
			return nil
		},
	}
}

func (a *App) projectFileArg(args []string) (string, error) {
	if len(args) == 1 {
		p, _ := filepath.Abs(platform.ExpandHome(args[0]))
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			p = filepath.Join(p, config.ProjectFile)
		}
		return p, nil
	}
	cwd, _ := os.Getwd()
	p := config.FindProject(cwd)
	if p == "" {
		return "", fail(ExitNotFound, "E_NOT_FOUND", "no .thinerc.toml in this project", "create one: see docs/configuration.md")
	}
	return p, nil
}

// ── config ─────────────────────────────────────────────────────────────────

func (a *App) configCmd() *cobra.Command {
	c := &cobra.Command{Use: "config", Short: "Locate, create, validate and edit your configuration", GroupID: "setup"}
	c.AddCommand(&cobra.Command{
		Use: "path", Short: "Print config, state and data locations", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			p := a.Paths
			if a.json() {
				return a.emitJSON(map[string]any{"config": a.Cfg.GlobalPath, "project": a.Cfg.ProjectPath, "state": p.StateDir, "data": p.DataDir, "quarantine": p.Quarantine, "ops_log": p.OpsLog})
			}
			fmt.Fprintln(a.T.Out, a.Cfg.GlobalPath)
			a.T.Note("state %s · quarantine %s", platform.Tilde(p.StateDir), platform.Tilde(p.Quarantine))
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use: "init", Short: "Write an annotated starter config (never overwrites)", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			created, err := config.WriteStarter(a.Cfg.GlobalPath)
			if err != nil {
				return fail(ExitFailure, "E_IO", err.Error(), "")
			}
			if !created {
				a.T.Step(ui.KindInfo, a.T.Muted(platform.Tilde(a.Cfg.GlobalPath)+" already exists — left untouched"))
				return nil
			}
			a.T.Step(ui.KindOK, a.T.Ok("created ")+platform.Tilde(a.Cfg.GlobalPath))
			a.T.Sub(a.T.Muted("edit it with ") + a.T.Code("thine config edit"))
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use: "validate", Short: "Check the global (and trusted project) config strictly", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if a.CfgErr != nil {
				return fail(ExitUsage, "E_CONFIG_INVALID", a.CfgErr.Error(), "fix the reported line, then re-run")
			}
			msg := platform.Tilde(a.Cfg.GlobalPath) + " is valid"
			if _, err := os.Stat(a.Cfg.GlobalPath); err != nil {
				msg = "no config file yet — built-in defaults are in use"
			}
			a.T.Step(ui.KindOK, a.T.Ok(msg))
			if a.Cfg.ProjectPath != "" {
				a.T.Sub(a.T.Muted(fmt.Sprintf("project %s: %s", platform.Tilde(a.Cfg.ProjectPath), a.Cfg.ProjectState)))
			}
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use: "edit", Short: "Open the config in $EDITOR, then validate it", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			config.WriteStarter(a.Cfg.GlobalPath)
			editor := firstNonEmpty(os.Getenv("VISUAL"), firstNonEmpty(os.Getenv("EDITOR"), "vi"))
			argv := append(strings.Fields(editor), a.Cfg.GlobalPath)
			if _, err := platform.Interactive(cmd.Context(), "", nil, argv...); err != nil {
				return fail(ExitFailure, "E_EXTERNAL_TOOL", "couldn't start "+editor+" — "+err.Error(), "set $EDITOR")
			}
			if _, err := config.Load(a.Cfg.GlobalPath, "", false); err != nil {
				return fail(ExitUsage, "E_CONFIG_INVALID", err.Error(), "thine config edit")
			}
			a.T.Step(ui.KindOK, a.T.Ok("config is valid"))
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use: "show", Short: "Print the effective, merged configuration as JSON", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return a.emitJSON(map[string]any{"config": a.Cfg})
		},
	})
	return c
}

// probeDecode strictly decodes a project file (used by trust before pinning).
func probeDecode(data []byte, c *config.Config) error {
	tmp, err := os.CreateTemp("", "thine-probe-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	tmp.Write(data)
	tmp.Close()
	got, err := config.Load(tmp.Name(), "", false)
	if got != nil {
		*c = *got
	}
	return err
}

// shortcutNames is used by doctor to look for shadowed definitions.
func (a *App) shortcutNames() []string {
	var out []string
	for _, s := range shellinit.Build(a.Cfg) {
		out = append(out, s.Name)
	}
	return out
}
