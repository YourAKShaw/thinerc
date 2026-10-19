package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/YourAKShaw/thinerc/internal/config"
	"github.com/YourAKShaw/thinerc/internal/oplog"
	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/procs"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

type killOpts struct {
	groups              []string
	pids                []int
	scope               string
	grace               string
	force, docker, yes  bool
	dryRun, sudo, apply bool
}

// killTarget is one thing to stop: a process (plus its job) or a container.
type killTarget struct {
	Port      int
	Proc      *procs.Proc
	Members   []procs.Proc // processes to signal (job scope ⊇ Proc)
	Tier      procs.Tier
	Reason    string
	Container string
	ContID    string
	Result    string // terminated | killed | survived | gone | skipped | refused | stopped | failed
	Signal    string
	Elapsed   time.Duration
}

func (a *App) killCmd() *cobra.Command {
	var o killOpts
	c := &cobra.Command{
		Use:     "kill [port...]",
		Aliases: []string{"kp", "killport"},
		Short:   "Gracefully free ports: SIGTERM, grace period, then SIGKILL for dev processes",
		GroupID: "ports",
		Example: "  kp 3000\n  killfb && pnpm dev\n  thine kill --group db --dry-run\n  thine kill 8080 --docker",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runKill(cmd.Context(), args, o)
		},
	}
	f := c.Flags()
	f.StringSliceVarP(&o.groups, "group", "g", nil, "port group(s) from config, e.g. fb")
	f.IntSliceVar(&o.pids, "pid", nil, "target process ID(s) directly")
	f.StringVar(&o.scope, "scope", "", "listener | job (default: job for groups, listener for ports)")
	f.StringVar(&o.grace, "grace", "", "time to wait after SIGTERM (default from config, 3s)")
	f.BoolVar(&o.force, "force", false, "allow SIGKILL for non-dev processes that ignore SIGTERM")
	f.BoolVar(&o.docker, "docker", false, "stop Docker containers that publish the port")
	f.BoolVarP(&o.yes, "yes", "y", false, "confirm non-dev targets without asking")
	f.BoolVar(&o.dryRun, "dry-run", false, "show the plan, change nothing")
	f.BoolVar(&o.apply, "apply", false, "apply even without a terminal (needed for confirm-tier targets)")
	f.BoolVar(&o.sudo, "sudo", false, "allow stopping another user's process via sudo kill")
	return c
}

