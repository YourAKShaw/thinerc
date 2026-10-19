package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/YourAKShaw/thinerc/internal/cleanup"
	"github.com/YourAKShaw/thinerc/internal/oplog"
	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/procs"
	"github.com/YourAKShaw/thinerc/internal/quarantine"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

// pkgManager is a detected Node package manager.
type pkgManager struct {
	Name     string
	Lockfile string
	Install  []string
	Frozen   []string
}

// detectPM picks the package manager from the lockfile (plan §7.6).
func detectPM(root string) (pkgManager, bool) {
	has := func(f string) bool { _, err := os.Stat(filepath.Join(root, f)); return err == nil }
	berry := has(".yarnrc.yml")
	if b, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		var pj struct {
			PackageManager string `json:"packageManager"`
		}
		if json.Unmarshal(b, &pj) == nil && strings.HasPrefix(pj.PackageManager, "yarn@") && !strings.HasPrefix(pj.PackageManager, "yarn@1") {
			berry = true
		}
	}
	switch {
	case has("pnpm-lock.yaml"):
		return pkgManager{"pnpm", "pnpm-lock.yaml", []string{"pnpm", "install"}, []string{"pnpm", "install", "--frozen-lockfile"}}, true
	case has("bun.lock") || has("bun.lockb"):
		return pkgManager{"bun", "bun.lock", []string{"bun", "install"}, []string{"bun", "install", "--frozen-lockfile"}}, true
	case has("yarn.lock") && berry:
		return pkgManager{"yarn", "yarn.lock", []string{"yarn", "install"}, []string{"yarn", "install", "--immutable"}}, true
	case has("yarn.lock"):
		return pkgManager{"yarn", "yarn.lock", []string{"yarn", "install"}, []string{"yarn", "install", "--frozen-lockfile"}}, true
	case has("package-lock.json"):
		return pkgManager{"npm", "package-lock.json", []string{"npm", "install"}, []string{"npm", "ci"}}, true
	}
	return pkgManager{"npm", "", []string{"npm", "install"}, []string{"npm", "install"}}, false
}

// findNodeRoot walks up to the nearest package.json, preferring one with a lockfile.
func findNodeRoot(dir string) string {
	first := ""
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "package.json")); err == nil {
			if first == "" {
				first = d
			}
			if _, ok := detectPM(d); ok {
				return d
			}
		}
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil || filepath.Dir(d) == d || d == platform.Home() {
			return first
		}
	}
}

// workspaceNodeModules lists every node_modules in the project, never
// descending into one.
func workspaceNodeModules(root string) []string {
	var out []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if p != root && (d.Name() == ".git" || strings.Count(p[len(root):], string(filepath.Separator)) > 6) {
			return fs.SkipDir
		}
		if d.Name() == "node_modules" {
			out = append(out, p)
			return fs.SkipDir
		}
		return nil
	})
	return out
}

func (a *App) nmCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "nm",
		Short:   "node_modules tools (reset = nuke-nm)",
		GroupID: "reclaim",
	}
	var yes, frozen, purgeOld bool
	reset := &cobra.Command{
		Use:     "reset",
		Aliases: []string{"nuke"},
		Short:   "Reinstall node_modules from the lockfile; roll back automatically if install fails",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.runNMReset(cmd.Context(), yes, frozen, purgeOld)
		},
	}
	reset.Flags().BoolVarP(&yes, "yes", "y", false, "don't ask")
	reset.Flags().BoolVar(&frozen, "frozen", false, "strict lockfile install (npm ci, --frozen-lockfile, --immutable)")
	reset.Flags().BoolVar(&purgeOld, "purge-old", false, "delete the old tree right away instead of keeping it in quarantine")
	c.AddCommand(reset)
	return c
}

