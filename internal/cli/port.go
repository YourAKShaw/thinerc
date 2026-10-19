package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/ports"
	"github.com/YourAKShaw/thinerc/internal/procs"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

// portRow is a listener enriched with its process and safety tier.
type portRow struct {
	ports.Listener
	Proc   *procs.Proc
	Tier   procs.Tier
	Reason string
}

func (a *App) policy() procs.Policy { return procs.NewPolicy(a.Cfg.Kill.DevNames, a.Cfg.Kill.Confirm) }

// groupPorts are all ports named by configured groups.
func (a *App) groupPorts() map[int]bool {
	out := map[int]bool{}
	for _, g := range a.Cfg.Groups {
		for _, p := range g.Ports {
			out[p] = true
		}
	}
	return out
}

func (a *App) resolvePorts(ctx context.Context, want []int, pol procs.Policy) ([]portRow, error) {
	all, err := ports.All(ctx)
	if err != nil {
		return nil, fail(ExitFailure, "E_RESOLVER", "couldn't list listening ports — "+err.Error(), "")
	}
	ls := ports.Filter(all, want)
	gp := a.groupPorts()
	rows := make([]portRow, 0, len(ls))
	for _, l := range ls {
		r := portRow{Listener: l}
		if l.PID > 0 {
			if p, err := procs.Lookup(l.PID); err == nil {
				r.Proc = &p
				r.Tier, r.Reason = pol.Classify(p, gp[l.Port])
				if ports.IsProxy(p.Name) {
					r.External = ports.DockerContainerFor(ctx, l.Port)
				}
			}
		} else {
			r.Tier, r.Reason = procs.TierConfirm, fmt.Sprintf("owned by %s; its PID is hidden without sudo", procs.Username(l.UID))
			r.External = ports.DockerContainerFor(ctx, l.Port)
		}
		rows = append(rows, r)
	}
	return rows, nil
}

func (a *App) tierBadge(r portRow) string {
	t := a.T
	switch {
	case r.External != nil:
		return t.Badge("docker", t.Pal.Info)
	case r.Tier == procs.TierDev:
		return t.Badge("dev", t.Pal.Success)
	case r.Tier == procs.TierProtected:
		return t.Badge("protected", t.Pal.Error)
	}
	return t.Badge("confirm", t.Pal.Warning)
}

func (a *App) portCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "port <port>...",
		Aliases: []string{"whoport"},
		Short:   "Show who is listening on one or more ports",
		GroupID: "ports",
		Args:    cobra.MinimumNArgs(1),
		Example: "  thine port 3000\n  whoport 3000 5173 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			want, err := parsePorts(args)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			sp := a.T.Spin("Looking up " + plural(len(want), "port", "ports"))
			rows, err := a.resolvePorts(ctx, want, a.policy())
			sp.Stop()
			if err != nil {
				return err
			}
			if a.json() {
				return a.emitJSON(map[string]any{"listeners": jsonRows(rows), "free": freePorts(want, rows)})
			}
			byPort := map[int][]portRow{}
			for _, r := range rows {
				byPort[r.Port] = append(byPort[r.Port], r)
			}
			for _, p := range want {
				rs := byPort[p]
				if len(rs) == 0 {
					a.T.Step(ui.KindOK, fmt.Sprintf("%s %s", a.T.Bold(fmt.Sprintf(":%d", p)), a.T.Muted("is free — nothing is listening")))
					continue
				}
				for _, r := range rs {
					a.printListener(r)
				}
			}
			return nil
		},
	}
}

func (a *App) printListener(r portRow) {
	t := a.T
	port := t.Accent(t.Bold(fmt.Sprintf(":%d", r.Port)))
	switch {
	case r.External != nil:
		t.Step(ui.KindInfo, fmt.Sprintf("%s %s %s", port, t.Text("held by Docker → container "+t.Bold(ui.Sanitize(r.External.Container, 60))), a.tierBadge(r)))
		t.Sub(t.Muted("stop it with ") + t.Code("thine kill "+fmt.Sprint(r.Port)+" --docker") + t.Muted(" (runs docker stop "+r.External.ID+")"))
	case r.Proc == nil:
		t.Step(ui.KindWarn, fmt.Sprintf("%s %s %s", port, t.Text("held by another user"), a.tierBadge(r)))
		t.Sub(t.Muted(r.Reason))
	default:
		p := r.Proc
		t.Step(kindForTier(r.Tier), fmt.Sprintf("%s %s %s %s", port, t.Bold(ui.Sanitize(p.Name, 40)), t.Muted(fmt.Sprintf("(%d)", p.PID)), a.tierBadge(r)))
		t.Sub(t.Text(ui.Truncate(ui.Sanitize(p.CommandLine(), 300), max(20, t.Width()-8))))
		meta := []string{}
		if p.Cwd != "" {
			meta = append(meta, ui.MiddleTruncate(platform.Tilde(p.Cwd), 48))
		}
		meta = append(meta, p.User, "up "+ui.HumanDuration(p.Uptime()), r.Addr)
		t.Indent(t.Muted(strings.Join(meta, " · ")))
	}
}

