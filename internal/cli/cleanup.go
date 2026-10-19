package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/YourAKShaw/thinerc/internal/cleanup"
	"github.com/YourAKShaw/thinerc/internal/config"
	"github.com/YourAKShaw/thinerc/internal/oplog"
	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/quarantine"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

type cleanupOpts struct {
	only, skip, include, roots []string
	older                      string
	apply, yes, dryRun         bool
}

func (a *App) cleanupCmd() *cobra.Command {
	var o cleanupOpts
	c := &cobra.Command{
		Use:     "cleanup",
		Short:   "Plan → confirm → apply → undo disk reclamation (dry run by default)",
		GroupID: "reclaim",
		Long: "cleanup scans package-manager caches, Docker, stale build artifacts and old installers, shows a plan\n" +
			"with honest totals, and only acts when you say yes. Caches are cleaned with each tool's own command;\n" +
			"your files are quarantined and restorable with `thine undo` for the retention period.",
		Example: "  cleanup\n  cleanup --apply --yes\n  cleanup --only npm,docker-build\n  cleanup --include system,go-modcache\n  thine cleanup --json   # plan for scripts and agents",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.runCleanup(cmd.Context(), o)
		},
	}
	f := c.Flags()
	f.StringSliceVar(&o.only, "only", nil, "only these categories (see `thine cleanup ls`)")
	f.StringSliceVar(&o.skip, "skip", nil, "skip these categories")
	f.StringSliceVar(&o.include, "include", nil, "add optional categories, or `system` for apt/journald (sudo)")
	f.StringSliceVar(&o.roots, "roots", nil, "project roots to search for stale artifacts")
	f.StringVar(&o.older, "older", "", "staleness threshold, e.g. 30d (default from config)")
	f.BoolVar(&o.apply, "apply", false, "apply the plan (still asks on a terminal unless --yes)")
	f.BoolVarP(&o.yes, "yes", "y", false, "don't ask for confirmation")
	f.BoolVar(&o.dryRun, "dry-run", false, "only show the plan")
	c.AddCommand(&cobra.Command{
		Use:   "ls",
		Short: "List cleanup categories",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return a.cleanupList() },
	})
	return c
}

func (a *App) cleanupList() error {
	t := a.T
	var rows []map[string]any
	for _, c := range cleanup.Catalog() {
		if !c.Applies() {
			continue
		}
		def := "optional"
		if c.Group != "" {
			def = "--include " + c.Group
		} else if c.Default {
			def = "default"
		}
		how := c.Action
		if len(c.Argv) > 0 {
			how = strings.Join(c.Argv, " ")
		}
		if a.json() {
			rows = append(rows, map[string]any{"id": c.ID, "title": c.Title, "risk": c.Risk, "selection": def, "action": how})
			continue
		}
		fmt.Fprintf(t.Out, "  %s %s %s %s\n", ui.PadRight(t.Accent(c.ID), 22), ui.PadRight(a.riskBadge(c.Risk), 15),
			ui.PadRight(t.Text(c.Title), 36), t.Muted(def+" · "+how))
	}
	if a.json() {
		return a.emitJSON(map[string]any{"categories": rows})
	}
	return nil
}

func (a *App) riskBadge(r string) string {
	t := a.T
	switch r {
	case cleanup.Regenerable:
		return t.Badge("regenerable", t.Pal.Info)
	case cleanup.Reversible:
		return t.Badge("reversible", t.Pal.Success)
	}
	return t.Badge("destructive", t.Pal.Error)
}

func (a *App) cleanupRoots(extra []string) []string {
	if len(extra) > 0 {
		return extra
	}
	var out []string
	for _, r := range a.Cfg.Cleanup.Roots {
		out = append(out, platform.ExpandHome(r))
	}
	return out
}