func (a *App) runKill(ctx context.Context, args []string, o killOpts) error {
	t := a.T
	want, err := parsePorts(args)
	if err != nil {
		return err
	}
	for _, g := range o.groups {
		grp, ok := a.Cfg.Groups[g]
		if !ok {
			return fail(ExitNotFound, "E_NOT_FOUND", fmt.Sprintf("no port group named %q", g), "define it under [groups."+g+"] in "+platform.Tilde(a.Cfg.GlobalPath))
		}
		want = append(want, grp.Ports...)
	}
	want = uniqueInts(want)
	if len(want) == 0 && len(o.pids) == 0 {
		return fail(ExitUsage, "E_USAGE", "nothing to stop — give a port, --group or --pid", "kp 3000 · killfb · thine kill --pid 4123")
	}
	grace, err := config.ParseDuration(firstNonEmpty(o.grace, a.Cfg.Kill.Grace))
	if err != nil {
		return fail(ExitUsage, "E_USAGE", "invalid --grace — "+err.Error(), "--grace 3s")
	}
	scope := o.scope
	if scope == "" && a.Cfg.Kill.Scope != "auto" {
		scope = a.Cfg.Kill.Scope
	}
	if scope == "" {
		scope = "listener"
		if len(o.groups) > 0 {
			scope = "job"
		}
	}

	// 1 — resolve.
	pol := a.policy()
	sp := t.Spin("Resolving " + plural(len(want), "port", "ports"))
	rows, err := a.resolvePorts(ctx, want, pol)
	var all []procs.Proc
	if err == nil && scope == "job" {
		all, _ = procs.List()
	}
	sp.Stop()
	if err != nil {
		return err
	}

	var targets []*killTarget
	seenPID := map[int]bool{}
	for _, r := range rows {
		kt := &killTarget{Port: r.Port, Tier: r.Tier, Reason: r.Reason}
		switch {
		case r.External != nil:
			kt.Container, kt.ContID = r.External.Container, r.External.ID
			kt.Tier, kt.Reason = procs.TierConfirm, "port is published by a Docker container"
		case r.Proc == nil:
			kt.Tier = procs.TierProtected
			kt.Reason = r.Reason + " — find it with: sudo ss -ltnp 'sport = :" + strconv.Itoa(r.Port) + "'"
		default:
			if seenPID[r.PID] {
				continue
			}
			seenPID[r.PID] = true
			kt.Proc = r.Proc
			kt.Members = []procs.Proc{*r.Proc}
			if scope == "job" && kt.Tier == procs.TierDev {
				kt.Members = pol.JobScope(*r.Proc, all)
			}
		}
		targets = append(targets, kt)
	}
	for _, pid := range o.pids {
		if seenPID[pid] {
			continue
		}
		p, err := procs.Lookup(pid)
		if err != nil {
			return fail(ExitNotFound, "E_NOT_FOUND", fmt.Sprintf("no process with PID %d", pid), "thine ports")
		}
		tier, why := pol.Classify(p, false)
		targets = append(targets, &killTarget{Proc: &p, Members: []procs.Proc{p}, Tier: tier, Reason: why})
	}

	if len(targets) == 0 {
		if a.json() {
			return a.emitJSON(map[string]any{"mode": "noop", "requested_ports": want, "targets": []any{}, "ports_free": want, "ports_held": []int{}, "exit_code": 0})
		}
		t.Step(ui.KindOK, fmt.Sprintf("%s %s", t.Bold(portList(want)), t.Muted(pick(len(want) == 1, "is already free", "are already free"))))
		return nil
	}

	// 2 — show the plan.
	needConfirm, refused := 0, 0
	for _, kt := range targets {
		if kt.Tier == procs.TierProtected {
			refused++
		} else if kt.Tier == procs.TierConfirm {
			if kt.Proc != nil && kt.Proc.UID != procs.Self() && !o.sudo {
				kt.Tier = procs.TierProtected
				kt.Reason += " → re-run with --sudo to stop it"
				refused++
				continue
			}
			needConfirm++
		}
	}
	if !a.json() {
		for _, kt := range targets {
			a.printKillPlan(kt, scope)
		}
	}

	// 3 — decide.
	exec := make([]*killTarget, 0, len(targets))
	for _, kt := range targets {
		if kt.Tier == procs.TierProtected {
			kt.Result = "refused"
			continue
		}
		exec = append(exec, kt)
	}
	if o.dryRun {
		if a.json() {
			return a.emitJSON(killJSON("dry-run", "", want, targets, nil, nil, 0))
		}
		t.Note("dry run — nothing was signalled")
		return nil
	}
	if needConfirm > 0 {
		agentDeny := a.isAgent() && a.Cfg.Agents.Policy.Kill != "allow-all"
		switch {
		case agentDeny:
			for _, kt := range exec {
				if kt.Tier == procs.TierConfirm {
					kt.Result = "refused"
				}
			}
			exec = filterTargets(exec, func(k *killTarget) bool { return k.Tier == procs.TierDev })
			t.Note("agent mode: non-dev targets need a human — run the same command yourself")
			refused += needConfirm
		case o.yes || o.apply:
		case t.Interactive():
			fmt.Fprintln(t.Err)
			if !t.Confirm(fmt.Sprintf("Stop %s that %s not a plain dev process?", plural(needConfirm, "target", "targets"), pick(needConfirm == 1, "is", "are"))) {
				exec = filterTargets(exec, func(k *killTarget) bool { return k.Tier == procs.TierDev })
				for _, kt := range targets {
					if kt.Tier == procs.TierConfirm {
						kt.Result = "skipped"
					}
				}
			}
		default:
			if a.json() {
				a.emitJSON(killJSON("needs-approval", "", want, targets, nil, nil, ExitNeedsApply))
			} else {
				t.Note("confirm-tier targets need --apply (no terminal to ask)")
			}
			return silent(ExitNeedsApply)
		}
	}
	if len(exec) == 0 {
		if a.json() {
			return a.emitJSON(killJSON("refused", "", want, targets, nil, want, ExitRefused))
		}
		t.Step(ui.KindFail, t.Bad("nothing was stopped")+t.Muted(" — every target is protected or was declined"))
		return silent(ExitRefused)
	}

	// 4 — apply, write-ahead logged.
	opID := oplog.NewID()
	intent := make([]oplog.Item, 0, len(exec))
	for _, kt := range exec {
		intent = append(intent, oplog.Item{ID: kt.id(), Action: kt.action(), Target: kt.describe()})
	}
	oplog.Append(oplog.Record{OpID: opID, Phase: "intent", Verb: "kill", Argv: a.argv, Caller: a.Caller, Items: intent})
	start := time.Now()
	if !a.json() {
		fmt.Fprintln(t.Err)
	}
	for _, kt := range exec {
		if kt.Container != "" {
			a.stopContainer(ctx, kt, o.docker)
			continue
		}
		a.stopProcess(kt, grace, o)
	}

	// 5 — verify the ports, report.
	after, _ := a.resolvePorts(ctx, want, pol)
	held := map[int]bool{}
	for _, r := range after {
		held[r.Port] = true
	}
	var free, stillHeld []int
	for _, p := range want {
		if held[p] {
			stillHeld = append(stillHeld, p)
		} else {
			free = append(free, p)
		}
	}
	code := ExitOK
	result := "completed"
	if len(stillHeld) > 0 {
		code, result = ExitPartial, "partial"
	}
	if len(stillHeld) == 0 && refused > 0 && len(free) == 0 {
		code = ExitRefused
	}
	items := make([]oplog.Item, 0, len(exec))
	for _, kt := range exec {
		items = append(items, oplog.Item{ID: kt.id(), Action: kt.action(), Target: kt.describe(), Result: kt.logResult()})
	}
	oplog.Append(oplog.Record{OpID: opID, Phase: "outcome", Verb: "kill", Caller: a.Caller, Items: items, Result: result})

	if a.json() {
		return a.emitJSON(killJSON("applied", opID, want, targets, free, stillHeld, code))
	}
	elapsed := ui.HumanDuration(time.Since(start))
	if len(want) > 0 {
		if len(stillHeld) == 0 {
			t.Step(ui.KindOK, t.Ok(fmt.Sprintf("%s %s", portList(want), pick(len(want) == 1, "is free", "are free")))+t.Muted(" ("+elapsed+")"))
		} else {
			t.Step(ui.KindFail, t.Bad(portList(stillHeld)+" still held")+t.Muted(" — see `thine port "+joinInts(stillHeld, " ")+"`"))
		}
	} else {
		t.Step(ui.KindOK, t.Muted("done ("+elapsed+")"))
	}
	if code != ExitOK {
		return silent(code)
	}
	return nil
}

