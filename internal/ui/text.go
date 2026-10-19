package ui

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)|\x1b[@-Z\\-_]`)

// Sanitize makes untrusted text safe to print (invariant S8): it strips ANSI
// and OSC escape sequences and C0/C1 control characters, and bounds length.
func Sanitize(s string, max int) string {
	s = ansiRE.ReplaceAllString(s, "")
	var b strings.Builder
	n := 0
	for _, r := range s {
		if r == '\t' {
			r = ' '
		}
		if unicode.IsControl(r) || (r >= 0x80 && r <= 0x9f) || r == '\u2028' || r == '\u2029' {
			continue
		}
		if max > 0 && n >= max {
			b.WriteRune('…')
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// StripANSI removes escape sequences (used to measure rendered width).
func StripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

// Width is the display width of s, ignoring escape sequences.
func Width(s string) int { return runewidth.StringWidth(StripANSI(s)) }

// Truncate shortens plain text to w display columns with an ellipsis.
func Truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= w {
		return s
	}
	return runewidth.Truncate(s, w, "…")
}

// MiddleTruncate keeps both ends of a path: ~/code/…/shop.
func MiddleTruncate(s string, w int) string {
	if runewidth.StringWidth(s) <= w || w < 5 {
		return Truncate(s, w)
	}
	runes := []rune(s)
	head := (w - 1) / 2
	tail := w - 1 - head
	for runewidth.StringWidth(string(runes[:head]))+runewidth.StringWidth(string(runes[len(runes)-tail:])) > w-1 && head > 1 {
		head--
	}
	return string(runes[:head]) + "…" + string(runes[len(runes)-tail:])
}

// PadRight pads styled text to w display columns.
func PadRight(s string, w int) string {
	if d := w - Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// PadLeft right-aligns styled text in w display columns.
func PadLeft(s string, w int) string {
	if d := w - Width(s); d > 0 {
		return strings.Repeat(" ", d) + s
	}
	return s
}