func (a *App) runCleanup(ctx context.Context, o cleanupOpts) error {
	t := a.T
	stale, err := config.ParseDuration(firstNonEmpty(o.older, a.Cfg.Cleanup.StaleAfter))
	if err != nil {
		return fail(ExitUsage, "E_USAGE", "invalid --older — "+err.Error(), "--older 30d")
	}
	known := map[string]bool{"system": true}
	for _, c := range cleanup.Catalog() {
		known[c.ID] = true
	}
	for _, id := range append(append(append([]string{}, o.only...), o.skip...), o.include...) {
		if !known[id] {
			return fail(ExitUsage, "E_USAGE", fmt.Sprintf("unknown cleanup category %q", id), "thine cleanup ls")
		}
	}
	roots := a.cleanupRoots(o.roots)

	sp := t.Spin("Scanning caches and projects")
	plan := cleanup.Scan(ctx, cleanup.Options{
		Only: o.only, Skip: append(o.skip, a.Cfg.Cleanup.Skip...), Include: o.include,
		Roots: roots, Exclude: a.Cfg.Cleanup.Exclude, StaleAfter: stale,
		Progress: func(cat string, done, total int) {
			sp.Update("Scanned " + cat)
			sp.Detail(fmt.Sprintf("%d/%d", done, total))
		},
	})
	sp.Stop()
	plan.ID = oplog.NewID()

	retention, _ := config.ParseDuration(a.Cfg.Cleanup.Retention)
	expired := expiredTrash(retention)

	if len(plan.Items) == 0 && len(expired) == 0 {
		if a.json() {
			return a.emitJSON(map[string]any{"plan_id": plan.ID, "mode": "noop", "items": []any{}, "totals": plan.Totals()})
		}
		t.Step(ui.KindOK, t.Ok("Spotless")+t.Muted(" — nothing worth reclaiming right now"))
		return nil
	}

	mode := "dry-run"
	if a.json() {
		if err := savePlan(plan); err != nil {
			return fail(ExitFailure, "E_IO", "couldn't save the plan — "+err.Error(), "")
		}
		if !o.apply || a.isAgent() {
			a.emitJSON(map[string]any{"plan_id": plan.ID, "mode": mode, "created_at": plan.CreatedAt, "items": plan.Items,
				"totals": plan.Totals(), "partial": plan.Partial, "expired_trash": expired, "next": "thine apply " + plan.ID})
			if o.dryRun {
				return nil
			}
			return silent(ExitNeedsApply)
		}
	} else {
		a.printCleanupPlan(plan, expired, retention)
	}

	// Decide.
	switch {
	case o.dryRun:
		t.Note("dry run — nothing changed")
		return nil
	case a.isAgent():
		savePlan(plan)
		t.Note("agent mode: plan saved — a human can apply it with `thine apply %s`", plan.ID)
		return silent(ExitNeedsApply)
	case o.yes:
	case t.Interactive():
		fmt.Fprintln(t.Err)
		if !t.Confirm("Apply this plan?") {
			t.Note("nothing changed · run `cleanup --apply --yes` to skip this question")
			return nil
		}
	case o.apply:
		// Non-interactive with --apply: regenerable and reversible items only (no destructive in catalog).
	default:
		savePlan(plan)
		t.Note("no terminal to confirm — re-run with --apply, or `thine apply %s`", plan.ID)
		return silent(ExitNeedsApply)
	}
	return a.applyCleanup(ctx, plan, roots, expired, retention)
}

