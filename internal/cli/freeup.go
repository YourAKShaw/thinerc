package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/YourAKShaw/thinerc/internal/config"
	"github.com/YourAKShaw/thinerc/internal/mem"
	"github.com/YourAKShaw/thinerc/internal/oplog"
	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/procs"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

func (a *App) freeupCmd() *cobra.Command {
	var stop string
	var dropCaches, yes bool
	var top int
	c := &cobra.Command{
		Use:     "freeup",
		Short:   "Honest memory report; reclaim by stopping real consumers",
		GroupID: "reclaim",
		Long: "freeup shows AVAILABLE memory and pressure — the numbers that matter — then offers to stop\n" +
			"the dev servers and language servers actually using it. Dropping the page cache only inflates\n" +
			"\"free\"; it is available behind --drop-caches, with a before/after that shows why.",
		Example: "  freeup\n  freeup --stop all\n  freeup --stop 1,3\n  freeup --drop-caches",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.runFreeup(cmd.Context(), stop, dropCaches, yes, top)
		},
	}
	f := c.Flags()
	f.StringVar(&stop, "stop", "", "stop candidates without asking: all, none, or numbers like 1,3-4")
	f.BoolVar(&dropCaches, "drop-caches", false, "also drop the page cache (needs sudo; explained first)")
	f.BoolVarP(&yes, "yes", "y", false, "skip the confirmation for --drop-caches")
	f.IntVar(&top, "top", 8, "how many top consumers to show")
	return c
}

type freeCandidate struct {
	mem.Consumer
	targets []procs.Proc
}

