package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/YourAKShaw/thinerc/internal/mem"
	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/ports"
	"github.com/YourAKShaw/thinerc/internal/procs"
	"github.com/YourAKShaw/thinerc/internal/quarantine"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

// knownAgents are AI coding agents whose running sessions whatsup counts.
var knownAgents = []string{"claude", "codex", "gemini", "copilot", "grok", "kiro-cli", "opencode", "aider", "cursor-agent", "goose", "amp"}

type dashboard struct {
	Host       string              `json:"host"`
	OS         string              `json:"os"`
	Uptime     time.Duration       `json:"uptime_ns"`
	Load       [3]float64          `json:"load"`
	CPUs       int                 `json:"cpus"`
	Listeners  []ports.Listener    `json:"listeners"`
	ListenProc map[int]string      `json:"-"`
	Memory     *mem.Snapshot       `json:"memory,omitempty"`
	DiskTotal  uint64              `json:"disk_total_bytes"`
	DiskFree   uint64              `json:"disk_free_bytes"`
	Quarantine int64               `json:"quarantine_bytes"`
	Docker     []string            `json:"docker_containers"`
	DockerUp   bool                `json:"docker_running"`
	Agents     map[string][]string `json:"agents"`
	Git        *gitInfo            `json:"git,omitempty"`
}

type gitInfo struct {
	Branch string `json:"branch"`
	Dirty  int    `json:"dirty_files"`
	Ahead  int    `json:"ahead"`
	Behind int    `json:"behind"`
}

func (a *App) whatsupCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "whatsup",
		Short:   "One-screen dashboard: ports, memory, disk, Docker, agents, git",
		GroupID: "reclaim",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sp := a.T.Spin("Gathering the dashboard")
			d := gatherDashboard(cmd.Context())
			sp.Stop()
			if a.json() {
				return a.emitJSON(map[string]any{"dashboard": d})
			}
			a.renderDashboard(d)
			return nil
		},
	}
}

// gatherDashboard collects every section concurrently, each under ~1 s.
func gatherDashboard(parent context.Context) dashboard {
	ctx, cancel := context.WithTimeout(parent, 1500*time.Millisecond)
	defer cancel()
	d := dashboard{OS: platform.OSName(), CPUs: runtime.NumCPU(), Agents: map[string][]string{}, ListenProc: map[int]string{}}
	d.Host, _ = os.Hostname()
	var wg sync.WaitGroup
	var mu sync.Mutex
	run := func(f func()) { wg.Add(1); go func() { defer wg.Done(); f() }() }
	run(func() { d.Load, d.Uptime = platform.LoadAvg(ctx), platform.Uptime(ctx) })
	run(func() {
		ls, _ := ports.All(ctx)
		mu.Lock()
		defer mu.Unlock()
		for _, l := range ls {
			if l.UID == procs.Self() && l.Port >= 1024 {
				d.Listeners = append(d.Listeners, l)
				d.ListenProc[l.PID] = l.Command
			}
		}
	})
	run(func() {
		if s, err := mem.Read(ctx); err == nil {
			mu.Lock()
			d.Memory = &s
			mu.Unlock()
		}
	})
	run(func() {
		d.DiskTotal, d.DiskFree, _ = platform.DiskUsage(platform.Home())
		if sums, err := quarantine.List(); err == nil {
			for _, s := range sums {
				d.Quarantine += s.Bytes
			}
		}
	})
	run(func() {
		if !platform.Have("docker") {
			return
		}
		out, err := platform.Output(ctx, time.Second, "docker", "ps", "--format", "{{.Names}}")
		mu.Lock()
		defer mu.Unlock()
		if err == nil {
			d.DockerUp = true
			for _, n := range strings.Split(out, "\n") {
				if n != "" {
					d.Docker = append(d.Docker, n)
				}
			}
		}
	})
	run(func() {
		all, _ := procs.List()
		found := map[string][]string{}
		for _, p := range all {
			if p.UID != procs.Self() {
				continue
			}
			for _, n := range p.MatchNames() {
				for _, ag := range knownAgents {
					if n == ag {
						where := platform.Tilde(p.Cwd)
						if where == "" {
							where = fmt.Sprintf("pid %d", p.PID)
						}
						found[ag] = append(found[ag], where)
					}
				}
			}
		}
		mu.Lock()
		d.Agents = found
		mu.Unlock()
	})
	run(func() { d.Git = readGit(ctx) })
	wg.Wait()
	return d
}

func readGit(ctx context.Context) *gitInfo {
	if !platform.Have("git") {
		return nil
	}
	out, err := platform.Output(ctx, time.Second, "git", "status", "--porcelain=v2", "--branch")
	if err != nil {
		return nil
	}
	g := &gitInfo{}
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			g.Branch = strings.TrimPrefix(line, "# branch.head ")
		case strings.HasPrefix(line, "# branch.ab "):
			fmt.Sscanf(strings.TrimPrefix(line, "# branch.ab "), "+%d -%d", &g.Ahead, &g.Behind)
		case line != "" && !strings.HasPrefix(line, "#"):
			g.Dirty++
		}
	}
	return g
}