func (a *App) printCleanupPlan(p cleanup.Plan, expired []quarantine.Summary, retention time.Duration) {
	t := a.T
	t.Step(ui.KindNeutral, fmt.Sprintf("%s %s %s", t.Icons.Disk, t.Bold("cleanup plan"), t.Muted("· "+p.ID[:10]+"… (dry run until you say yes)")))
	type group struct {
		cat   string
		items []cleanup.Item
		bytes int64
	}
	var groups []*group
	idx := map[string]*group{}
	for _, it := range p.Items {
		g, ok := idx[it.Category]
		if !ok {
			g = &group{cat: it.Category}
			idx[it.Category] = g
			groups = append(groups, g)
		}
		g.items = append(g.items, it)
		g.bytes += it.Bytes
	}
	titles := map[string]string{}
	for _, c := range cleanup.Catalog() {
		titles[c.ID] = c.Title
	}
	w := t.Width()
	for _, g := range groups {
		first := g.items[0]
		title := titles[g.cat]
		if len(g.items) > 1 {
			title += fmt.Sprintf(" ×%d", len(g.items))
		}
		how := ""
		switch first.Action {
		case cleanup.ActExec:
			how = strings.Join(first.Argv, " ")
		case cleanup.ActSudo:
			how = "sudo " + strings.Join(first.Argv, " ")
		case cleanup.ActDelete:
			how = "empty " + platform.Tilde(first.Paths[0])
		case cleanup.ActQuarantine:
			how = fmt.Sprintf("quarantine · undo for %s", ui.HumanDuration(retention))
		}
		fmt.Fprintf(t.Out, "  %s %s %s\n", ui.PadRight(a.riskBadge(first.Risk), 15), ui.PadRight(t.Bold(title), 36),
			ui.PadLeft(t.Accent(t.Bold(ui.HumanBytes(g.bytes))), 10))
		fmt.Fprintf(t.Out, "  %s %s\n", ui.Repeat(" ", 15), t.Muted("↳ "+ui.Truncate(how, max(10, w-22))))
		if first.Action == cleanup.ActQuarantine {
			sort.Slice(g.items, func(i, j int) bool { return g.items[i].Bytes > g.items[j].Bytes })
			for i, it := range g.items {
				if i == 4 {
					fmt.Fprintf(t.Out, "  %s %s\n", ui.Repeat(" ", 15), t.Muted(fmt.Sprintf("  … and %d more", len(g.items)-4)))
					break
				}
				extra := fmt.Sprintf("idle %dd", it.IdleDays)
				if it.Shared > 0 {
					extra += fmt.Sprintf(" · %s shared with a store", ui.HumanBytes(it.Shared))
				}
				fmt.Fprintf(t.Out, "  %s   %s %s %s\n", ui.Repeat(" ", 15), ui.PadRight(t.Text(ui.MiddleTruncate(ui.Sanitize(it.Title, 200), 34)), 34),
					ui.PadLeft(t.Muted(ui.HumanBytes(it.Bytes)), 10), t.Muted(extra))
			}
		}
	}
	var expBytes int64
	for _, e := range expired {
		expBytes += e.Bytes
	}
	if len(expired) > 0 {
		fmt.Fprintf(t.Out, "  %s %s %s\n", ui.PadRight(t.Badge("expired", t.Pal.Warning), 15), ui.PadRight(t.Bold("quarantine past retention"), 36),
			ui.PadLeft(t.Accent(t.Bold(ui.HumanBytes(expBytes))), 10))
		fmt.Fprintf(t.Out, "  %s %s\n", ui.Repeat(" ", 15), t.Muted("↳ purge "+plural(len(expired), "operation", "operations")+" older than "+ui.HumanDuration(retention)))
	}
	tot := p.Totals()
	t.Rule("")
	summary := fmt.Sprintf("frees now %s %s frees after retention %s",
		t.Ok(t.Bold(ui.HumanBytes(tot.FreesNow+expBytes))), t.Subtle("·"), t.Accent(ui.HumanBytes(tot.AfterRetention)))
	if tot.NotReclaimable > 0 {
		summary += t.Subtle(" · ") + t.Muted("not reclaimable "+ui.HumanBytes(tot.NotReclaimable))
	}
	fmt.Fprintln(t.Out, "  "+summary)
	if p.Partial {
		t.Note("the scan hit its time budget — totals are partial")
	}
	for _, s := range p.Skipped {
		t.Note("skipped %s", s)
	}
}

func expiredTrash(retention time.Duration) []quarantine.Summary {
	sums, _ := quarantine.List()
	var out []quarantine.Summary
	for _, s := range sums {
		if time.Since(s.Created) > retention {
			out = append(out, s)
		}
	}
	return out
}

