package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

// agentInfo is what `thine ai doctor` knows about one coding agent.
type agentInfo struct {
	Name           string `json:"name"`
	Bin            string `json:"bin"`
	Installed      bool   `json:"installed"`
	Version        string `json:"version,omitempty"`
	HookFile       string `json:"config,omitempty"`
	HooksInstalled bool   `json:"thine_hooks"`
}

var agentDefs = []struct{ name, bin, config string }{
	{"Claude Code", "claude", ".claude/settings.json"},
	{"Codex CLI", "codex", ".codex/config.toml"},
	{"Gemini CLI", "gemini", ".gemini/settings.json"},
	{"GitHub Copilot CLI", "copilot", ""},
	{"Grok Build", "grok", ""},
	{"Kiro CLI", "kiro-cli", ""},
	{"OpenCode", "opencode", ""},
	{"Aider", "aider", ""},
}

// detectAgents probes every known agent in parallel (2 s budget each).
func detectAgents(ctx context.Context) []agentInfo {
	out := make([]agentInfo, len(agentDefs))
	var wg sync.WaitGroup
	for i, d := range agentDefs {
		wg.Add(1)
		go func(i int, name, bin, cfg string) {
			defer wg.Done()
			ai := agentInfo{Name: name, Bin: bin}
			if platform.Have(bin) {
				ai.Installed = true
				if v, err := platform.Output(ctx, 2*time.Second, bin, "--version"); err == nil {
					ai.Version = ui.Sanitize(strings.Split(v, "\n")[0], 60)
				}
			}
			if cfg != "" {
				p := filepath.Join(platform.Home(), cfg)
				ai.HookFile = p
				if b, err := os.ReadFile(p); err == nil {
					ai.HooksInstalled = bytes.Contains(b, []byte("thine notify"))
				}
			}
			out[i] = ai
		}(i, d.name, d.bin, d.config)
	}
	wg.Wait()
	return out
}

func (a *App) aiCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "ai",
		Short:   "Vendor-neutral glue for coding agents (Claude Code, Codex, Gemini…)",
		GroupID: "setup",
	}
	c.AddCommand(&cobra.Command{
		Use:   "doctor",
		Short: "Which agents are installed, their hooks, and this repo's context files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			t := a.T
			sp := t.Spin("Asking each agent for its version")
			agents := detectAgents(cmd.Context())
			sp.Stop()
			cwd, _ := os.Getwd()
			ctxFiles := map[string]bool{}
			for _, f := range []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md", ".github/copilot-instructions.md", ".cursorrules"} {
				ctxFiles[f] = fileExists(filepath.Join(cwd, f))
			}
			if a.json() {
				return a.emitJSON(map[string]any{"agents": agents, "context_files": ctxFiles, "caller": a.Caller})
			}
			t.Step(ui.KindNeutral, t.Icons.Agent+" "+t.Bold("AI agents")+t.Muted(" · thine never calls a model; it glues the ones you use"))
			for _, ag := range agents {
				if !ag.Installed {
					fmt.Fprintf(t.Out, "    %s %s %s\n", t.Subtle("○"), ui.PadRight(t.Muted(ag.Name), 22), t.Muted("not installed"))
					continue
				}
				hook := ""
				if ag.HookFile != "" {
					hook = t.Muted(" · notify hook: ") + pick(ag.HooksInstalled, t.Ok("yes"), t.Warnc("no")+t.Muted(" (thine ai hooks)"))
				}
				fmt.Fprintf(t.Out, "    %s %s %s%s\n", t.Ok("●"), ui.PadRight(t.Bold(ag.Name), 22), t.Text(ag.Version), hook)
			}
			fmt.Fprintln(t.Out)
			t.Step(ui.KindInfo, t.Bold("Context files")+t.Muted(" in "+platform.Tilde(cwd)))
			for _, f := range []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md", ".github/copilot-instructions.md", ".cursorrules"} {
				mark := t.Subtle("○ ")
				if ctxFiles[f] {
					mark = t.Ok("● ")
				}
				fmt.Fprintf(t.Out, "    %s%s\n", mark, t.Text(f))
			}
			if ctxFiles["AGENTS.md"] && !ctxFiles["CLAUDE.md"] {
				t.Sub(t.Muted("Claude Code reads CLAUDE.md, not AGENTS.md — a one-line CLAUDE.md containing ") + t.Code("@AGENTS.md") + t.Muted(" imports it"))
			}
			if a.Caller != "human" {
				t.Sub(t.Muted("you're inside " + a.Caller + " — destructive verbs only plan here"))
			}
			if platform.Have("ccusage") {
				t.Sub(t.Muted("usage and cost for Claude Code: ") + t.Code("ccusage"))
			}
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "hooks",
		Short: "Print ready-to-paste notification hooks for Claude Code and Codex CLI",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			t := a.T
			bin := binRef()
			claude := map[string]any{"hooks": map[string]any{
				"Stop":         []any{map[string]any{"hooks": []any{map[string]string{"type": "command", "command": bin + " notify --from claude --event stop"}}}},
				"Notification": []any{map[string]any{"hooks": []any{map[string]string{"type": "command", "command": bin + " notify --from claude --event attention"}}}},
			}}
			j, _ := json.MarshalIndent(claude, "", "  ")
			t.Step(ui.KindNeutral, t.Bold("Claude Code")+t.Muted(" · merge into ~/.claude/settings.json (or run /hooks inside Claude Code)"))
			for _, l := range strings.Split(string(j), "\n") {
				t.Indent(t.Code(l))
			}
			fmt.Fprintln(t.Out)
			t.Step(ui.KindNeutral, t.Bold("Codex CLI")+t.Muted(" · put at the TOP of ~/.codex/config.toml (before any [table]; one notify per file)"))
			t.Indent(t.Code(fmt.Sprintf(`notify = ["%s", "notify", "--from", "codex"]`, bin)))
			fmt.Fprintln(t.Out)
			t.Sub(t.Muted("test it: ") + t.Code("thine notify --test"))
			return nil
		},
	})
	return c
}

