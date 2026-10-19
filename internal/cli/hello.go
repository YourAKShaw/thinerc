package cli

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"os/user"
	"time"

	"github.com/spf13/cobra"

	"github.com/YourAKShaw/thinerc/internal/buildinfo"
	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/ports"
	"github.com/YourAKShaw/thinerc/internal/procs"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

var tips = []string{
	"`killfb && pnpm dev` — killfb exits 0 once both ports are free, so it chains",
	"`whoport 3000` shows the process, its command line and how long it has run",
	"`cleanup` is a dry run until you say yes; `thine undo` restores quarantined files",
	"`freeup` shows *available* memory — the number that actually matters",
	"`thine theme ls` previews nine palettes; `thine theme set dracula` switches",
	"`thine fonts` installs Fira Code or Victor Mono Nerd Fonts for ligatures and cursive italics",
	"`thine import` turns the aliases in your .bashrc into typed, reviewable recipes",
	"`thine fx all` plays every animation — confetti, rain, fireworks",
	"add `--json` to any read command; agents and scripts get a versioned schema",
	"`thine doctor` explains which shortcuts your own aliases shadow, and where",
}

func (a *App) helloCmd() *cobra.Command {
	var greeting bool
	c := &cobra.Command{
		Use:     "hello",
		Short:   "Play the animated welcome screen",
		GroupID: "look",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if greeting {
				a.greeting()
				return nil
			}
			return a.hello(true)
		},
	}
	c.Flags().BoolVar(&greeting, "greeting", false, "one fast line for new shells (no animation, no scans)")
	return c
}

// hello is the bare-`thine` experience: splash, welcome box, live status, tip.
func (a *App) hello(force bool) error {
	t := a.T
	if t.Opts.Splash || force {
		t.Splash("the safe superpower layer for your terminal · " + buildinfo.Get().Version)
		fmt.Fprintln(t.Err)
	}
	name := "friend"
	if u, err := user.Current(); err == nil && u.Username != "" {
		name = u.Username
	}
	cwd, _ := os.Getwd()
	lines := []string{
		t.Brand(t.Icons.Star) + " " + t.Bold("Welcome to thine") + t.Muted(", "+name+"!"),
		"",
		t.Muted("  ") + t.Code("thine --help") + t.Muted(" for every verb · ") + t.Code("thine doctor") + t.Muted(" for a health check"),
		"",
		t.Muted("  cwd: ") + t.Text(platform.Tilde(cwd)),
		t.Muted("  os:  ") + t.Text(platform.OSName()) + t.Muted(" · shell: ") + t.Text(platform.ShellName()),
	}
	fmt.Fprint(t.Out, t.Box("", lines, t.Pal.Brand[0]))
	fmt.Fprintln(t.Out)

	// Live status — cheap reads only (no Docker, no scans).
	sp := t.Spin("Taking the pulse")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ls, _ := ports.All(ctx)
	devPorts := 0
	for _, l := range ls {
		if l.UID == procs.Self() && l.Port >= 1024 {
			devPorts++
		}
	}
	_, avail, _ := platform.DiskUsage(platform.Home())
	sp.Stop()
	t.Step(ui.KindInfo, t.Bold("Right now"))
	t.Sub(fmt.Sprintf("%s %s listening %s %s free on ~",
		t.Icons.Port, t.Accent(plural(devPorts, "dev port", "dev ports")),
		t.Subtle("·"), t.Accent(ui.HumanBytes(int64(avail)))))
	if a.Cfg.ProjectPath != "" {
		t.Sub(fmt.Sprintf("%s project config %s (%s)", t.Icons.Lock, platform.Tilde(a.Cfg.ProjectPath), a.Cfg.ProjectState))
	}
	fmt.Fprintln(t.Out)
	t.Step(ui.KindNeutral, t.Bold("Tip"))
	tip := tips[rand.IntN(len(tips))]
	if t.Animate {
		fmt.Fprint(t.Err, "  "+t.Subtle(t.Icons.Tree)+"  ")
		t.Typewriter(t.Muted(tip), 6*time.Millisecond)
	} else {
		t.Sub(t.Muted(tip))
	}
	return nil
}

// greeting is the opt-in one-liner for new shells: no sleeps, no scans.
func (a *App) greeting() {
	t := a.T
	if !t.OutTTY {
		return
	}
	fmt.Fprintln(t.Out, t.Brand(t.Icons.Star+" thine")+" "+t.Muted("ready · try ")+
		t.Code("whoport")+t.Muted(", ")+t.Code("killfb")+t.Muted(", ")+t.Code("whatsup"))
}

func (a *App) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print version, commit, build date and target",
		GroupID: "setup",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			bi := buildinfo.Get()
			if a.json() {
				return a.emitJSON(map[string]any{"version": bi.Version, "commit": bi.Commit, "date": bi.Date, "go_version": bi.GoVersion, "target": bi.Target})
			}
			t := a.T
			fmt.Fprintf(t.Out, "%s %s\n", t.Brand(t.Icons.Star+" thine"), t.Bold(bi.Version))
			t.Sub(t.Muted(fmt.Sprintf("commit %s · built %s · %s · %s", bi.Commit, bi.Date, bi.GoVersion, bi.Target)))
			return nil
		},
	}
}
