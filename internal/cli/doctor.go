package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/YourAKShaw/thinerc/internal/buildinfo"
	"github.com/YourAKShaw/thinerc/internal/fonts"
	"github.com/YourAKShaw/thinerc/internal/oplog"
	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/quarantine"
	"github.com/YourAKShaw/thinerc/internal/shellinit"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

// check is one doctor finding.
type check struct {
	Section string   `json:"section"`
	Name    string   `json:"name"`
	Status  string   `json:"status"` // ok | warn | fail | info
	Detail  string   `json:"detail"`
	Fix     string   `json:"fix,omitempty"`
	Extra   []string `json:"extra,omitempty"`
}

func mk(section, name, status, detail, fix string, extra ...string) check {
	return check{Section: section, Name: name, Status: status, Detail: detail, Fix: fix, Extra: extra}
}

func (a *App) doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "doctor",
		Short:   "Health check: install, shell wiring, shadowed shortcuts, fonts, config, agents",
		GroupID: "setup",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			checks := a.runChecks(cmd.Context())
			if a.json() {
				code := ExitOK
				for _, c := range checks {
					if c.Status == "fail" {
						code = ExitPartial
					}
				}
				return a.emitJSONCode(map[string]any{"checks": checks}, code)
			}
			return a.printChecks(checks)
		},
	}
}