func (a *App) printKillPlan(kt *killTarget, scope string) {
	t := a.T
	port := ""
	if kt.Port > 0 {
		port = t.Accent(t.Bold(fmt.Sprintf(":%d", kt.Port))) + " "
	}
	switch {
	case kt.Container != "":
		t.Step(ui.KindInfo, fmt.Sprintf("%sDocker container %s %s", port, t.Bold(ui.Sanitize(kt.Container, 60)), t.Badge("docker", t.Pal.Info)))
		t.Sub(t.Muted("plan: docker stop " + kt.ContID))
	case kt.Proc == nil:
		t.Step(ui.KindFail, port+t.Bad("can't stop")+" "+t.Badge("refused", t.Pal.Error))
		t.Sub(t.Muted(kt.Reason))
	default:
		p := kt.Proc
		badge := map[procs.Tier]string{
			procs.TierDev:       t.Badge("dev", t.Pal.Success),
			procs.TierConfirm:   t.Badge("confirm", t.Pal.Warning),
			procs.TierProtected: t.Badge("refused", t.Pal.Error),
		}[kt.Tier]
		t.Step(kindForTier(kt.Tier), fmt.Sprintf("%s%s %s %s", port, t.Bold(ui.Sanitize(p.Name, 40)), t.Muted(fmt.Sprintf("(%d)", p.PID)), badge))
		t.Sub(t.Text(ui.Truncate(ui.Sanitize(p.CommandLine(), 300), max(20, t.Width()-8))))
		if kt.Tier != procs.TierDev {
			t.Indent(t.Muted(kt.Reason))
		}
		if len(kt.Members) > 1 {
			names := []string{}
			for _, m := range kt.Members {
				names = append(names, fmt.Sprintf("%s(%d)", ui.Sanitize(m.Name, 20), m.PID))
			}
			t.Indent(t.Muted(fmt.Sprintf("job scope · %s: %s", plural(len(kt.Members), "process", "processes"), strings.Join(names, " "))))
		}
	}
}

