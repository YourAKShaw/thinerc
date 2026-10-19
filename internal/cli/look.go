package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/YourAKShaw/thinerc/internal/config"
	"github.com/YourAKShaw/thinerc/internal/fonts"
	"github.com/YourAKShaw/thinerc/internal/netfetch"
	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

// ── fonts ──────────────────────────────────────────────────────────────────

func (a *App) fontsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "fonts",
		Short:   "Recommended terminal fonts: ligatures, Nerd Font icons, cursive italics",
		GroupID: "look",
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return a.fontsList(cmd.Context()) },
	}
	var yes bool
	install := &cobra.Command{
		Use:   "install <font>",
		Short: "Download a pinned Nerd Font release, verify its SHA-256, install it for your user",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			var ids []string
			for _, f := range fonts.Catalog {
				ids = append(ids, f.ID)
			}
			return ids, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error { return a.fontsInstall(cmd.Context(), args[0], yes) },
	}
	install.Flags().BoolVarP(&yes, "yes", "y", false, "don't ask before downloading")
	c.AddCommand(install, &cobra.Command{
		Use:   "test",
		Short: "Print a specimen to check italics, ligatures, icons and colour in your terminal",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { a.fontSpecimen(); return nil },
	})
	return c
}

func (a *App) fontsList(ctx context.Context) error {
	t := a.T
	installed := fonts.Installed(ctx)
	if a.json() {
		var rows []map[string]any
		for _, f := range fonts.Catalog {
			rows = append(rows, map[string]any{"id": f.ID, "family": f.Family, "italics": f.Italics, "ligatures": f.Ligatures,
				"installed": fonts.IsInstalled(f, installed), "url": f.URL(), "sha256": f.SHA256})
		}
		return a.emitJSON(map[string]any{"nerd_fonts_version": fonts.NerdFontsVersion, "fonts": rows})
	}
	t.Step(ui.KindNeutral, t.Icons.Font+" "+t.Bold("Terminal fonts")+t.Muted(" · Nerd Fonts "+fonts.NerdFontsVersion+" · pinned and checksum-verified"))
	for _, f := range fonts.Catalog {
		state := t.Muted("not installed")
		if fonts.IsInstalled(f, installed) {
			state = t.Ok(t.Icons.OK + " installed")
		} else if fonts.HasPlain(f, installed) {
			state = t.Warnc("plain build only") + t.Muted(" (no icons)")
		}
		ital := map[string]string{"cursive": t.Accent("cursive italics"), "true": t.Text("true italics"), "none": t.Muted("no italics")}[f.Italics]
		fmt.Fprintf(t.Out, "  %s %s %s %s\n", ui.PadRight(t.Accent(t.Bold(f.ID)), 16), ui.PadRight(t.Text(f.Family), 26), ui.PadRight(ital, 16), state)
		fmt.Fprintf(t.Out, "  %s %s\n", ui.Repeat(" ", 16), t.Muted(f.Blurb))
	}
	for _, have := range installed {
		if strings.Contains(strings.ToLower(have), "iscript") {
			fmt.Fprintln(t.Out)
			t.Step(ui.KindOK, t.Bold("You have "+have)+t.Muted(" — Fira Code with a script italic. thine's hints are italic, so they'll render in it."))
			break
		}
	}
	fmt.Fprintln(t.Out)
	t.Sub(t.Muted("install: ") + t.Code("thine fonts install victor-mono") + t.Muted(" · check yours: ") + t.Code("thine fonts test"))
	return nil
}