func (a *App) runFreeup(ctx context.Context, stop string, dropCaches, yes bool, top int) error {
	t := a.T
	sp := t.Spin("Reading memory")
	before, err := mem.Read(ctx)
	if err != nil {
		sp.Stop()
		return fail(ExitFailure, "E_RESOLVER", "couldn't read memory — "+err.Error(), "")
	}
	sp.Update("Measuring who uses it")
	all, _ := procs.List()
	consumers := mem.TopConsumers(all, max(top, 1), a.Cfg.Kill.DevNames, a.Cfg.Freeup.LanguageServers)
	sp.Stop()

	pol := a.policy()
	var cands []freeCandidate
	for _, c := range mem.TopConsumers(all, 40, a.Cfg.Kill.DevNames, a.Cfg.Freeup.LanguageServers) {
		if !c.Own || (c.Kind != "dev" && c.Kind != "language-server") {
			continue
		}
		fc := freeCandidate{Consumer: c}
		for _, p := range c.Procs {
			if tier, _ := pol.Classify(p, false); tier == procs.TierDev || (c.Kind == "language-server" && tier != procs.TierProtected && p.UID == procs.Self()) {
				fc.targets = append(fc.targets, p)
			}
		}
		if len(fc.targets) > 0 {
			cands = append(cands, fc)
		}
	}

	if a.json() && stop == "" && !dropCaches {
		cj := []map[string]any{}
		for i, c := range cands {
			cj = append(cj, map[string]any{"index": i + 1, "name": c.Name, "kind": c.Kind, "bytes": c.Bytes, "processes": len(c.targets)})
		}
		return a.emitJSON(map[string]any{"memory": before, "top": consumers, "candidates": cj,
			"next": "thine freeup --stop all"})
	}
	if !a.json() {
		a.printMemory(before)
		a.printConsumers(consumers)
	}

	stopped := 0
	if len(cands) > 0 && !a.json() {
		fmt.Fprintln(t.Out)
		t.Step(ui.KindNeutral, t.Bold("Reclaimable by stopping")+" "+t.Muted("(your dev and language servers)"))
		var est int64
		for i, c := range cands {
			est += c.Bytes
			ex := ""
			if c.Example != "" {
				ex = t.Muted(" · " + ui.Truncate(ui.Sanitize(c.Example, 200), 46))
			}
			t.Indent(fmt.Sprintf("%s %s %s%s", t.Accent(fmt.Sprintf("%2d.", i+1)), ui.PadLeft(t.Bold(ui.HumanBytes(c.Bytes)), 9),
				t.Text(fmt.Sprintf("%s ×%d", ui.Sanitize(c.Name, 30), len(c.targets))), ex))
		}
		t.Sub(t.Muted(fmt.Sprintf("≈ %s (est.) if you stop them all", ui.HumanBytes(est))))
	} else if !a.json() {
		fmt.Fprintln(t.Out)
		t.Step(ui.KindOK, t.Muted("no idle dev or language servers to stop — your memory is going to apps you're using"))
	}

	// Choose what to stop.
	var chosen []int
	switch {
	case len(cands) == 0:
	case stop != "":
		chosen, err = ui.ParseSelection(stop, len(cands))
		if err != nil {
			return fail(ExitUsage, "E_USAGE", err.Error(), "freeup --stop 1,3")
		}
	case a.isAgent() && a.Cfg.Agents.Policy.Freeup != "allow":
		t.Note("agent mode: freeup only reports — a human can run `thine freeup --stop all`")
	case t.Interactive():
		ans := t.Ask("Stop which? numbers like 1,3 · all · none", "none")
		chosen, err = ui.ParseSelection(ans, len(cands))
		if err != nil {
			return fail(ExitUsage, "E_USAGE", err.Error(), "")
		}
	}
	if len(chosen) > 0 {
		grace, _ := config.ParseDuration(a.Cfg.Kill.Grace)
		opID := oplog.NewID()
		var items []oplog.Item
		for _, i := range chosen {
			for _, p := range cands[i].targets {
				items = append(items, oplog.Item{ID: fmt.Sprintf("pid:%d", p.PID), Action: "signal", Target: ui.Sanitize(p.Name, 60)})
			}
		}
		oplog.Append(oplog.Record{OpID: opID, Phase: "intent", Verb: "freeup", Argv: a.argv, Caller: a.Caller, Items: items})
		fmt.Fprintln(t.Err)
		for _, i := range chosen {
			for _, p := range cands[i].targets {
				p := p
				kt := &killTarget{Proc: &p, Members: []procs.Proc{p}, Tier: procs.TierDev}
				a.stopProcess(kt, grace, killOpts{})
				if kt.logResult() == "done" {
					stopped++
				}
			}
		}
		oplog.Append(oplog.Record{OpID: opID, Phase: "outcome", Verb: "freeup", Caller: a.Caller, Items: items, Result: "completed"})
	}

	// Optional, honest cache drop.
	if dropCaches {
		if err := a.dropCaches(ctx, yes); err != nil {
			return err
		}
	}

	if stopped > 0 || dropCaches {
		sp := t.Spin("Letting memory settle")
		time.Sleep(2 * time.Second)
		after, _ := mem.Read(ctx)
		sp.Stop()
		delta := after.Available - before.Available
		msg := fmt.Sprintf("available %s → %s (%s%s, measured)", ui.HumanBytes(before.Available), ui.HumanBytes(after.Available), sign(delta), ui.HumanBytes(abs(delta)))
		if a.json() {
			return a.emitJSON(map[string]any{"before": before, "after": after, "stopped": stopped, "available_delta_bytes": delta})
		}
		fmt.Fprintln(t.Out)
		if delta >= 512<<20 {
			t.Celebrate(t.Icons.Party + " " + msg)
		} else {
			t.Step(ui.KindOK, msg)
		}
		if dropCaches {
			t.Sub(t.Muted(fmt.Sprintf("page cache %s → %s · that cache was already reclaimable, which is why \"available\" barely moves",
				ui.HumanBytes(before.Cache), ui.HumanBytes(after.Cache))))
		}
	}
	return nil
}

