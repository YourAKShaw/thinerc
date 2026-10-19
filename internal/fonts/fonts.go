// Package fonts knows the terminal fonts thine recommends: pinned Nerd Fonts
// releases (ligatures + icon glyphs) and which ones have true or cursive
// italics, plus how to tell each terminal emulator to use them.
package fonts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YourAKShaw/thinerc/internal/platform"
)

// NerdFontsVersion is the pinned upstream release (github.com/ryanoasis/nerd-fonts).
const NerdFontsVersion = "v3.5.1"

// Font is one installable font family.
type Font struct {
	ID        string
	Family    string // the name to put in terminal settings
	Asset     string // release asset file name
	SHA256    string // pinned digest of the asset (from the GitHub release API)
	Size      int64
	Ligatures bool
	Italics   string // none | true | cursive
	Blurb     string
	Detect    []string // family names that count as "installed"
}

// URL is the pinned download location.
func (f Font) URL() string {
	return "https://github.com/ryanoasis/nerd-fonts/releases/download/" + NerdFontsVersion + "/" + f.Asset
}

// Catalog lists the recommended fonts, best "italics + ligatures" first.
var Catalog = []Font{
	{ID: "victor-mono", Family: "VictorMono Nerd Font", Asset: "VictorMono.tar.xz",
		SHA256: "c6c52657921835862314b7edbbccf5695146c1e6c5d3507c3deab88e555215ed", Size: 3715280,
		Ligatures: true, Italics: "cursive", Blurb: "slender, with a true cursive italic — hints and comments look handwritten",
		Detect: []string{"VictorMono Nerd Font", "Victor Mono"}},
	{ID: "fira-code", Family: "FiraCode Nerd Font", Asset: "FiraCode.tar.xz",
		SHA256: "68e3bd6164864b8b514605bc34e3a87ac401c8c48682fcce6478c70263340207", Size: 3061404,
		Ligatures: true, Italics: "none", Blurb: "the classic ligature font (=> !== <=); no italic — pair with Fira Code iScript for cursive",
		Detect: []string{"FiraCode Nerd Font", "Fira Code"}},
	{ID: "cascadia-code", Family: "CaskaydiaCove Nerd Font", Asset: "CascadiaCode.tar.xz",
		SHA256: "ae598e9401e2846aa3ee364513715de0490b844a4b54f0768991f45f23aa8369", Size: 3592604,
		Ligatures: true, Italics: "cursive", Blurb: "Microsoft's Cascadia Code with a cursive italic",
		Detect: []string{"CaskaydiaCove Nerd Font", "Cascadia Code"}},
	{ID: "jetbrains-mono", Family: "JetBrainsMono Nerd Font", Asset: "JetBrainsMono.tar.xz",
		SHA256: "04d5e8f903693f9dd13e16f867e994834e681eb3c72c0d337a770dcda09010cf", Size: 7284244,
		Ligatures: true, Italics: "true", Blurb: "tall x-height, crisp at small sizes, real italics",
		Detect: []string{"JetBrainsMono Nerd Font", "JetBrains Mono"}},
	{ID: "monaspace", Family: "MonaspiceRn Nerd Font", Asset: "Monaspace.tar.xz",
		SHA256: "7cdb44b7cd3855a231641b3cc4cf38f434d615d12e1c475eb86232868c667dc0", Size: 19257096,
		Ligatures: true, Italics: "cursive", Blurb: "GitHub's superfamily; Radon (Rn) is the handwritten one",
		Detect: []string{"MonaspiceRn Nerd Font", "Monaspace Radon"}},
}

// ByID finds a font.
func ByID(id string) (Font, bool) {
	for _, f := range Catalog {
		if f.ID == id {
			return f, true
		}
	}
	return Font{}, false
}

// InstallDir is where thine puts fonts: user-level, no sudo.
func InstallDir(f Font) string {
	if platform.IsDarwin() {
		return filepath.Join(platform.Home(), "Library", "Fonts")
	}
	return filepath.Join(platform.Home(), ".local", "share", "fonts", "thine", f.ID)
}