func (a *App) applyCleanup(ctx context.Context, plan cleanup.Plan, roots []string, expired []quarantine.Summary, retention time.Duration) error {
	t := a.T
	lock, err := platform.AcquireLock(a.Paths.StateDir)
	if errors.Is(err, platform.ErrLocked) {
		return fail(ExitLocked, "E_LOCKED", "another thine operation is applying changes", "wait for it to finish, then retry")
	} else if err != nil {
		return fail(ExitFailure, "E_IO", err.Error(), "")
	}
	defer lock.Release()

	opID := plan.ID
	intent := make([]oplog.Item, 0, len(plan.Items))
	for _, it := range plan.Items {
		intent = append(intent, oplog.Item{ID: it.ID, Action: it.Action, Target: firstNonEmpty(it.Path, strings.Join(it.Argv, " ")), Bytes: it.Bytes})
	}
	if err := oplog.Append(oplog.Record{OpID: opID, Phase: "intent", Verb: "cleanup", Argv: a.argv, Caller: a.Caller, Items: intent}); err != nil {
		return fail(ExitFailure, "E_IO", "couldn't write the operations log — "+err.Error(), "refusing to act without an audit trail")
	}

	var freedExpired int64
	if len(expired) > 0 {
		want := map[string]bool{}
		for _, e := range expired {
			want[e.OpID] = true
		}
		freedExpired, _, _ = quarantine.Purge(func(s quarantine.Summary) bool { return want[s.OpID] })
	}

	var sp *ui.Spinner
	if !a.json() {
		fmt.Fprintln(t.Err)
	}
	outs := cleanup.Apply(ctx, opID, plan, roots, cleanup.Hooks{
		Start: func(it cleanup.Item, i, n int) {
			sp = t.Spin(cleanupVerb(it))
			sp.Detail(fmt.Sprintf("%d/%d", i+1, n))
		},
		Done: func(o cleanup.Outcome) {
			sp.Stop()
			if a.json() {
				return
			}
			name := ui.Sanitize(firstNonEmpty(o.Title, o.Category), 60)
			switch o.Result {
			case "done":
				if o.Action == cleanup.ActQuarantine {
					t.Step(ui.KindOK, fmt.Sprintf("%s %s %s", t.Bold(ui.MiddleTruncate(name, 50)), t.Ok("quarantined"), t.Muted(ui.HumanBytes(o.Bytes))))
				} else {
					t.Step(ui.KindOK, fmt.Sprintf("%s %s %s", t.Bold(name), t.Ok("freed"), t.Accent(ui.HumanBytes(o.Freed))))
				}
			case "skipped":
				t.Step(ui.KindWarn, fmt.Sprintf("%s %s %s", t.Bold(name), t.Warnc("skipped"), t.Muted("— "+o.Error)))
			default:
				t.Step(ui.KindFail, fmt.Sprintf("%s %s %s", t.Bold(name), t.Bad("failed"), t.Muted("— "+ui.Sanitize(o.Error, 200))))
			}
		},
		Sudo: func(argv []string) bool {
			if !t.Interactive() {
				return false
			}
			sp.Stop()
			return t.Confirm("Run " + t.Code("sudo "+strings.Join(argv, " ")) + "?")
		},
	})

	var freed, quarantined int64
	failed, skipped := 0, 0
	items := make([]oplog.Item, 0, len(outs))
	for _, o := range outs {
		freed += o.Freed
		if o.Result == "done" && o.Action == cleanup.ActQuarantine {
			quarantined += o.Bytes
		}
		switch o.Result {
		case "failed":
			failed++
		case "skipped":
			skipped++
		}
		items = append(items, oplog.Item{ID: o.ID, Action: o.Action, Target: firstNonEmpty(o.Path, strings.Join(o.Argv, " ")),
			Bytes: max(o.Freed, o.Bytes*boolInt(o.Action == cleanup.ActQuarantine)), Result: o.Result, Error: o.Error,
			Undoable: o.Action == cleanup.ActQuarantine})
	}
	freed += freedExpired
	result := "completed"
	code := ExitOK
	if failed+skipped > 0 {
		result, code = "partial", ExitPartial
	}
	if ctx.Err() != nil {
		result, code = "aborted", ExitInterrupted
	}
	oplog.Append(oplog.Record{OpID: opID, Phase: "outcome", Verb: "cleanup", Caller: a.Caller, Items: items, Result: result, Bytes: freed + quarantined})
	removePlan(opID)

	if a.json() {
		return a.emitJSON(map[string]any{"op_id": opID, "mode": "applied", "result": result, "freed_bytes": freed,
			"quarantined_bytes": quarantined, "outcomes": outs, "exit_code": code})
	}
	fmt.Fprintln(t.Out)
	var msg string
	switch {
	case freed > 0 && quarantined > 0:
		msg = fmt.Sprintf("%s Reclaimed %s now + %s in quarantine", t.Icons.Party, ui.HumanBytes(freed), ui.HumanBytes(quarantined))
	case freed > 0:
		msg = fmt.Sprintf("%s Reclaimed %s", t.Icons.Party, ui.HumanBytes(freed))
	default:
		msg = fmt.Sprintf("%s Moved %s to quarantine — it frees for good after the retention period", t.Icons.Party, ui.HumanBytes(quarantined))
	}
	if freed >= 1<<30 {
		t.Celebrate(msg + " — your disk thanks you")
	} else {
		t.Step(ui.KindOK, t.Bold(msg))
	}
	if quarantined > 0 {
		t.Sub(t.Muted(fmt.Sprintf("restore anything with `thine undo %s` until %s", opID[:10], time.Now().Add(retention).Format("Jan 2"))))
	}
	if code != ExitOK {
		return silent(code)
	}
	return nil
}