// stopProcess sends SIGTERM to every member, waits up to grace, then escalates
// to SIGKILL for dev targets (or with --force).
func (a *App) stopProcess(kt *killTarget, grace time.Duration, o killOpts) {
	t := a.T
	start := time.Now()
	label := fmt.Sprintf("%s (%d)", ui.Sanitize(kt.Proc.Name, 30), kt.Proc.PID)
	signalled := 0
	for _, m := range kt.Members {
		err := procs.Signal(m.Identity, syscall.SIGTERM)
		switch {
		case err == nil:
			signalled++
		case errors.Is(err, procs.ErrIdentityChanged):
			if m.PID == kt.Proc.PID {
				kt.Result = "skipped"
				t.Step(ui.KindWarn, label+t.Warnc(" skipped")+t.Muted(" — the PID now belongs to a different process"))
				return
			}
		case errors.Is(err, procs.ErrPermission) && o.sudo:
			t.Note("sudo kill -TERM %d", m.PID)
			if _, serr := platform.Sudo(context.Background(), "kill", "-TERM", strconv.Itoa(m.PID)); serr == nil {
				signalled++
			}
		case errors.Is(err, procs.ErrGone):
		default:
			kt.Result = "failed"
			t.Step(ui.KindFail, label+t.Bad(" couldn't be signalled")+t.Muted(" — "+err.Error()))
			return
		}
	}
	kt.Signal = "TERM"
	allGone := func() bool {
		for _, m := range kt.Members {
			if procs.Alive(m.Identity) {
				return false
			}
		}
		return true
	}
	gone := t.Countdown("Stopping "+label, grace, 50*time.Millisecond, allGone)
	if !gone && (kt.Tier == procs.TierDev || o.force) {
		for _, m := range kt.Members {
			procs.Signal(m.Identity, syscall.SIGKILL)
		}
		kt.Signal = "KILL"
		gone = t.Countdown("Forcing "+label, time.Second, 50*time.Millisecond, allGone)
	}
	kt.Elapsed = time.Since(start)
	switch {
	case gone && kt.Signal == "KILL":
		kt.Result = "killed"
	case gone && signalled == 0:
		kt.Result = "gone"
	case gone:
		kt.Result = "terminated"
	default:
		kt.Result = "survived"
	}
	port := ""
	if kt.Port > 0 {
		port = fmt.Sprintf(":%d ", kt.Port)
	}
	if gone {
		t.Pulse("  "+port, t.Pal.Success)
		how := "stopped"
		if kt.Result == "gone" {
			how = "already stopped with its job"
		}
		if kt.Signal == "KILL" {
			how = "force-stopped (ignored SIGTERM)"
		}
		t.Step(ui.KindOK, fmt.Sprintf("%s%s %s %s", t.Accent(port), t.Bold(label), t.Ok(how), t.Muted("in "+ui.HumanDuration(kt.Elapsed))))
	} else {
		hint := "re-run with --force to send SIGKILL"
		t.Step(ui.KindFail, fmt.Sprintf("%s%s %s", t.Accent(port), t.Bold(label), t.Bad("is still running")+t.Muted(" — "+hint)))
	}
}