func (a *App) printMemory(s mem.Snapshot) {
	t := a.T
	used := s.UsedPct() / 100
	head := fmt.Sprintf("%s %s total · %s available (%d%%)", t.Icons.Memory, t.Bold(ui.HumanBytes(s.Total)),
		t.Ok(t.Bold(ui.HumanBytes(s.Available))), int(100*float64(s.Available)/float64(max(s.Total, 1))))
	t.Step(ui.KindInfo, head)
	stops := []ui.RGB{t.Pal.Success, t.Pal.Warning, t.Pal.Error}
	barW := min(40, t.Width()-30)
	// Animate the gauge filling, then print the final frame to stdout.
	if t.Animate {
		for i := 0; i <= 14; i++ {
			f := used * float64(i) / 14
			io.WriteString(t.Err, "\r\x1b[2K  "+t.Subtle(t.Icons.Tree)+"  "+t.Bar(f, barW, stops...)+" "+t.Muted(fmt.Sprintf("%3.0f%% used", f*100)))
			time.Sleep(22 * time.Millisecond)
		}
		io.WriteString(t.Err, "\r\x1b[2K")
	}
	t.Sub(t.Bar(used, barW, stops...) + " " + t.Muted(fmt.Sprintf("%3.0f%% used", used*100)))
	var parts []string
	if s.HasPressure {
		label := "calm"
		switch {
		case s.Pressure >= 25:
			label = t.Bad("high")
		case s.Pressure >= 5:
			label = t.Warnc("some")
		}
		src := "PSI avg10"
		if platform.IsDarwin() {
			src = "memory_pressure"
		}
		parts = append(parts, fmt.Sprintf("pressure %.1f%% %s", s.Pressure, label)+t.Muted(" ("+src+")"))
	}
	if s.SwapTotal > 0 {
		parts = append(parts, fmt.Sprintf("swap %s / %s", ui.HumanBytes(s.SwapUsed), ui.HumanBytes(s.SwapTotal)))
	}
	parts = append(parts, fmt.Sprintf("page cache %s", ui.HumanBytes(s.Cache))+t.Muted(" (reclaimable on demand)"))
	t.Indent(strings.Join(parts, t.Subtle(" · ")))
	if platform.IsWSL() {
		t.Indent(t.Muted("WSL: these numbers describe the Linux VM, not all of Windows"))
	}
}

func (a *App) printConsumers(cs []mem.Consumer) {
	t := a.T
	fmt.Fprintln(t.Out)
	t.Step(ui.KindNeutral, t.Bold("Top consumers"))
	advice := map[string]string{
		"browser":         "close tabs to reclaim",
		"ide":             "close unused windows",
		"docker":          "stop idle containers: docker compose down",
		"language-server": "restarts on demand",
		"dev":             "stop when idle",
	}
	for _, c := range cs {
		unit := "RSS"
		if c.Exact {
			unit = "PSS"
		}
		name := ui.Sanitize(c.Name, 28)
		if c.Count > 1 {
			name += fmt.Sprintf(" ×%d", c.Count)
		}
		hint := advice[c.Kind]
		if !c.Own {
			hint = "another user's process"
		}
		if hint != "" {
			unit += " · " + hint
		}
		t.Indent(fmt.Sprintf("%s  %s %s %s", ui.PadLeft(t.Bold(ui.HumanBytes(c.Bytes)), 9), ui.PadRight(t.Text(name), 30),
			ui.PadRight(t.Muted(c.Kind), 16), t.Muted(unit)))
	}
}

// dropCaches explains, prints the exact argv, confirms, then runs it.
func (a *App) dropCaches(ctx context.Context, yes bool) error {
	t := a.T
	if platform.InContainer() {
		return fail(ExitRefused, "E_CONTAINER", "can't drop caches inside a container — /proc/sys is read-only here", "")
	}
	argv := mem.DropCachesArgv()
	fmt.Fprintln(t.Out)
	t.Step(ui.KindWarn, t.Bold("About --drop-caches"))
	t.Sub(t.Text("The page cache is memory the kernel already hands back the moment an app needs it."))
	t.Indent(t.Text("Dropping it makes \"free\" jump, but real headroom (\"available\") barely moves,"))
	t.Indent(t.Text("and the next file reads are slower while the cache refills."))
	t.Indent(t.Muted("will run: ") + t.Code("sudo "+strings.Join(argv, " ")))
	if !yes {
		if !t.Interactive() {
			t.Note("add --yes to run it without a terminal")
			return silent(ExitNeedsApply)
		}
		if !t.Confirm("Drop the page cache anyway?") {
			return nil
		}
	}
	if platform.IsLinux() {
		platform.Run(ctx, 30*time.Second, "sync")
	}
	if _, err := platform.Sudo(ctx, argv...); err != nil {
		return fail(ExitFailure, "E_EXTERNAL_TOOL", "sudo "+strings.Join(argv, " ")+" failed — "+err.Error(), "")
	}
	return nil
}

func sign(n int64) string {
	if n < 0 {
		return "−"
	}
	return "+"
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
