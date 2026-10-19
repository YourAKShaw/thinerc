package ui

import "sort"

// Palette is the set of semantic colours every screen draws from. Themes only
// change hues; meaning (green = done, red = refused…) never changes.
type Palette struct {
	Name     string
	Title    string
	Brand    []RGB // gradient stops for the logo, spinner and celebrations
	Accent   RGB   // links, highlights, prompts
	Success  RGB
	Warning  RGB
	Error    RGB
	Info     RGB
	Text     RGB
	Muted    RGB    // metadata and hints (rendered dim + italic)
	Subtle   RGB    // borders and rules
	Surface  RGB    // gauge track / backgrounds
	Upstream string // where the palette comes from, for `thine theme ls`
}

var themes = map[string]Palette{
	"thine": {
		Name: "thine", Title: "Thine Aurora",
		Brand:  []RGB{Hex("#8B5CF6"), Hex("#EC4899"), Hex("#F97316"), Hex("#FACC15")},
		Accent: Hex("#A78BFA"), Success: Hex("#34D399"), Warning: Hex("#FBBF24"),
		Error: Hex("#F87171"), Info: Hex("#38BDF8"), Text: Hex("#E5E7EB"),
		Muted: Hex("#9CA3AF"), Subtle: Hex("#6B7280"), Surface: Hex("#374151"),
		Upstream: "original",
	},
	"catppuccin-mocha": {
		Name: "catppuccin-mocha", Title: "Catppuccin Mocha",
		Brand:  []RGB{Hex("#CBA6F7"), Hex("#F5C2E7"), Hex("#FAB387"), Hex("#F9E2AF")},
		Accent: Hex("#89B4FA"), Success: Hex("#A6E3A1"), Warning: Hex("#F9E2AF"),
		Error: Hex("#F38BA8"), Info: Hex("#89DCEB"), Text: Hex("#CDD6F4"),
		Muted: Hex("#A6ADC8"), Subtle: Hex("#6C7086"), Surface: Hex("#313244"),
		Upstream: "github.com/catppuccin/catppuccin",
	},
	"catppuccin-latte": {
		Name: "catppuccin-latte", Title: "Catppuccin Latte (light)",
		Brand:  []RGB{Hex("#8839EF"), Hex("#EA76CB"), Hex("#FE640B"), Hex("#DF8E1D")},
		Accent: Hex("#1E66F5"), Success: Hex("#40A02B"), Warning: Hex("#DF8E1D"),
		Error: Hex("#D20F39"), Info: Hex("#04A5E5"), Text: Hex("#4C4F69"),
		Muted: Hex("#6C6F85"), Subtle: Hex("#9CA0B0"), Surface: Hex("#CCD0DA"),
		Upstream: "github.com/catppuccin/catppuccin",
	},
	"tokyo-night": {
		Name: "tokyo-night", Title: "Tokyo Night",
		Brand:  []RGB{Hex("#7AA2F7"), Hex("#BB9AF7"), Hex("#F7768E"), Hex("#FF9E64")},
		Accent: Hex("#7DCFFF"), Success: Hex("#9ECE6A"), Warning: Hex("#E0AF68"),
		Error: Hex("#F7768E"), Info: Hex("#7DCFFF"), Text: Hex("#C0CAF5"),
		Muted: Hex("#9AA5CE"), Subtle: Hex("#565F89"), Surface: Hex("#292E42"),
		Upstream: "github.com/folke/tokyonight.nvim",
	},
	"dracula": {
		Name: "dracula", Title: "Dracula",
		Brand:  []RGB{Hex("#BD93F9"), Hex("#FF79C6"), Hex("#FFB86C"), Hex("#F1FA8C")},
		Accent: Hex("#8BE9FD"), Success: Hex("#50FA7B"), Warning: Hex("#F1FA8C"),
		Error: Hex("#FF5555"), Info: Hex("#8BE9FD"), Text: Hex("#F8F8F2"),
		Muted: Hex("#BFBFBF"), Subtle: Hex("#6272A4"), Surface: Hex("#44475A"),
		Upstream: "github.com/dracula/dracula-theme",
	},
	"nord": {
		Name: "nord", Title: "Nord",
		Brand:  []RGB{Hex("#5E81AC"), Hex("#81A1C1"), Hex("#88C0D0"), Hex("#8FBCBB")},
		Accent: Hex("#88C0D0"), Success: Hex("#A3BE8C"), Warning: Hex("#EBCB8B"),
		Error: Hex("#BF616A"), Info: Hex("#81A1C1"), Text: Hex("#ECEFF4"),
		Muted: Hex("#D8DEE9"), Subtle: Hex("#4C566A"), Surface: Hex("#3B4252"),
		Upstream: "github.com/nordtheme/nord",
	},
	"gruvbox": {
		Name: "gruvbox", Title: "Gruvbox Dark",
		Brand:  []RGB{Hex("#D3869B"), Hex("#FB4934"), Hex("#FE8019"), Hex("#FABD2F")},
		Accent: Hex("#83A598"), Success: Hex("#B8BB26"), Warning: Hex("#FABD2F"),
		Error: Hex("#FB4934"), Info: Hex("#8EC07C"), Text: Hex("#EBDBB2"),
		Muted: Hex("#BDAE93"), Subtle: Hex("#665C54"), Surface: Hex("#3C3836"),
		Upstream: "github.com/morhetz/gruvbox",
	},
	"rose-pine": {
		Name: "rose-pine", Title: "Rosé Pine",
		Brand:  []RGB{Hex("#C4A7E7"), Hex("#EBBCBA"), Hex("#EB6F92"), Hex("#F6C177")},
		Accent: Hex("#9CCFD8"), Success: Hex("#31748F"), Warning: Hex("#F6C177"),
		Error: Hex("#EB6F92"), Info: Hex("#9CCFD8"), Text: Hex("#E0DEF4"),
		Muted: Hex("#908CAA"), Subtle: Hex("#6E6A86"), Surface: Hex("#26233A"),
		Upstream: "github.com/rose-pine/rose-pine-theme",
	},
	"synthwave": {
		Name: "synthwave", Title: "Synthwave '84",
		Brand:  []RGB{Hex("#FF7EDB"), Hex("#F97E72"), Hex("#FEDE5D"), Hex("#36F9F6")},
		Accent: Hex("#36F9F6"), Success: Hex("#72F1B8"), Warning: Hex("#FEDE5D"),
		Error: Hex("#FE4450"), Info: Hex("#36F9F6"), Text: Hex("#F0EFF1"),
		Muted: Hex("#B6B1C7"), Subtle: Hex("#495495"), Surface: Hex("#34294F"),
		Upstream: "github.com/robb0wen/synthwave-vscode",
	},
}

// ThemeByName returns a palette, falling back to the default "thine" theme.
func ThemeByName(name string) Palette {
	if p, ok := themes[name]; ok {
		return p
	}
	return themes["thine"]
}

// HasTheme reports whether a palette exists.
func HasTheme(name string) bool { _, ok := themes[name]; return ok }

// ThemeNames lists palettes, default first, the rest alphabetically.
func ThemeNames() []string {
	names := make([]string, 0, len(themes))
	for n := range themes {
		if n != "thine" {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return append([]string{"thine"}, names...)
}