func cleanupVerb(it cleanup.Item) string {
	switch it.Action {
	case cleanup.ActQuarantine:
		return "Quarantining " + ui.MiddleTruncate(ui.Sanitize(it.Title, 200), 40)
	case cleanup.ActExec, cleanup.ActSudo:
		return "Running " + strings.Join(it.Argv, " ")
	}
	return "Emptying " + it.Title
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// Plans are saved so an agent can propose and a human can apply later.

func planPath(id string) string {
	return filepath.Join(platform.GetPaths().StateDir, "plans", filepath.Base(id)+".json")
}

func savePlan(p cleanup.Plan) error {
	dir := filepath.Dir(planPath(p.ID))
	if err := platform.EnsurePrivateDir(dir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(planPath(p.ID), data, 0o600)
}

func removePlan(id string) { os.Remove(planPath(id)) }

func (a *App) applyCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:     "apply <plan-id>",
		Short:   "Apply a saved cleanup plan (e.g. one an agent proposed)",
		GroupID: "reclaim",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(planPath(args[0]))
			if err != nil {
				return fail(ExitNotFound, "E_NOT_FOUND", fmt.Sprintf("no saved plan %q", args[0]), "plans expire after 24 h — run `thine cleanup` again")
			}
			var p cleanup.Plan
			if err := json.Unmarshal(data, &p); err != nil {
				return fail(ExitFailure, "E_IO", "the saved plan is corrupt", "")
			}
			if time.Since(p.CreatedAt) > 24*time.Hour {
				removePlan(p.ID)
				return fail(ExitNotFound, "E_EXPIRED", "that plan is older than 24 h", "run `thine cleanup` for a fresh one")
			}
			if a.isAgent() {
				return fail(ExitRefused, "E_AGENT_POLICY", "plans are applied by a human, not an agent", "ask the user to run `thine apply "+p.ID+"`")
			}
			retention, _ := config.ParseDuration(a.Cfg.Cleanup.Retention)
			if !a.json() {
				a.printCleanupPlan(p, nil, retention)
			}
			if !yes {
				if !a.T.Interactive() {
					return silent(ExitNeedsApply)
				}
				if !a.T.Confirm("Apply this saved plan? Every item is re-checked first.") {
					return nil
				}
			}
			return a.applyCleanup(cmd.Context(), p, a.cleanupRoots(nil), nil, retention)
		},
	}
	c.Flags().BoolVarP(&yes, "yes", "y", false, "don't ask")
	return c
}
