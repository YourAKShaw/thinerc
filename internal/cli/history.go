package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/YourAKShaw/thinerc/internal/config"
	"github.com/YourAKShaw/thinerc/internal/oplog"
	"github.com/YourAKShaw/thinerc/internal/quarantine"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

func (a *App) historyCmd() *cobra.Command {
	var limit int
	c := &cobra.Command{
		Use:     "history",
		Short:   "Show what thine changed: kills, cleanups, undos",
		GroupID: "reclaim",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ops, err := oplog.Ops()
			if err != nil {
				return fail(ExitFailure, "E_IO", err.Error(), "")
			}
			if len(ops) > limit {
				ops = ops[:limit]
			}
			if a.json() {
				return a.emitJSON(map[string]any{"operations": ops})
			}
			t := a.T
			if len(ops) == 0 {
				t.Step(ui.KindInfo, t.Muted("no operations yet — thine has not changed anything on this machine"))
				return nil
			}
			t.Step(ui.KindInfo, t.Bold("History")+t.Muted(" · "+oplog.LogPath()))
			for _, op := range ops {
				res := map[string]string{"completed": t.Ok("✓ completed"), "partial": t.Warnc("◐ partial"),
					"aborted": t.Warnc("◌ aborted"), "incomplete": t.Bad("✗ incomplete")}[op.Result]
				if res == "" {
					res = t.Muted(op.Result)
				}
				undo := ""
				if op.Undoable && op.Verb != "undo" {
					undo = t.Accent(" ↺ undoable")
				}
				size := ""
				if op.Bytes > 0 {
					size = ui.HumanBytes(op.Bytes)
				}
				fmt.Fprintf(t.Out, "  %s %s %s %s %s %s%s\n", t.Muted(op.ID[:10]), ui.PadRight(t.Text(op.Started.Local().Format("Jan 02 15:04")), 13),
					ui.PadRight(t.Bold(op.Verb), 9), ui.PadRight(res, 14), ui.PadLeft(t.Accent(size), 9),
					t.Muted(fmt.Sprintf("%s · %s", plural(len(op.Items), "item", "items"), op.Caller)), undo)
			}
			return nil
		},
	}
	c.Flags().IntVarP(&limit, "limit", "n", 20, "how many operations to show")
	return c
}

func (a *App) undoCmd() *cobra.Command {
	var items []string
	c := &cobra.Command{
		Use:     "undo [op-id]",
		Short:   "Restore what an operation quarantined (default: the latest undoable one)",
		GroupID: "reclaim",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t := a.T
			var op oplog.Op
			var err error
			if len(args) == 1 {
				op, err = oplog.Find(args[0])
				if err != nil {
					return fail(ExitNotFound, "E_NOT_FOUND", err.Error(), "thine history")
				}
			} else {
				ops, _ := oplog.Ops()
				found := false
				for _, o := range ops {
					if o.Undoable && o.Verb != "undo" {
						if sums, _ := quarantine.List(); hasQuarantine(sums, o.ID) {
							op, found = o, true
							break
						}
					}
				}
				if !found {
					return fail(ExitNotFound, "E_NOT_FOUND", "nothing to undo — no quarantined items remain", "thine history")
				}
			}
			only := map[string]bool{}
			for _, i := range items {
				only[i] = true
			}
			for _, it := range op.Items {
				if it.Action == "signal" {
					t.Note("%s (%s) was a process stop — processes can't be un-stopped; start it again", it.Target, it.ID)
				}
			}
			sp := t.Spin("Restoring " + op.ID[:10])
			res, rerr := quarantine.Restore(op.ID, only)
			sp.Stop()
			undoID := oplog.NewID()
			var logged []oplog.Item
			for _, r := range res {
				logged = append(logged, oplog.Item{ID: r.ID, Action: "restore", Target: r.To, Bytes: r.Bytes, Result: "done"})
			}
			oplog.Append(oplog.Record{OpID: undoID, Phase: "outcome", Verb: "undo", Argv: a.argv, Caller: a.Caller, Items: logged, Result: "completed"})
			if a.json() {
				code := ExitOK
				if rerr != nil {
					code = ExitPartial
				}
				return a.emitJSONCode(map[string]any{"op_id": op.ID, "restored": res, "error": errString(rerr)}, code)
			}
			for _, r := range res {
				msg := fmt.Sprintf("%s %s", t.Ok("restored"), t.Bold(ui.MiddleTruncate(ui.Sanitize(r.Original, 300), 60)))
				if r.Conflict {
					msg += t.Warnc(" → "+r.To) + t.Muted(" (the original path was taken)")
				}
				t.Step(ui.KindOK, msg)
			}
			if rerr != nil {
				return fail(ExitPartial, "E_PARTIAL", "some items couldn't be restored — "+rerr.Error(), "")
			}
			if len(res) == 0 {
				t.Step(ui.KindInfo, t.Muted("nothing left to restore for "+op.ID[:10]))
			}
			return nil
		},
	}
	c.Flags().StringSliceVar(&items, "item", nil, "restore only these item IDs")
	return c
}