// ── notify ─────────────────────────────────────────────────────────────────

func (a *App) notifyCmd() *cobra.Command {
	var from, title, body, event string
	var test bool
	c := &cobra.Command{
		Use:     "notify [message]",
		Short:   "Desktop notification — for scripts and agent hooks (`long-task; thine notify`)",
		GroupID: "setup",
		Args:    cobra.ArbitraryArgs,
		Example: "  pnpm build; thine notify \"build finished\"\n  thine notify --from claude   # reads the hook's JSON on stdin\n  thine notify --test",
		RunE: func(cmd *cobra.Command, args []string) error {
			n := a.buildNotification(from, title, body, event, args, test)
			if from != "" && throttled(n) {
				return nil
			}
			err := sendNotification(cmd.Context(), n)
			if err != nil && from == "" {
				return fail(ExitFailure, "E_NOTIFY", "couldn't show a notification — "+err.Error(), "install libnotify (notify-send) on Linux")
			}
			// Called from an agent hook: never block or fail the agent.
			return nil
		},
	}
	f := c.Flags()
	f.StringVar(&from, "from", "", "agent calling this hook: claude | codex | gemini (always exits 0)")
	f.StringVar(&title, "title", "", "notification title")
	f.StringVar(&body, "body", "", "notification body")
	f.StringVar(&event, "event", "", "event name: stop | attention | done")
	f.BoolVar(&test, "test", false, "send a sample notification")
	return c
}

type notification struct {
	Agent, Title, Body, Event, Cwd string
}