func (a *App) renderDashboard(d dashboard) {
	t := a.T
	stagger := func() {
		if t.Animate {
			time.Sleep(55 * time.Millisecond)
		}
	}
	load := fmt.Sprintf("load %.2f %.2f %.2f", d.Load[0], d.Load[1], d.Load[2])
	fmt.Fprintln(t.Out, t.Brand(t.Icons.Star+" whatsup")+"  "+t.Bold(d.Host)+t.Muted(fmt.Sprintf(" · %s · up %s · %s · %d CPUs", d.OS, ui.HumanDuration(d.Uptime), load, d.CPUs)))
	fmt.Fprintln(t.Out)
	barW := min(30, max(10, t.Width()-50))

	// Ports.
	stagger()
	if len(d.Listeners) == 0 {
		t.Step(ui.KindOK, t.Icons.Port+" "+t.Bold("Ports")+t.Muted(" · no dev ports listening"))
	} else {
		t.Step(ui.KindInfo, t.Icons.Port+" "+t.Bold("Ports")+t.Muted(" · "+plural(len(d.Listeners), "listener", "listeners")))
		sort.Slice(d.Listeners, func(i, j int) bool { return d.Listeners[i].Port < d.Listeners[j].Port })
		var parts []string
		budget := t.Width() - 12
		for i, l := range d.Listeners {
			p := t.Accent(fmt.Sprintf(":%d", l.Port)) + " " + t.Muted(ui.Sanitize(l.Command, 16))
			if used := ui.Width(strings.Join(append(parts, p), "  ·  ")); used > budget && i > 0 {
				parts = append(parts, t.Muted(fmt.Sprintf("+%d more", len(d.Listeners)-i)))
				break
			}
			parts = append(parts, p)
		}
		t.Sub(strings.Join(parts, t.Subtle("  ·  ")))
	}

	// Memory.
	stagger()
	if m := d.Memory; m != nil {
		used := m.UsedPct() / 100
		t.Step(ui.KindInfo, t.Icons.Memory+" "+t.Bold("Memory")+t.Muted(fmt.Sprintf(" · %s available of %s", ui.HumanBytes(m.Available), ui.HumanBytes(m.Total))))
		extra := ""
		if m.HasPressure {
			extra = fmt.Sprintf(" · pressure %.1f%%", m.Pressure)
		}
		if m.SwapTotal > 0 {
			extra += fmt.Sprintf(" · swap %s", ui.HumanBytes(m.SwapUsed))
		}
		t.Sub(t.Bar(used, barW, t.Pal.Success, t.Pal.Warning, t.Pal.Error) + t.Muted(fmt.Sprintf(" %3.0f%% used%s", used*100, extra)))
	}

	// Disk.
	stagger()
	if d.DiskTotal > 0 {
		used := 1 - float64(d.DiskFree)/float64(d.DiskTotal)
		t.Step(ui.KindInfo, t.Icons.Disk+" "+t.Bold("Disk")+t.Muted(fmt.Sprintf(" · %s free on ~ of %s", ui.HumanBytes(int64(d.DiskFree)), ui.HumanBytes(int64(d.DiskTotal)))))
		q := ""
		if d.Quarantine > 0 {
			q = fmt.Sprintf(" · %s in quarantine", ui.HumanBytes(d.Quarantine))
		}
		t.Sub(t.Bar(used, barW, t.Pal.Success, t.Pal.Warning, t.Pal.Error) + t.Muted(fmt.Sprintf(" %3.0f%% used%s · try `cleanup`", used*100, q)))
	}

	// Docker.
	stagger()
	switch {
	case !platform.Have("docker"):
	case !d.DockerUp:
		t.Step(ui.KindNeutral, t.Icons.Docker+" "+t.Bold("Docker")+t.Muted(" · daemon not running"))
	default:
		t.Step(ui.KindInfo, t.Icons.Docker+" "+t.Bold("Docker")+t.Muted(" · "+plural(len(d.Docker), "running container", "running containers")))
		if len(d.Docker) > 0 {
			t.Sub(t.Text(strings.Join(d.Docker, ", ")))
		}
	}

	// Agents.
	stagger()
	if len(d.Agents) == 0 {
		t.Step(ui.KindNeutral, t.Icons.Agent+" "+t.Bold("Agents")+t.Muted(" · none running"))
	} else {
		names := make([]string, 0, len(d.Agents))
		total := 0
		for n, w := range d.Agents {
			names = append(names, n)
			total += len(w)
		}
		sort.Strings(names)
		t.Step(ui.KindInfo, t.Icons.Agent+" "+t.Bold("Agents")+t.Muted(" · "+plural(total, "session", "sessions")))
		for _, n := range names {
			where := uniqueStrings(d.Agents[n])
			t.Sub(t.Accent(n) + t.Muted(fmt.Sprintf(" ×%d · ", len(d.Agents[n]))) + t.Text(ui.Truncate(strings.Join(where, ", "), max(20, t.Width()-24))))
		}
	}

	// Git.
	stagger()
	if g := d.Git; g != nil && g.Branch != "" {
		cwd, _ := os.Getwd()
		state := t.Ok("clean")
		if g.Dirty > 0 {
			state = t.Warnc(plural(g.Dirty, "changed file", "changed files"))
		}
		ab := ""
		if g.Ahead > 0 {
			ab += fmt.Sprintf(" · ↑%d", g.Ahead)
		}
		if g.Behind > 0 {
			ab += fmt.Sprintf(" · ↓%d", g.Behind)
		}
		t.Step(ui.KindInfo, t.Icons.Git+" "+t.Bold("Git")+t.Muted(" · "+filepath.Base(cwd)))
		t.Sub(t.Accent(g.Branch) + t.Muted(" · ") + state + t.Muted(ab))
	}
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