func hasQuarantine(sums []quarantine.Summary, id string) bool {
	for _, s := range sums {
		if s.OpID == id {
			return true
		}
	}
	return false
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (a *App) trashCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "trash",
		Short:   "Inspect or purge thine's quarantine",
		GroupID: "reclaim",
	}
	c.AddCommand(&cobra.Command{
		Use:   "ls",
		Short: "Show quarantined operations with age and size",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sums, err := quarantine.List()
			if err != nil {
				return fail(ExitFailure, "E_IO", err.Error(), "")
			}
			if a.json() {
				return a.emitJSON(map[string]any{"quarantine": sums, "path": quarantine.Root()})
			}
			t := a.T
			if len(sums) == 0 {
				t.Step(ui.KindOK, t.Muted("quarantine is empty"))
				return nil
			}
			var total int64
			retention, _ := config.ParseDuration(a.Cfg.Cleanup.Retention)
			t.Step(ui.KindInfo, t.Bold("Quarantine")+t.Muted(" · "+quarantine.Root()))
			for _, s := range sums {
				total += s.Bytes
				left := retention - time.Since(s.Created)
				when := t.Muted("purges in " + ui.HumanDuration(max(0, left)))
				if left <= 0 {
					when = t.Warnc("expired — purged on next cleanup")
				}
				fmt.Fprintf(t.Out, "  %s %s %s %s %s\n", t.Muted(s.OpID[:10]), ui.PadRight(t.Bold(s.Verb), 9),
					ui.PadLeft(t.Accent(ui.HumanBytes(s.Bytes)), 10), ui.PadRight(t.Text(plural(s.Items, "item", "items")), 9), when)
			}
			t.Sub(t.Muted("total " + ui.HumanBytes(total) + " · restore with `thine undo <id>`"))
			return nil
		},
	})
	var older, op string
	var all bool
	purge := &cobra.Command{
		Use:   "purge",
		Short: "Permanently delete quarantined items (typed confirmation)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			t := a.T
			var age time.Duration
			if older != "" {
				d, err := config.ParseDuration(older)
				if err != nil {
					return fail(ExitUsage, "E_USAGE", "invalid --older — "+err.Error(), "--older 7d")
				}
				age = d
			}
			if !all && op == "" && older == "" {
				return fail(ExitUsage, "E_USAGE", "say what to purge", "--older 7d · --op <id> · --all")
			}
			sel := func(s quarantine.Summary) bool {
				return all || (op != "" && strings.HasPrefix(s.OpID, strings.ToUpper(op))) || (older != "" && time.Since(s.Created) > age)
			}
			sums, _ := quarantine.List()
			var n int
			var bytes int64
			for _, s := range sums {
				if sel(s) {
					n++
					bytes += s.Bytes
				}
			}
			if n == 0 {
				t.Step(ui.KindOK, t.Muted("nothing matches — quarantine unchanged"))
				return nil
			}
			if a.isAgent() {
				return fail(ExitRefused, "E_AGENT_POLICY", "purging the quarantine is destructive and needs a human", "")
			}
			phrase := fmt.Sprintf("purge %d", n)
			if !t.ConfirmTyped(fmt.Sprintf("This permanently deletes %s (%s). It can't be undone.", plural(n, "operation", "operations"), ui.HumanBytes(bytes)), phrase) {
				t.Note("nothing deleted")
				return silent(ExitNeedsApply)
			}
			freed, k, err := quarantine.Purge(sel)
			id := oplog.NewID()
			oplog.Append(oplog.Record{OpID: id, Phase: "outcome", Verb: "purge", Argv: a.argv, Caller: a.Caller, Bytes: freed, Result: "completed"})
			if err != nil {
				return fail(ExitPartial, "E_PARTIAL", err.Error(), "")
			}
			t.Step(ui.KindOK, fmt.Sprintf("%s purged %s %s", t.Icons.Trash, plural(k, "operation", "operations"), t.Accent(ui.HumanBytes(freed))))
			return nil
		},
	}
	purge.Flags().StringVar(&older, "older", "", "purge operations older than this, e.g. 7d")
	purge.Flags().StringVar(&op, "op", "", "purge one operation by ID prefix")
	purge.Flags().BoolVar(&all, "all", false, "purge everything")
	c.AddCommand(purge)
	return c
}