func (a *App) runChecks(ctx context.Context) []check {
	var cs []check
	add := func(c check) { cs = append(cs, c) }
	bi := buildinfo.Get()

	// Install.
	self, _ := os.Executable()
	add(mk("install", "binary", "ok", fmt.Sprintf("thine %s at %s (%s)", bi.Version, platform.Tilde(self), bi.Target), ""))
	if p := platform.Which("thine"); p == "" {
		add(mk("install", "PATH", "warn", "`thine` is not on your PATH — shortcuts will use the absolute path", "make install (puts it in ~/.local/bin)"))
	} else if binRef() != "thine" {
		add(mk("install", "PATH", "warn", "the `thine` on PATH ("+platform.Tilde(p)+") is a different binary than this one", "reinstall, or remove the stale copy"))
	} else {
		add(mk("install", "PATH", "ok", "on PATH as "+platform.Tilde(p), ""))
	}
	env := platform.OSName()
	if platform.InContainer() {
		env += " · container (sudo paths disabled)"
	}
	add(mk("install", "system", "info", env, ""))

	// Shell wiring.
	var rcFiles []string
	for _, sh := range shellinit.Shells {
		rc := shellinit.RCFile(sh)
		data, err := os.ReadFile(rc)
		installed := platform.Have(sh)
		switch {
		case err != nil && installed && sh == platform.ShellName():
			add(mk("shell", sh, "fail", "your login shell has no thine block ("+platform.Tilde(rc)+" missing)", "thine setup"))
		case err != nil:
			continue
		case sh == "fish":
			add(mk("shell", sh, "ok", "conf.d/thine.fish present", ""))
		case !shellinit.HasBlock(string(data)):
			status := "info"
			fix := ""
			if sh == platform.ShellName() {
				status, fix = "fail", "thine setup"
			}
			add(mk("shell", sh, status, platform.Tilde(rc)+" has no thine block", fix))
			rcFiles = append(rcFiles, rc)
		case !shellinit.BlockIsLast(string(data)):
			add(mk("shell", sh, "warn", "the thine block isn't last in "+platform.Tilde(rc)+" — aliases defined after it won't be seen by the collision guard", "thine setup (moves it to the end)"))
			rcFiles = append(rcFiles, rc)
		default:
			add(mk("shell", sh, "ok", "wired in via "+platform.Tilde(rc), ""))
			rcFiles = append(rcFiles, rc)
		}
		if installed {
			if _, err := os.Stat(shellinit.CompletionPath(sh)); err != nil && sh == platform.ShellName() {
				add(mk("shell", sh+" completions", "warn", "tab completion isn't installed", "thine setup"))
			}
		}
	}
	for _, extra := range []string{".bash_aliases", ".zsh_aliases", ".aliases", ".profile"} {
		if p := filepath.Join(platform.Home(), extra); fileExists(p) {
			rcFiles = append(rcFiles, p)
		}
	}
	defs := shellinit.FindDefinitions(rcFiles, a.shortcutNames())
	if len(defs) == 0 {
		add(mk("shell", "shortcuts", "ok", "no shortcut is shadowed by your own aliases", ""))
	} else {
		var lines []string
		for _, d := range defs {
			lines = append(lines, fmt.Sprintf("%s ← %s:%d  %s", d.Name, platform.Tilde(d.File), d.Line, ui.Truncate(d.Text, 60)))
		}
		add(mk("shell", "shortcuts", "warn", plural(len(defs), "shortcut is", "shortcuts are")+" shadowed by your own definitions (yours win)",
			"remove yours, or set [shortcuts] force = [\"name\"] — or rename: [shortcuts] rename = { kp = \"kport\" }", lines...))
	}

	// Terminal.
	lvl := map[ui.ColorLevel]string{ui.ColorNone: "no colour", ui.Color16: "16 colours", ui.Color256: "256 colours", ui.ColorTrue: "truecolor"}[a.T.Level]
	status := "ok"
	if a.T.Level < ui.ColorTrue && a.T.OutTTY {
		status = "warn"
	}
	termName := firstNonEmpty(os.Getenv("TERM_PROGRAM"), os.Getenv("TERM"))
	add(mk("terminal", "colour", status, fmt.Sprintf("%s (%s)", lvl, termName), pick(status == "warn", "set COLORTERM=truecolor if your terminal supports it", "")))
	installedFonts := fonts.Installed(ctx)
	var nerd []string
	for _, f := range fonts.Catalog {
		if fonts.IsInstalled(f, installedFonts) {
			nerd = append(nerd, f.Family)
		}
	}
	if len(nerd) == 0 {
		add(mk("terminal", "fonts", "warn", "no Nerd Font found — icons=nerd will show boxes", "thine fonts install victor-mono"))
	} else {
		add(mk("terminal", "fonts", "ok", strings.Join(nerd, ", "), ""))
	}

	// Config & state.
	if a.CfgErr != nil {
		add(mk("config", "global", "fail", a.CfgErr.Error(), "thine config edit"))
	} else if fileExists(a.Cfg.GlobalPath) {
		add(mk("config", "global", "ok", platform.Tilde(a.Cfg.GlobalPath)+" is valid", ""))
	} else {
		add(mk("config", "global", "info", "using built-in defaults", "thine config init"))
	}
	if a.Cfg.ProjectPath != "" {
		st := map[string]string{"trusted": "ok", "untrusted": "warn", "changed": "warn"}[a.Cfg.ProjectState]
		add(mk("config", "project", st, platform.Tilde(a.Cfg.ProjectPath)+" is "+a.Cfg.ProjectState, pick(st == "warn", "review it, then thine trust", "")))
	}
	if fi, err := os.Stat(a.Paths.StateDir); err == nil {
		if fi.Mode().Perm()&0o077 != 0 {
			add(mk("config", "state dir", "warn", fmt.Sprintf("%s is %v — other users could read your history", platform.Tilde(a.Paths.StateDir), fi.Mode().Perm()), "chmod 700 "+a.Paths.StateDir))
		} else {
			ops, _ := oplog.Ops()
			add(mk("config", "state dir", "ok", fmt.Sprintf("%s (0700) · %s logged", platform.Tilde(a.Paths.StateDir), plural(len(ops), "operation", "operations")), ""))
		}
	}
	if sums, _ := quarantine.List(); len(sums) > 0 {
		var b int64
		for _, s := range sums {
			b += s.Bytes
		}
		add(mk("config", "quarantine", "info", fmt.Sprintf("%s held for undo", ui.HumanBytes(b)), "thine trash ls"))
	}

	// Companions (integrate, don't rebuild).
	companions := [][3]string{
		{"starship", "prompt", "https://starship.rs"}, {"atuin", "shell history", "https://atuin.sh"},
		{"zoxide", "smarter cd", "https://github.com/ajeetdsouza/zoxide"}, {"fzf", "fuzzy finder", "https://github.com/junegunn/fzf"},
		{"eza", "modern ls", "https://eza.rocks"}, {"bat", "cat with syntax highlighting", "https://github.com/sharkdp/bat"},
		{"fd", "friendly find", "https://github.com/sharkdp/fd"}, {"mise", "runtime versions", "https://mise.jdx.dev"},
	}
	var have, missing []string
	for _, c := range companions {
		if platform.Have(c[0]) {
			have = append(have, c[0])
		} else {
			missing = append(missing, fmt.Sprintf("%s — %s (%s)", c[0], c[1], c[2]))
		}
	}
	if len(have) > 0 {
		add(mk("companions", "installed", "ok", strings.Join(have, ", "), ""))
	}
	if len(missing) > 0 {
		add(mk("companions", "suggested", "info", "tools thine plays well with:", "", missing...))
	}

	// Agents.
	for _, ag := range detectAgents(ctx) {
		if !ag.Installed {
			continue
		}
		st, detail := "ok", firstNonEmpty(ag.Version, "installed")
		if !ag.HooksInstalled && ag.HookFile != "" {
			st, detail = "info", detail+" · no thine notification hook"
		}
		add(mk("agents", ag.Name, st, detail, pick(st == "info", "thine ai hooks", "")))
	}
	return cs
}