func (a *App) stopContainer(ctx context.Context, kt *killTarget, approved bool) {
	t := a.T
	if !approved {
		kt.Result = "skipped"
		t.Step(ui.KindWarn, fmt.Sprintf("container %s skipped %s", t.Bold(kt.Container), t.Muted("— add --docker to run docker stop")))
		return
	}
	sp := t.Spin("docker stop " + kt.Container)
	_, err := platform.Run(ctx, 30*time.Second, "docker", "stop", kt.ContID)
	sp.Stop()
	if err != nil {
		kt.Result = "failed"
		t.Step(ui.KindFail, "docker stop "+kt.Container+t.Bad(" failed")+t.Muted(" — "+err.Error()))
		return
	}
	kt.Result = "stopped"
	t.Step(ui.KindOK, fmt.Sprintf("%s container %s %s", t.Icons.Docker, t.Bold(kt.Container), t.Ok("stopped")))
}

func (kt *killTarget) id() string {
	if kt.Container != "" {
		return "docker:" + kt.ContID
	}
	return fmt.Sprintf("pid:%d", kt.Proc.PID)
}

func (kt *killTarget) action() string {
	if kt.Container != "" {
		return "exec"
	}
	return "signal"
}

func (kt *killTarget) describe() string {
	if kt.Container != "" {
		return "docker stop " + kt.ContID
	}
	return fmt.Sprintf("%s (%d) :%d", ui.Sanitize(kt.Proc.Name, 60), kt.Proc.PID, kt.Port)
}

func (kt *killTarget) logResult() string {
	switch kt.Result {
	case "terminated", "killed", "gone", "stopped":
		return "done"
	case "skipped", "refused":
		return "skipped"
	}
	return "failed"
}

func killJSON(mode, opID string, want []int, targets []*killTarget, free, held []int, code int) map[string]any {
	ts := make([]map[string]any, 0, len(targets))
	for _, kt := range targets {
		m := map[string]any{"port": kt.Port, "tier": string(kt.Tier), "reason": kt.Reason, "result": kt.Result, "signal": kt.Signal, "elapsed_ms": kt.Elapsed.Milliseconds()}
		if kt.Proc != nil {
			m["pid"], m["start_time"], m["name"] = kt.Proc.PID, kt.Proc.StartTime, ui.Sanitize(kt.Proc.Name, 100)
			members := []int{}
			for _, mm := range kt.Members {
				members = append(members, mm.PID)
			}
			m["group_members"] = members
		}
		if kt.Container != "" {
			m["container"], m["container_id"] = kt.Container, kt.ContID
		}
		ts = append(ts, m)
	}
	if free == nil {
		free = []int{}
	}
	if held == nil {
		held = []int{}
	}
	return map[string]any{"op_id": opID, "mode": mode, "requested_ports": want, "targets": ts, "ports_free": free, "ports_held": held, "exit_code": code}
}

func filterTargets(in []*killTarget, keep func(*killTarget) bool) []*killTarget {
	var out []*killTarget
	for _, k := range in {
		if keep(k) {
			out = append(out, k)
		}
	}
	return out
}

func uniqueInts(in []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Ints(out)
	return out
}

func portList(ps []int) string {
	parts := make([]string, len(ps))
	for i, p := range ps {
		parts[i] = fmt.Sprintf(":%d", p)
	}
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

func joinInts(in []int, sep string) string {
	s := make([]string, len(in))
	for i, v := range in {
		s[i] = strconv.Itoa(v)
	}
	return strings.Join(s, sep)
}

func pick(c bool, a, b string) string {
	if c {
		return a
	}
	return b
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