// Installed lists the font family names the system knows (fc-list on Linux,
// the user and system font folders on macOS).
func Installed(ctx context.Context) []string {
	if platform.Have("fc-list") {
		out, err := platform.Output(ctx, 5*time.Second, "fc-list", ":", "family")
		if err == nil {
			seen := map[string]bool{}
			var fams []string
			for _, line := range strings.Split(out, "\n") {
				for _, f := range strings.Split(line, ",") {
					f = strings.TrimSpace(f)
					if f != "" && !seen[f] {
						seen[f] = true
						fams = append(fams, f)
					}
				}
			}
			return fams
		}
	}
	var fams []string
	for _, dir := range []string{filepath.Join(platform.Home(), "Library", "Fonts"), "/Library/Fonts"} {
		ents, _ := os.ReadDir(dir)
		for _, e := range ents {
			fams = append(fams, strings.TrimSuffix(strings.TrimSuffix(e.Name(), ".ttf"), ".otf"))
		}
	}
	return fams
}

func norm(s string) string { return strings.ToLower(strings.ReplaceAll(s, " ", "")) }

// IsInstalled reports whether the Nerd Font build (with icon glyphs) is present.
func IsInstalled(f Font, installed []string) bool {
	for _, have := range installed {
		if strings.HasPrefix(norm(have), norm(f.Detect[0])) {
			return true
		}
	}
	return false
}

// HasPlain reports whether the upstream font is present without Nerd glyphs.
func HasPlain(f Font, installed []string) bool {
	for _, have := range installed {
		for _, want := range f.Detect[1:] {
			if strings.HasPrefix(norm(have), norm(want)) {
				return true
			}
		}
	}
	return false
}

// TerminalHint explains how to switch the current terminal emulator's font.
type TerminalHint struct {
	Terminal string
	Steps    []string
}

// HintFor detects the terminal emulator and returns how to set its font.
func HintFor(family string) TerminalHint {
	tp := os.Getenv("TERM_PROGRAM")
	term := os.Getenv("TERM")
	switch {
	case os.Getenv("KITTY_WINDOW_ID") != "" || strings.Contains(term, "kitty"):
		return TerminalHint{"kitty", []string{"add to ~/.config/kitty/kitty.conf:", "font_family " + family, "italic_font auto", "then reload: ctrl+shift+F5"}}
	case tp == "ghostty" || strings.Contains(term, "ghostty"):
		return TerminalHint{"Ghostty", []string{"add to ~/.config/ghostty/config:", `font-family = "` + family + `"`, "then reload: ctrl+shift+, (cmd+shift+, on macOS)"}}
	case tp == "WezTerm":
		return TerminalHint{"WezTerm", []string{"in ~/.wezterm.lua:", `config.font = wezterm.font("` + family + `")`}}
	case os.Getenv("ALACRITTY_SOCKET") != "" || os.Getenv("ALACRITTY_WINDOW_ID") != "":
		return TerminalHint{"Alacritty", []string{"in ~/.config/alacritty/alacritty.toml:", "[font.normal]", `family = "` + family + `"`}}
	case tp == "iTerm.app":
		return TerminalHint{"iTerm2", []string{"Settings → Profiles → Text → Font → " + family, "tick “Use ligatures”, and keep “Italic text” enabled"}}
	case tp == "Apple_Terminal":
		return TerminalHint{"Terminal.app", []string{"Settings → Profiles → Text → Font → Change… → " + family}}
	case tp == "vscode":
		return TerminalHint{"VS Code terminal", []string{`settings.json: "terminal.integrated.fontFamily": "` + family + `"`, `"terminal.integrated.fontLigatures.enabled": true`}}
	case os.Getenv("KONSOLE_VERSION") != "":
		return TerminalHint{"Konsole", []string{"Settings → Edit Current Profile → Appearance → Font → " + family}}
	case os.Getenv("VTE_VERSION") != "":
		if platform.Have("gsettings") {
			return TerminalHint{"GNOME Terminal / VTE", []string{
				"Preferences → your profile → Custom font → " + family + " 12",
				"or from a shell:",
				`p=$(gsettings get org.gnome.Terminal.ProfilesList default | tr -d "'")`,
				`gsettings set org.gnome.Terminal.Legacy.Profile:/org/gnome/terminal/legacy/profiles:/:$p/ use-system-font false`,
				`gsettings set org.gnome.Terminal.Legacy.Profile:/org/gnome/terminal/legacy/profiles:/:$p/ font "` + family + ` 12"`,
			}}
		}
		return TerminalHint{"VTE terminal", []string{"Preferences → Profile → Font → " + family}}
	}
	return TerminalHint{"your terminal", []string{"open its preferences and set the font to: " + family}}
}