func (a *App) printChecks(cs []check) error {
	t := a.T
	t.Step(ui.KindNeutral, t.Brand(t.Icons.Star+" thine doctor")+t.Muted(" · "+time.Now().Format("Mon Jan 2 15:04")))
	section := ""
	fails, warns := 0, 0
	titles := map[string]string{"install": "Install", "shell": "Shell integration", "terminal": "Terminal", "config": "Config & state", "companions": "Companions", "agents": "AI agents"}
	for _, c := range cs {
		if c.Section != section {
			section = c.Section
			fmt.Fprintln(t.Out)
			fmt.Fprintln(t.Out, "  "+t.Bold(titles[section]))
		}
		if t.Animate {
			time.Sleep(35 * time.Millisecond)
		}
		var mark string
		switch c.Status {
		case "ok":
			mark = t.Ok(t.Icons.OK)
		case "warn":
			mark = t.Warnc(t.Icons.Warn)
			warns++
		case "fail":
			mark = t.Bad(t.Icons.Fail)
			fails++
		default:
			mark = t.Infoc(t.Icons.Info)
		}
		fmt.Fprintf(t.Out, "    %s %s %s\n", mark, ui.PadRight(t.Text(c.Name), 16), t.Muted(ui.Sanitize(c.Detail, 300)))
		for _, e := range c.Extra {
			fmt.Fprintf(t.Out, "      %s %s\n", ui.Repeat(" ", 16), t.Subtle("· ")+t.Text(ui.Sanitize(e, 300)))
		}
		if c.Fix != "" && c.Status != "ok" {
			fmt.Fprintf(t.Out, "      %s %s\n", ui.Repeat(" ", 16), t.Subtle(t.Icons.Arrow+" ")+t.Code(c.Fix))
		}
	}
	fmt.Fprintln(t.Out)
	switch {
	case fails > 0:
		t.Step(ui.KindFail, t.Bad(plural(fails, "problem", "problems"))+t.Muted(fmt.Sprintf(" · %s", plural(warns, "warning", "warnings"))))
		return silent(ExitPartial)
	case warns > 0:
		t.Step(ui.KindWarn, t.Warnc(plural(warns, "warning", "warnings"))+t.Muted(" · everything essential works"))
	default:
		t.Step(ui.KindOK, t.Ok("all good")+t.Muted(" — thine is healthy"))
	}
	return nil
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// ── bench ──────────────────────────────────────────────────────────────────

func (a *App) benchCmd() *cobra.Command {
	var runs int
	c := &cobra.Command{
		Use:     "bench",
		Short:   "Measure thine's own shell-startup cost (init generation + sourcing)",
		GroupID: "setup",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			t := a.T
			self, _ := os.Executable()
			type result struct {
				Name     string        `json:"name"`
				P50, P95 time.Duration `json:"-"`
				P50ms    float64       `json:"p50_ms"`
				P95ms    float64       `json:"p95_ms"`
				Budget   float64       `json:"budget_ms"`
			}
			var results []result
			measure := func(name string, budget float64, argv ...string) {
				sp := t.Spin("Measuring " + name)
				var ds []time.Duration
				for i := 0; i < runs; i++ {
					start := time.Now()
					platform.Run(cmd.Context(), 10*time.Second, argv...)
					ds = append(ds, time.Since(start))
					sp.Detail(fmt.Sprintf("%d/%d", i+1, runs))
				}
				sp.Stop()
				p50, p95 := percentile(ds, 50), percentile(ds, 95)
				results = append(results, result{name, p50, p95, ms(p50), ms(p95), budget})
			}
			measure("thine --version (cold start)", 5, self, "--version")
			shell := platform.ShellName()
			if shell != "zsh" && shell != "bash" {
				shell = "bash"
			}
			measure("thine init "+shell, 5, self, "init", shell)
			if platform.Have(shell) {
				measure(shell+" -c 'true' (baseline)", 0, shell, "-c", "true")
				measure(shell+" -c 'eval \"$(thine init)\"'", 0, shell, "-c", `eval "$("$0" init `+shell+`)"`, self)
			}
			if a.json() {
				return a.emitJSON(map[string]any{"runs": runs, "results": results})
			}
			t.Step(ui.KindInfo, t.Bold("thine bench")+t.Muted(fmt.Sprintf(" · %d runs each · p50 / p95", runs)))
			for _, r := range results {
				verdict := ""
				if r.Budget > 0 {
					if r.P95ms <= r.Budget {
						verdict = t.Ok(fmt.Sprintf("within %gms budget", r.Budget))
					} else {
						verdict = t.Warnc(fmt.Sprintf("over %gms budget", r.Budget))
					}
				}
				fmt.Fprintf(t.Out, "    %s %s %s %s\n", ui.PadRight(t.Text(r.Name), 44), ui.PadLeft(t.Accent(fmt.Sprintf("%.1fms", r.P50ms)), 9),
					ui.PadLeft(t.Muted(fmt.Sprintf("%.1fms", r.P95ms)), 9), verdict)
			}
			if len(results) == 4 {
				t.Sub(t.Muted(fmt.Sprintf("thine adds ≈ %.1fms to a new %s (difference of the last two rows); 0 ms per command — it installs no hooks",
					results[3].P50ms-results[2].P50ms, shell)))
			}
			t.Indent(t.Muted("for whole-shell latency, use zsh-bench: https://github.com/romkatv/zsh-bench"))
			return nil
		},
	}
	c.Flags().IntVarP(&runs, "runs", "n", 30, "iterations per measurement")
	return c
}

func percentile(ds []time.Duration, p int) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	s := append([]time.Duration{}, ds...)
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
	idx := (len(s)*p + 99) / 100
	return s[min(max(idx-1, 0), len(s)-1)]
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
