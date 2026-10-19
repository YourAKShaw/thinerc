// Package ui renders everything thine shows a human: colour, gradients, icons,
// the Claude-style star spinner, the animated splash and celebration effects.
//
// Rules this package enforces:
//   - Data goes to stdout; spinners, prompts and animation go to stderr.
//   - Colour and animation switch off for NO_COLOR, TERM=dumb, CI, --plain,
//     THINE_NO_ANIM, non-TTY streams and --json.
//   - Untrusted text (process names, paths, agent payloads) passes through
//     Sanitize before it reaches a terminal.
package ui

import (
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// ColorLevel is how many colours the terminal can show.
type ColorLevel int

const (
	ColorNone ColorLevel = iota
	Color16
	Color256
	ColorTrue
)

// Options is set once from global flags and config, before any output.
type Options struct {
	JSON       bool   // machine output: no colour, icons or animation on stdout
	Plain      bool   // no icons, colour or animation (screen-reader friendly)
	Quiet      bool   // only results and errors
	NoColor    bool   // --no-color
	Animations bool   // config ui.animations (default true)
	Icons      string // emoji | nerd | ascii
	Theme      string // palette name, see themes.go
	Splash     bool   // animated logo on bare `thine`
}

// Term describes the live output environment.
type Term struct {
	Out, Err     io.Writer
	OutTTY       bool
	ErrTTY       bool
	InTTY        bool
	Level        ColorLevel
	Animate      bool
	Opts         Options
	Pal          Palette
	Icons        IconSet
	mu           sync.Mutex
	cursorHidden bool
}

var (
	std     *Term
	stdOnce sync.Once
)

// Std returns the process-wide terminal, initialised with defaults on first use.
func Std() *Term {
	stdOnce.Do(func() { std = newTerm(Options{Animations: true, Icons: "emoji", Theme: "thine"}) })
	return std
}

// Init configures the process-wide terminal. Call it once, early.
func Init(o Options) *Term {
	stdOnce.Do(func() {})
	std = newTerm(o)
	return std
}

func newTerm(o Options) *Term {
	t := &Term{
		Out:    os.Stdout,
		Err:    os.Stderr,
		OutTTY: isTTY(os.Stdout),
		ErrTTY: isTTY(os.Stderr),
		InTTY:  isTTY(os.Stdin),
		Opts:   o,
	}
	t.Level = detectLevel(t.OutTTY || t.ErrTTY, o)
	t.Animate = o.Animations && t.ErrTTY && t.Level != ColorNone && !o.Plain && !o.JSON &&
		os.Getenv("THINE_NO_ANIM") == "" && !isCI()
	t.Pal = ThemeByName(o.Theme)
	switch {
	case o.Plain || o.JSON:
		t.Icons = asciiIcons
	case o.Icons == "nerd":
		t.Icons = nerdIcons
	case o.Icons == "ascii":
		t.Icons = asciiIcons
	default:
		t.Icons = emojiIcons
	}
	return t
}

func isTTY(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }

func isCI() bool {
	for _, k := range []string{"CI", "GITHUB_ACTIONS", "BUILDKITE", "GITLAB_CI", "TF_BUILD"} {
		if v := os.Getenv(k); v != "" && v != "false" && v != "0" {
			return true
		}
	}
	return false
}

func detectLevel(tty bool, o Options) ColorLevel {
	if o.JSON || o.Plain || o.NoColor || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return ColorNone
	}
	force := os.Getenv("FORCE_COLOR")
	if !tty && (force == "" || force == "0") {
		return ColorNone
	}
	ct := strings.ToLower(os.Getenv("COLORTERM"))
	if ct == "truecolor" || ct == "24bit" || force == "3" {
		return ColorTrue
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "iTerm.app", "WezTerm", "vscode", "ghostty", "Hyper", "Tabby", "WarpTerminal":
		return ColorTrue
	}
	termEnv := os.Getenv("TERM")
	if strings.Contains(termEnv, "kitty") || strings.Contains(termEnv, "ghostty") || strings.Contains(termEnv, "direct") {
		return ColorTrue
	}
	if os.Getenv("WT_SESSION") != "" || os.Getenv("VTE_VERSION") != "" || os.Getenv("KONSOLE_VERSION") != "" {
		return ColorTrue
	}
	if strings.Contains(termEnv, "256") || force == "2" {
		return Color256
	}
	return Color16
}

// Width is the stderr/stdout terminal width, or 80 when unknown.
func (t *Term) Width() int {
	for _, f := range []*os.File{os.Stdout, os.Stderr} {
		if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 0 {
			return w
		}
	}
	return 80
}

// Interactive reports whether we may prompt the user.
func (t *Term) Interactive() bool { return t.InTTY && t.ErrTTY && !t.Opts.JSON }

// HideCursor hides the cursor on stderr until ShowCursor (or RestoreTerminal) runs.
func (t *Term) HideCursor() {
	if !t.Animate {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.cursorHidden {
		io.WriteString(t.Err, "\x1b[?25l")
		t.cursorHidden = true
	}
}

// ShowCursor restores the cursor.
func (t *Term) ShowCursor() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cursorHidden {
		io.WriteString(t.Err, "\x1b[?25h")
		t.cursorHidden = false
	}
}

// RestoreTerminal undoes any visual state; safe to call from signal handlers.
func RestoreTerminal() {
	if std != nil {
		std.ShowCursor()
	}
}