func kindForTier(tr procs.Tier) ui.Kind {
	switch tr {
	case procs.TierDev:
		return ui.KindOK
	case procs.TierProtected:
		return ui.KindFail
	}
	return ui.KindWarn
}

func (a *App) portsCmd() *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:     "ports",
		Short:   "List listening ports (yours, ≥ 1024 by default)",
		GroupID: "ports",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sp := a.T.Spin("Scanning listeners")
			rows, err := a.resolvePorts(cmd.Context(), nil, a.policy())
			sp.Stop()
			if err != nil {
				return err
			}
			if !all {
				kept := rows[:0]
				for _, r := range rows {
					if r.Port >= 1024 && r.UID == procs.Self() {
						kept = append(kept, r)
					}
				}
				rows = kept
			}
			if a.json() {
				return a.emitJSON(map[string]any{"listeners": jsonRows(rows)})
			}
			a.printPortTable(rows, all)
			return nil
		},
	}
	c.Flags().BoolVarP(&all, "all", "a", false, "include system and other users' listeners")
	return c
}

func (a *App) printPortTable(rows []portRow, all bool) {
	t := a.T
	if len(rows) == 0 {
		t.Step(ui.KindOK, t.Muted("no dev ports are listening — a clean slate"))
		if !all {
			t.Sub(t.Muted("system listeners are hidden; add --all to see them"))
		}
		return
	}
	t.Step(ui.KindInfo, t.Bold(plural(len(rows), "listener", "listeners")))
	w := t.Width()
	for _, r := range rows {
		port := ui.PadRight(t.Accent(t.Bold(fmt.Sprintf(":%d", r.Port))), 7)
		dot := t.Paint(kindForTier(r.Tier), "●")
		name, cmdline, meta := "", "", ""
		switch {
		case r.External != nil:
			dot = t.Infoc("◌")
			name = "docker → " + ui.Sanitize(r.External.Container, 30)
		case r.Proc == nil:
			dot = t.Warnc("◌")
			name = procs.Username(r.UID)
		default:
			name = fmt.Sprintf("%s (%d)", ui.Sanitize(r.Proc.Name, 20), r.PID)
			cmdline = ui.Sanitize(r.Proc.CommandLine(), 200)
			meta = "up " + ui.HumanDuration(r.Proc.Uptime())
			if r.Proc.Cwd != "" {
				meta = ui.MiddleTruncate(platform.Tilde(r.Proc.Cwd), 28) + " · " + meta
			}
		}
		badge := a.tierBadge(r)
		left := fmt.Sprintf("  %s %s %s %s ", port, dot, ui.PadRight(t.Bold(name), 22), badge)
		right := ""
		if meta != "" {
			right = " " + t.Muted(meta)
		}
		rest := w - ui.Width(left) - ui.Width(right) - 1
		if rest < 12 { // narrow terminal: drop the metadata column
			right, rest = "", w-ui.Width(left)-1
		}
		line := left + ui.PadRight(t.Text(ui.Truncate(cmdline, max(0, rest))), max(0, rest)) + right
		fmt.Fprintln(t.Out, line)
	}
}

func jsonRows(rows []portRow) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		m := map[string]any{"port": r.Port, "proto": r.Proto, "addr": r.Addr, "pid": r.PID, "uid": r.UID,
			"tier": string(r.Tier), "external": r.External}
		if r.External != nil {
			m["tier"] = "docker"
		}
		if r.Proc != nil {
			m["name"] = ui.Sanitize(r.Proc.Name, 100)
			m["cmdline"] = sanitizeAll(r.Proc.Cmdline)
			m["cwd"] = r.Proc.Cwd
			m["user"] = r.Proc.User
			m["start_time"] = r.Proc.StartTime
			m["uptime_s"] = int(r.Proc.Uptime() / time.Second)
		}
		out = append(out, m)
	}
	return out
}

func sanitizeAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = ui.Sanitize(s, 1000)
	}
	return out
}

func freePorts(want []int, rows []portRow) []int {
	held := map[int]bool{}
	for _, r := range rows {
		held[r.Port] = true
	}
	free := []int{}
	for _, p := range want {
		if !held[p] {
			free = append(free, p)
		}
	}
	return free
}

func parsePorts(args []string) ([]int, error) {
	seen := map[int]bool{}
	var out []int
	for _, a := range args {
		for _, part := range strings.Split(a, ",") {
			if part == "" {
				continue
			}
			p, err := ports.ParsePort(part)
			if err != nil {
				return nil, fail(ExitUsage, "E_USAGE", err.Error(), "thine port 3000")
			}
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	sort.Ints(out)
	return out, nil
}