// buildNotification reads at most 64 KiB of hook JSON (stdin for Claude Code,
// the last argument for Codex), keeps only the event, and sanitizes all text.
func (a *App) buildNotification(from, title, body, event string, args []string, test bool) notification {
	cwd, _ := os.Getwd()
	n := notification{Agent: from, Event: event, Cwd: cwd}
	var payload []byte
	switch from {
	case "codex":
		if len(args) > 0 {
			payload = []byte(args[len(args)-1])
			args = nil
		}
	case "":
	default:
		if !a.T.InTTY {
			payload, _ = io.ReadAll(io.LimitReader(os.Stdin, 64<<10))
		}
	}
	if len(payload) > 0 {
		var p struct {
			Hook    string `json:"hook_event_name"`
			Type    string `json:"type"`
			Cwd     string `json:"cwd"`
			Message string `json:"message"`
		}
		if json.Unmarshal(payload, &p) == nil {
			if n.Event == "" {
				n.Event = firstNonEmpty(p.Hook, p.Type)
			}
			if p.Cwd != "" {
				n.Cwd = p.Cwd
			}
		}
	}
	agentNames := map[string]string{"claude": "Claude Code", "codex": "Codex", "gemini": "Gemini"}
	repo := filepath.Base(n.Cwd)
	branch := ""
	if b, err := platform.Output(context.Background(), time.Second, "git", "-C", n.Cwd, "branch", "--show-current"); err == nil && b != "" {
		branch = " (" + b + ")"
	}
	switch {
	case title != "":
		n.Title = title
	case from != "":
		n.Title = fmt.Sprintf("%s · %s%s", firstNonEmpty(agentNames[from], from), repo, branch)
	default:
		n.Title = "✦ thine · " + repo + branch
	}
	switch {
	case body != "":
		n.Body = body
	case len(args) > 0:
		n.Body = strings.Join(args, " ")
	case test:
		n.Body = "notifications work 🎉"
	default:
		ev := strings.ToLower(n.Event)
		switch {
		case strings.Contains(ev, "notification") || strings.Contains(ev, "attention") || strings.Contains(ev, "permission"):
			n.Body = "needs your input"
		case strings.Contains(ev, "stop") || strings.Contains(ev, "complete") || strings.Contains(ev, "done"):
			n.Body = "finished"
		default:
			n.Body = "done"
		}
	}
	n.Title, n.Body = ui.Sanitize(n.Title, 120), ui.Sanitize(n.Body, 200)
	return n
}

// throttled coalesces identical (agent, cwd, event) notifications within 3 s.
func throttled(n notification) bool {
	dir := filepath.Join(platform.GetPaths().StateDir, "notify")
	if platform.EnsurePrivateDir(dir) != nil {
		return false
	}
	key := fmt.Sprintf("%x", []byte(n.Agent+"|"+n.Cwd+"|"+n.Event))
	if len(key) > 120 {
		key = key[len(key)-120:]
	}
	p := filepath.Join(dir, key)
	if fi, err := os.Stat(p); err == nil && time.Since(fi.ModTime()) < 3*time.Second {
		return true
	}
	os.WriteFile(p, nil, 0o600)
	return false
}

// sendNotification picks a backend. Text is always passed as argv, never
// interpolated into a script (T5: prompt-injected agent output).
func sendNotification(ctx context.Context, n notification) error {
	switch {
	case platform.IsDarwin() && platform.Have("osascript"):
		_, err := platform.Run(ctx, 5*time.Second, "osascript",
			"-e", "on run argv", "-e", "display notification (item 1 of argv) with title (item 2 of argv) sound name \"Glass\"", "-e", "end run",
			"--", n.Body, n.Title)
		return err
	case platform.Have("notify-send"):
		_, err := platform.Run(ctx, 5*time.Second, "notify-send", "--app-name=thine", "--", n.Title, n.Body)
		return err
	case platform.IsWSL() && platform.Have("powershell.exe"):
		script := "param($t,$b) [void][Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime];" +
			"$x=[Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent(1);$n=$x.GetElementsByTagName('text');" +
			"$n.Item(0).AppendChild($x.CreateTextNode($t))>$null;$n.Item(1).AppendChild($x.CreateTextNode($b))>$null;" +
			"[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('thine').Show([Windows.UI.Notifications.ToastNotification]::new($x))"
		_, err := platform.Run(ctx, 8*time.Second, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "& {"+script+"}", n.Title, n.Body)
		return err
	}
	// Terminal fallback: OSC 9 (iTerm2, WezTerm, kitty, Windows Terminal…) then the bell.
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer tty.Close()
	_, err = fmt.Fprintf(tty, "\x1b]9;%s: %s\x07\a", n.Title, n.Body)
	return err
}