func (a *App) runNMReset(ctx context.Context, yes, frozen, purgeOld bool) error {
	t := a.T
	cwd, _ := os.Getwd()
	root := findNodeRoot(cwd)
	if root == "" {
		return fail(ExitNotFound, "E_NOT_FOUND", "no package.json here or in any parent", "cd into a Node project first")
	}
	pm, locked := detectPM(root)
	argv := pm.Install
	if frozen {
		argv = pm.Frozen
	}
	if !platform.Have(argv[0]) {
		return fail(ExitNotFound, "E_NOT_FOUND", argv[0]+" is not installed", "install "+argv[0]+" or remove its lockfile")
	}
	sp := t.Spin("Sizing node_modules")
	dirs := workspaceNodeModules(root)
	var total int64
	for _, d := range dirs {
		s, _ := cleanup.DirSize(ctx, d)
		total += s.Bytes
	}
	all, _ := procs.List()
	var busy []procs.Proc
	for _, p := range all {
		if p.UID == procs.Self() && p.Cwd != "" && (p.Cwd == root || strings.HasPrefix(p.Cwd, root+string(filepath.Separator))) && p.PID != os.Getpid() && !procs.IsShellish(p) && !procs.Ancestors()[p.PID] {
			busy = append(busy, p)
		}
	}
	sp.Stop()

	t.Step(ui.KindNeutral, fmt.Sprintf("%s %s %s", t.Bold("nuke-nm"), t.Text(platform.Tilde(root)), t.Badge(pm.Name, t.Pal.Info)))
	lock := pm.Lockfile
	if !locked {
		lock = "no lockfile — versions may drift"
	}
	t.Sub(fmt.Sprintf("%s %s %s", t.Text("quarantine "+plural(len(dirs), "node_modules", "node_modules dirs")), t.Accent(ui.HumanBytes(total)), t.Muted("("+lock+")")))
	t.Indent(t.Text("run ") + t.Code(strings.Join(argv, " ")))
	t.Indent(t.Muted("if the install fails, the old node_modules come back automatically"))
	if len(busy) > 0 {
		names := []string{}
		for _, p := range busy {
			names = append(names, fmt.Sprintf("%s(%d)", ui.Sanitize(p.Name, 20), p.PID))
		}
		t.Warn("running in this project: %s — stop them first (killfb) or they may break mid-install", strings.Join(names, " "))
	}
	if a.isAgent() {
		return fail(ExitRefused, "E_AGENT_POLICY", "nuke-nm rewrites dependencies and needs a human", "")
	}
	if !yes {
		if !t.Interactive() {
			t.Note("re-run with --yes (no terminal to confirm)")
			return silent(ExitNeedsApply)
		}
		if !t.Confirm("Reinstall now?") {
			return nil
		}
	}

	lk, err := platform.AcquireLock(a.Paths.StateDir)
	if err != nil {
		return fail(ExitLocked, "E_LOCKED", err.Error(), "")
	}
	defer lk.Release()
	opID := oplog.NewID()
	var items []oplog.Item
	for _, d := range dirs {
		items = append(items, oplog.Item{ID: "nm:" + d, Action: "quarantine", Target: d, Undoable: true})
	}
	oplog.Append(oplog.Record{OpID: opID, Phase: "intent", Verb: "nm-reset", Argv: a.argv, Caller: a.Caller, Items: items})

	// 1 — move every node_modules aside (all or nothing).
	for i, d := range dirs {
		if _, err := quarantine.Move(opID, "nm-reset", "nm:"+d, d, 0); err != nil {
			quarantine.Restore(opID, nil)
			oplog.Append(oplog.Record{OpID: opID, Phase: "outcome", Verb: "nm-reset", Caller: a.Caller, Result: "failed"})
			return fail(ExitFailure, "E_IO", fmt.Sprintf("couldn't move %s aside — %v", platform.Tilde(d), err), "nothing changed; everything was put back")
		}
		items[i].Result = "done"
	}

	// 2 — install, streaming the tool's own output.
	fmt.Fprintln(t.Err)
	t.StepTo(t.Err, ui.KindInfo, t.Code(strings.Join(argv, " ")))
	start := time.Now()
	code, ierr := platform.Stream(ctx, root, prefixWriter(t.Err, "  "+t.Subtle("│")+" "), argv...)
	if ierr != nil || code != 0 {
		// 3 — roll back: park the partial tree for inspection, restore the old one.
		for _, d := range workspaceNodeModules(root) {
			cleanup.RemoveTrashedDir(opID+"-failed", d)
		}
		res, rerr := quarantine.Restore(opID, nil)
		oplog.Append(oplog.Record{OpID: opID, Phase: "outcome", Verb: "nm-reset", Caller: a.Caller, Items: items, Result: "partial"})
		t.Step(ui.KindFail, t.Bad(fmt.Sprintf("%s exited %d", argv[0], code))+t.Muted(fmt.Sprintf(" — restored %d previous node_modules", len(res))))
		if rerr != nil {
			t.Warn("restore problem: %v", rerr)
		}
		t.Sub(t.Muted("the failed partial install is kept for inspection: `thine trash ls`"))
		return silent(ExitPartial)
	}
	oplog.Append(oplog.Record{OpID: opID, Phase: "outcome", Verb: "nm-reset", Caller: a.Caller, Items: items, Result: "completed", Bytes: total})
	if purgeOld {
		quarantine.Purge(func(s quarantine.Summary) bool { return s.OpID == opID })
	}
	fmt.Fprintln(t.Out)
	t.Celebrate(fmt.Sprintf("%s fresh node_modules in %s", t.Icons.Spark, ui.HumanDuration(time.Since(start))))
	if !purgeOld {
		t.Sub(t.Muted(fmt.Sprintf("old tree (%s) kept in quarantine · `thine undo %s` to roll back", ui.HumanBytes(total), opID[:10])))
	}
	return nil
}

// prefixWriter indents a subprocess's output and strips its escape codes so
// its spinners can't fight ours.
func prefixWriter(w io.Writer, prefix string) io.Writer {
	return &prefixed{w: w, prefix: prefix, bol: true}
}

type prefixed struct {
	w      io.Writer
	prefix string
	bol    bool
}

func (p *prefixed) Write(b []byte) (int, error) {
	for _, line := range strings.SplitAfter(string(b), "\n") {
		if line == "" {
			continue
		}
		if p.bol {
			io.WriteString(p.w, p.prefix)
		}
		io.WriteString(p.w, ui.Sanitize(strings.TrimRight(line, "\n"), 0))
		p.bol = strings.HasSuffix(line, "\n")
		if p.bol {
			io.WriteString(p.w, "\n")
		}
	}
	return len(b), nil
}
