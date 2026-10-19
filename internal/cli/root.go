// Package cli wires thine's command tree: global flags, configuration, output
// mode and the stable exit codes in docs/reference/exit-codes.md.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/YourAKShaw/thinerc/internal/buildinfo"
	"github.com/YourAKShaw/thinerc/internal/config"
	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

// Exit codes (stable contract; plan Appendix D).
const (
	ExitOK          = 0
	ExitFailure     = 1
	ExitUsage       = 2
	ExitNotFound    = 3
	ExitPartial     = 4
	ExitRefused     = 5
	ExitNeedsApply  = 6
	ExitLocked      = 7
	ExitInterrupted = 130
)

// ExitError carries an exit code and a stable error code for --json output.
type ExitError struct {
	Code    int
	ErrCode string
	Msg     string
	Hint    string
	Quiet   bool // already reported to the user
}

func (e *ExitError) Error() string { return e.Msg }

func fail(code int, errCode, msg, hint string) *ExitError {
	return &ExitError{Code: code, ErrCode: errCode, Msg: msg, Hint: hint}
}

// silent wraps a code whose details were already printed.
func silent(code int) *ExitError { return &ExitError{Code: code, Quiet: true} }

// App holds per-invocation state shared by commands.
type App struct {
	T       *ui.Term
	Cfg     *config.Config
	Paths   platform.Paths
	CfgErr  error
	flags   globalFlags
	Caller  string
	argv    []string
	noticed bool
}

type globalFlags struct {
	json, plain, quiet, noColor, noProject, noAnim bool
	config, theme                                  string
}

// Execute runs thine and returns the process exit code.
func Execute(args []string) int {
	app := &App{argv: args}
	root := app.rootCmd()
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		return ExitOK
	}
	t := app.term()
	var ee *ExitError
	if !errors.As(err, &ee) {
		// cobra usage errors (unknown flag/command, bad args)
		msg := err.Error()
		ee = &ExitError{Code: ExitUsage, ErrCode: "E_USAGE", Msg: msg, Hint: "thine --help"}
		if !strings.Contains(msg, "unknown") && !strings.Contains(msg, "arg") && !strings.Contains(msg, "flag") {
			ee.Code, ee.ErrCode, ee.Hint = ExitFailure, "E_INTERNAL", ""
		}
	}
	if ee.Quiet {
		return ee.Code
	}
	if app.flags.json {
		json.NewEncoder(os.Stdout).Encode(map[string]any{
			"schema_version": 1,
			"error":          map[string]string{"code": ee.ErrCode, "message": ee.Msg, "hint": ee.Hint},
			"exit_code":      ee.Code,
		})
		return ee.Code
	}
	what, why, _ := strings.Cut(ee.Msg, " — ")
	t.Fail(what, why, ee.Hint)
	return ee.Code
}

func (a *App) term() *ui.Term {
	if a.T == nil {
		a.T = ui.Std()
	}
	return a.T
}

func (a *App) rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "thine",
		Short: "One-word superpowers for your terminal — safe, animated, agent-aware",
		Long: "thine is the safe verbs layer behind .thinerc: free stuck ports, reclaim disk and memory\n" +
			"honestly, and run typed recipes — with previews, undo and gorgeous output.",
		Version:           buildinfo.Get().Version,
		SilenceUsage:      true,
		SilenceErrors:     true,
		CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error { return a.setup(cmd) },
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !a.T.Interactive() && !a.T.OutTTY {
				return cmd.Help()
			}
			return a.hello(false)
		},
	}
	root.SetVersionTemplate("thine {{.Version}}\n")
	pf := root.PersistentFlags()
	pf.BoolVar(&a.flags.json, "json", false, "machine-readable output (versioned schema, no colour)")
	pf.BoolVar(&a.flags.plain, "plain", false, "no colour, icons or animation (screen-reader friendly)")
	pf.BoolVarP(&a.flags.quiet, "quiet", "q", false, "only results and errors")
	pf.BoolVar(&a.flags.noColor, "no-color", false, "disable colour")
	pf.BoolVar(&a.flags.noAnim, "no-anim", false, "disable animation")
	pf.BoolVar(&a.flags.noProject, "no-project", false, "ignore the project's .thinerc.toml")
	pf.StringVar(&a.flags.config, "config", "", "path to the global config file")
	pf.StringVar(&a.flags.theme, "theme", "", "colour theme for this run (see: thine theme ls)")

	root.AddGroup(
		&cobra.Group{ID: "ports", Title: "Ports & processes:"},
		&cobra.Group{ID: "reclaim", Title: "Reclaim disk & memory:"},
		&cobra.Group{ID: "recipes", Title: "Recipes (your own verbs):"},
		&cobra.Group{ID: "look", Title: "Look & feel:"},
		&cobra.Group{ID: "setup", Title: "Setup & health:"},
	)
	for _, c := range a.commands() {
		root.AddCommand(c)
	}
	root.SetHelpCommandGroupID("setup")
	root.SetCompletionCommandGroupID("setup")
	return root
}