func (a *App) fontsInstall(ctx context.Context, id string, yes bool) error {
	t := a.T
	f, ok := fonts.ByID(id)
	if !ok {
		return fail(ExitNotFound, "E_NOT_FOUND", fmt.Sprintf("unknown font %q", id), "thine fonts")
	}
	dir := fonts.InstallDir(f)
	t.Step(ui.KindNeutral, fmt.Sprintf("%s %s %s", t.Icons.Font, t.Bold("Install "+f.Family), t.Muted("("+ui.HumanBytes(f.Size)+")")))
	t.Sub(t.Muted("from   ") + t.Text(f.URL()))
	t.Indent(t.Muted("sha256 ") + t.Text(f.SHA256))
	t.Indent(t.Muted("into   ") + t.Text(platform.Tilde(dir)) + t.Muted(" (your user only, no sudo)"))
	if !platform.Have("tar") {
		return fail(ExitNotFound, "E_NOT_FOUND", "tar is needed to unpack the font", "")
	}
	if a.isAgent() {
		return fail(ExitRefused, "E_AGENT_POLICY", "downloads need a human", "run it yourself: thine fonts install "+id)
	}
	if !yes {
		if !t.Interactive() {
			return silent(ExitNeedsApply)
		}
		if !t.Confirm("Download and install? This is thine's only network call.") {
			return nil
		}
	}
	tmp, err := os.MkdirTemp("", "thine-font-")
	if err != nil {
		return fail(ExitFailure, "E_IO", err.Error(), "")
	}
	defer os.RemoveAll(tmp)
	archive := filepath.Join(tmp, f.Asset)
	sp := t.Spin("Downloading " + f.Asset)
	err = netfetch.Download(ctx, f.URL(), archive, f.SHA256, func(done, total int64) {
		if total > 0 {
			sp.Detail(fmt.Sprintf("%s / %s", ui.HumanBytes(done), ui.HumanBytes(total)))
		}
	})
	sp.Stop()
	if err != nil {
		return fail(ExitFailure, "E_DOWNLOAD", "download failed — "+err.Error(), "check your connection and retry")
	}
	t.Step(ui.KindOK, t.Ok("checksum verified ")+t.Muted(f.SHA256[:16]+"…"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fail(ExitFailure, "E_IO", err.Error(), "")
	}
	sp = t.Spin("Unpacking")
	_, err = platform.Run(ctx, 2*time.Minute, "tar", "-xJf", archive, "-C", dir, "--wildcards", "*.ttf")
	if err != nil {
		_, err = platform.Run(ctx, 2*time.Minute, "tar", "-xJf", archive, "-C", dir) // bsdtar has no --wildcards
	}
	if err == nil && platform.Have("fc-cache") {
		sp.Update("Refreshing the font cache")
		platform.Run(ctx, 2*time.Minute, "fc-cache", "-f", dir)
	}
	sp.Stop()
	if err != nil {
		return fail(ExitFailure, "E_EXTERNAL_TOOL", "unpacking failed — "+err.Error(), "")
	}
	t.Celebrate(t.Icons.Spark + " " + f.Family + " installed")
	h := fonts.HintFor(f.Family)
	t.Step(ui.KindInfo, t.Bold("Now tell "+h.Terminal+" to use it"))
	for i, s := range h.Steps {
		if i == 0 {
			t.Sub(t.Text(s))
		} else {
			t.Indent(t.Code(s))
		}
	}
	t.Indent(t.Muted("then check with: ") + t.Code("thine fonts test"))
	return nil
}

// fontSpecimen shows everything a good terminal font + theme should render.
func (a *App) fontSpecimen() {
	t := a.T
	lvl := map[ui.ColorLevel]string{ui.ColorNone: "none", ui.Color16: "16 colours", ui.Color256: "256 colours", ui.ColorTrue: "truecolor (24-bit)"}[t.Level]
	t.Step(ui.KindNeutral, t.Bold("Font & colour check")+t.Muted(" · colour support: "+lvl))
	row := func(label, sample string) {
		fmt.Fprintf(t.Out, "  %s %s\n", ui.PadRight(t.Muted(label), 14), sample)
	}
	row("regular", t.Text("The quick brown fox jumps over the lazy dog 0O 1lI"))
	row("bold", t.Bold("The quick brown fox jumps over the lazy dog"))
	row("italic", ui.S().Italic().Fg(t.Pal.Text).Render("The quick brown fox jumps over the lazy dog")+t.Muted("  ← cursive here = 💯"))
	row("bold italic", ui.S().Bold().Italic().Fg(t.Pal.Accent).Render("The quick brown fox jumps over the lazy dog"))
	row("ligatures", t.Accent("=> -> <= >= != === !== && || ::  <!-- --> |> <|> ~~>"))
	row("nerd icons", t.Text("\uf00c \uf00d \uf013 \ue725 \uf308 \U000f035b \U000f00e2 \uf135")+t.Muted("  ← boxes mean no Nerd Font"))
	row("emoji", "🧹 🧠 ⚡ 🐳 🤖 🎉 ✨")
	row("box drawing", t.Subtle("╭──┬──╮ │ ⎿ ⏺ ▏▎▍▌▋▊▉█ ╰──┴──╯"))
	row("gradient", t.Brand(strings.Repeat("█", 40)))
	row("palette", a.swatch(t.Pal))
}

// ── themes ─────────────────────────────────────────────────────────────────

func (a *App) swatch(p ui.Palette) string {
	t := a.T
	var b strings.Builder
	for _, c := range append(append([]ui.RGB{}, p.Brand...), p.Accent, p.Success, p.Warning, p.Error, p.Info, p.Muted) {
		b.WriteString(ui.S().Fg(c).RenderAt(t.Level, "██"))
	}
	return b.String()
}

func (a *App) themeCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "theme",
		Short:   "Preview and switch thine's colour themes; theme your whole terminal",
		GroupID: "look",
	}
	c.AddCommand(&cobra.Command{
		Use:   "ls",
		Short: "List themes with live swatches",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			t := a.T
			if a.json() {
				var rows []map[string]string
				for _, n := range ui.ThemeNames() {
					p := ui.ThemeByName(n)
					rows = append(rows, map[string]string{"name": n, "title": p.Title, "upstream": p.Upstream})
				}
				return a.emitJSON(map[string]any{"themes": rows, "active": t.Pal.Name})
			}
			t.Step(ui.KindNeutral, t.Icons.Theme+" "+t.Bold("Themes")+t.Muted(" · switch with `thine theme set <name>` · try one with --theme <name>"))
			for _, n := range ui.ThemeNames() {
				p := ui.ThemeByName(n)
				mark := "  "
				if n == t.Pal.Name {
					mark = t.Ok("● ")
				}
				word := ui.GradientAt(t.Level, "✦ thine", 0, p.Brand...)
				fmt.Fprintf(t.Out, "  %s%s %s %s %s\n", mark, ui.PadRight(ui.S().Bold().Fg(p.Accent).RenderAt(t.Level, n), 18), word, a.swatch(p), t.Muted(p.Title))
			}
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "preview [name]",
		Short: "Render a sample screen in a theme (default: all, animated)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			names := ui.ThemeNames()
			if len(args) == 1 {
				if !ui.HasTheme(args[0]) {
					return fail(ExitNotFound, "E_NOT_FOUND", fmt.Sprintf("no theme %q", args[0]), "thine theme ls")
				}
				names = args
			}
			orig := a.T.Pal
			defer func() { a.T.Pal = orig }()
			for _, n := range names {
				a.T.Pal = ui.ThemeByName(n)
				a.themeSample()
				if a.T.Animate && len(names) > 1 {
					time.Sleep(350 * time.Millisecond)
				}
			}
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "set <name>",
		Short: "Make a theme the default (writes ui.theme in your config)",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return ui.ThemeNames(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(_ *cobra.Command, args []string) error {
			if !ui.HasTheme(args[0]) {
				return fail(ExitNotFound, "E_NOT_FOUND", fmt.Sprintf("no theme %q", args[0]), "thine theme ls")
			}
			if err := config.SetKey(a.Cfg.GlobalPath, "ui", "theme", strconv.Quote(args[0])); err != nil {
				return fail(ExitFailure, "E_IO", "couldn't update config — "+err.Error(), "")
			}
			a.T.Pal = ui.ThemeByName(args[0])
			a.T.Wave(a.T.Icons.Theme+" theme set to "+a.T.Pal.Title, 900*time.Millisecond)
			a.T.Sub(a.T.Muted("saved in " + platform.Tilde(a.Cfg.GlobalPath)))
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "terminal",
		Short: "How to give your whole terminal a matching colour scheme",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { a.terminalThemeGuide(); return nil },
	})
	return c
}

func (a *App) themeSample() {
	t := a.T
	p := t.Pal
	fmt.Fprintln(t.Out)
	fmt.Fprintln(t.Out, "  "+ui.GradientAt(t.Level, "✦ "+p.Title, 0, p.Brand...)+" "+t.Muted("· "+p.Upstream))
	t.Step(ui.KindOK, t.Accent(t.Bold(":3000"))+" "+t.Bold("node")+t.Muted(" (4123) ")+t.Badge("dev", p.Success))
	t.Sub(t.Text("next dev") + t.Muted("  ~/code/shop · up 2h14m"))
	t.Step(ui.KindWarn, t.Accent(t.Bold(":5432"))+" "+t.Bold("postgres")+t.Muted(" (911) ")+t.Badge("confirm", p.Warning))
	t.Step(ui.KindFail, t.Bad("couldn't stop PID 1")+t.Muted(" — protected ")+t.Badge("refused", p.Error))
	t.Step(ui.KindInfo, t.Icons.Memory+" "+t.Bold("Memory")+t.Muted(" · 3.1 GiB available"))
	t.Sub(t.Bar(0.62, 28, p.Success, p.Warning, p.Error) + t.Muted(" 62% used"))
	fmt.Fprintf(t.Out, "  %s %s %s %s\n", t.Badge("regenerable", p.Info), t.Badge("reversible", p.Success), t.Badge("destructive", p.Error), a.swatch(p))
}

func (a *App) terminalThemeGuide() {
	t := a.T
	t.Step(ui.KindNeutral, t.Icons.Theme+" "+t.Bold("Theme your whole terminal")+t.Muted(" · thine's colours adapt; your terminal's 16-colour palette sets the mood"))
	sources := [][2]string{
		{"Gogh", "github.com/Gogh-Co/Gogh — 250+ schemes with an interactive installer for GNOME Terminal, Xfce, Konsole, Tilix, kitty…"},
		{"iTerm2-Color-Schemes", "github.com/mbadolato/iTerm2-Color-Schemes — the biggest collection, exported for iTerm2, kitty, Alacritty, WezTerm, Ghostty, Windows Terminal, Xresources…"},
		{"Catppuccin", "github.com/catppuccin/catppuccin — per-terminal ports (catppuccin/iterm, /kitty, /alacritty, /gnome-terminal…) matching thine's catppuccin themes"},
		{"Dracula · Nord · Tokyo Night · Rosé Pine", "each publishes official terminal ports — match them with `thine theme set <name>`"},
	}
	for _, s := range sources {
		t.Sub(t.Accent(t.Bold(s[0])))
		t.Indent(t.Muted(s[1]))
	}
	fmt.Fprintln(t.Out)
	tp := os.Getenv("TERM_PROGRAM")
	switch {
	case os.Getenv("KITTY_WINDOW_ID") != "":
		t.Step(ui.KindInfo, t.Bold("kitty")+t.Muted(" has a built-in theme browser: ")+t.Code("kitty +kitten themes"))
	case tp == "ghostty":
		t.Step(ui.KindInfo, t.Bold("Ghostty")+t.Muted(" ships hundreds of themes: ")+t.Code("ghostty +list-themes")+t.Muted(" then ")+t.Code(`theme = "catppuccin-mocha"`))
	case tp == "WezTerm":
		t.Step(ui.KindInfo, t.Bold("WezTerm")+t.Muted(" bundles iTerm2-Color-Schemes: ")+t.Code(`config.color_scheme = "Catppuccin Mocha"`))
	case tp == "iTerm.app":
		t.Step(ui.KindInfo, t.Bold("iTerm2")+t.Muted(": download a .itermcolors file, then Settings → Profiles → Colors → Color Presets → Import"))
	case os.Getenv("VTE_VERSION") != "":
		t.Step(ui.KindInfo, t.Bold("GNOME Terminal / VTE")+t.Muted(": Gogh's installer adds schemes as new profiles; pick one under Preferences"))
	default:
		t.Step(ui.KindInfo, t.Muted("pick a scheme from a source above for your terminal, then match it: ")+t.Code("thine theme set catppuccin-mocha"))
	}
	t.Sub(t.Muted("thine's own output uses truecolor when available, so it looks right even before you change the terminal palette"))
}

// ── fx ─────────────────────────────────────────────────────────────────────

func (a *App) fxCmd() *cobra.Command {
	var seconds float64
	c := &cobra.Command{
		Use:       "fx [confetti|rain|fireworks|wave|splash|spinner|all]",
		Short:     "Play terminal animations — just for fun",
		GroupID:   "look",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"confetti", "rain", "fireworks", "wave", "splash", "spinner", "all"},
		RunE: func(_ *cobra.Command, args []string) error {
			t := a.T
			if !t.Animate {
				return fail(ExitUsage, "E_NO_TTY", "animations need an interactive, colour terminal", "unset NO_COLOR / THINE_NO_ANIM and run in a terminal")
			}
			which := "all"
			if len(args) == 1 {
				which = args[0]
			}
			d := time.Duration(seconds * float64(time.Second))
			play := map[string]func(){
				"splash":    func() { t.Splash("the safe superpower layer for your terminal") },
				"spinner":   func() { s := t.Spin("Reticulating splines"); time.Sleep(d); s.Done(ui.KindOK, "splines reticulated") },
				"confetti":  func() { t.Confetti(d, 10) },
				"rain":      func() { t.Rain(d, 12) },
				"fireworks": func() { t.Fireworks(d, 14) },
				"wave":      func() { t.Wave("✦ thine — one-word superpowers for your terminal ✦", d) },
			}
			order := []string{"splash", "spinner", "wave", "confetti", "rain", "fireworks"}
			if which != "all" {
				f, ok := play[which]
				if !ok {
					return fail(ExitUsage, "E_USAGE", fmt.Sprintf("unknown effect %q", which), "thine fx all")
				}
				f()
				return nil
			}
			for _, n := range order {
				fmt.Fprintln(t.Err, t.Muted("— "+n+" —"))
				play[n]()
			}
			t.Celebrate(t.Icons.Party + " that's the show")
			return nil
		},
	}
	c.Flags().Float64Var(&seconds, "seconds", 2.5, "how long each effect runs")
	return c
}