// setup loads config and initialises the terminal once per invocation.
func (a *App) setup(cmd *cobra.Command) error {
	a.Paths = platform.GetPaths()
	cfgPath := a.Paths.ConfigFile
	if a.flags.config != "" {
		cfgPath = platform.ExpandHome(a.flags.config)
	}
	cwd, _ := os.Getwd()
	cfg, err := config.Load(cfgPath, cwd, !a.flags.noProject)
	a.Cfg, a.CfgErr = cfg, err
	theme := cfg.UI.Theme
	if a.flags.theme != "" {
		theme = a.flags.theme
	}
	if v := os.Getenv("THINE_THEME"); v != "" && a.flags.theme == "" {
		theme = v
	}
	a.T = ui.Init(ui.Options{
		JSON: a.flags.json, Plain: a.flags.plain, Quiet: a.flags.quiet,
		NoColor:    a.flags.noColor || cfg.UI.Color == "never",
		Animations: config.Bool(cfg.UI.Animations, true) && !a.flags.noAnim,
		Icons:      cfg.UI.Icons, Theme: theme, Splash: config.Bool(cfg.UI.Splash, true),
	})
	if cfg.UI.Color == "always" && !a.flags.json && !a.flags.plain && a.T.Level == ui.ColorNone && os.Getenv("NO_COLOR") == "" {
		a.T.Level = ui.ColorTrue
	}
	a.Caller = "human"
	if c := platform.AgentCaller(); c != "" {
		a.Caller = "agent:" + c
	}
	if platform.IsRoot() && os.Getenv("THINE_ALLOW_ROOT") == "" && cmd.Name() != "version" && cmd.Name() != "init" {
		return fail(ExitRefused, "E_ROOT", "refusing to run as root — root-owned files would end up in your state directory",
			"run thine as your normal user (set THINE_ALLOW_ROOT=1 to override)")
	}
	if err != nil {
		switch cmd.Name() {
		case "doctor", "config", "validate", "path", "edit", "init", "version", "help":
		default:
			return fail(ExitUsage, "E_CONFIG_INVALID", "configuration is invalid — "+err.Error(), "thine config validate")
		}
	}
	if !a.flags.json && !a.flags.quiet && cmd.Name() != "init" && cmd.Name() != "hello" {
		a.projectNotice()
	}
	return nil
}

// projectNotice tells the user (once per run) that a project config is being ignored.
func (a *App) projectNotice() {
	if a.noticed || a.Cfg == nil || a.Cfg.ProjectPath == "" {
		return
	}
	a.noticed = true
	switch a.Cfg.ProjectState {
	case "untrusted":
		a.T.Note("%s ignoring untrusted %s — review it, then run `thine trust`", a.T.Icons.Lock, platform.Tilde(a.Cfg.ProjectPath))
	case "changed":
		a.T.Note("%s %s changed since you trusted it — review, then `thine trust` again", a.T.Icons.Lock, platform.Tilde(a.Cfg.ProjectPath))
	}
}

// emitJSON writes v with the schema version on stdout.
func (a *App) emitJSON(v map[string]any) error {
	v["schema_version"] = 1
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (a *App) json() bool { return a.flags.json }

// isAgent reports agent mode (plan §6.7).
func (a *App) isAgent() bool { return strings.HasPrefix(a.Caller, "agent:") }

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}
