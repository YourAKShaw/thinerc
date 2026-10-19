package ui

import (
	"fmt"
	"io"
	"math"
	"strings"
	"time"
)

// Kind selects the semantic colour of a step marker.
type Kind int

const (
	KindNeutral Kind = iota
	KindOK
	KindFail
	KindWarn
	KindInfo
)

// Colour helpers — each returns styled text (or plain text when colour is off).

func (t *Term) Ok(s string) string     { return S().Fg(t.Pal.Success).RenderAt(t.Level, s) }
func (t *Term) Bad(s string) string    { return S().Fg(t.Pal.Error).RenderAt(t.Level, s) }
func (t *Term) Warnc(s string) string  { return S().Fg(t.Pal.Warning).RenderAt(t.Level, s) }
func (t *Term) Infoc(s string) string  { return S().Fg(t.Pal.Info).RenderAt(t.Level, s) }
func (t *Term) Accent(s string) string { return S().Fg(t.Pal.Accent).RenderAt(t.Level, s) }
func (t *Term) Text(s string) string   { return S().Fg(t.Pal.Text).RenderAt(t.Level, s) }
func (t *Term) Bold(s string) string   { return S().Bold().Fg(t.Pal.Text).RenderAt(t.Level, s) }
func (t *Term) Subtle(s string) string { return S().Fg(t.Pal.Subtle).RenderAt(t.Level, s) }

// Muted is for metadata and hints: dim and italic (a cursive italic font shines here).
func (t *Term) Muted(s string) string { return S().Italic().Fg(t.Pal.Muted).RenderAt(t.Level, s) }

// Code styles a command the user can type.
func (t *Term) Code(s string) string { return S().Bold().Fg(t.Pal.Accent).RenderAt(t.Level, s) }

// Brand paints text with the theme gradient.
func (t *Term) Brand(s string) string { return GradientAt(t.Level, s, 0, t.Pal.Brand...) }

func (t *Term) kindColor(k Kind) RGB {
	switch k {
	case KindOK:
		return t.Pal.Success
	case KindFail:
		return t.Pal.Error
	case KindWarn:
		return t.Pal.Warning
	case KindInfo:
		return t.Pal.Info
	}
	return t.Pal.Brand[0]
}

// Paint renders s in the colour for kind k.
func (t *Term) Paint(k Kind, s string) string { return S().Fg(t.kindColor(k)).RenderAt(t.Level, s) }

// Step prints a Claude-Code-style lead line: "⏺ title".
func (t *Term) Step(k Kind, title string) { t.StepTo(t.Out, k, title) }

// StepTo prints a step line to w.
func (t *Term) StepTo(w io.Writer, k Kind, title string) {
	fmt.Fprintf(w, "%s %s\n", S().Fg(t.kindColor(k)).RenderAt(t.Level, t.Icons.Step), title)
}

// Sub prints a result line hanging under the previous step: "  ⎿  text".
func (t *Term) Sub(text string) { t.SubTo(t.Out, text) }

// SubTo prints a result line to w.
func (t *Term) SubTo(w io.Writer, text string) {
	fmt.Fprintf(w, "  %s  %s\n", t.Subtle(t.Icons.Tree), text)
}

// Indent prints continuation lines aligned under a Sub line.
func (t *Term) Indent(text string) { fmt.Fprintf(t.Out, "     %s\n", text) }

// Line prints one line to stdout.
func (t *Term) Line(format string, a ...any) { fmt.Fprintf(t.Out, format+"\n", a...) }

// Note prints a hint to stderr (suppressed by --quiet).
func (t *Term) Note(format string, a ...any) {
	if t.Opts.Quiet {
		return
	}
	fmt.Fprintf(t.Err, "%s\n", t.Muted(fmt.Sprintf(format, a...)))
}

// Fail prints "✖ what — why → fix" to stderr (plan §10.3).
func (t *Term) Fail(what, why, fix string) {
	msg := t.Bad(t.Icons.Fail + " " + what)
	if why != "" {
		msg += t.Subtle(" — ") + why
	}
	if fix != "" {
		msg += t.Subtle(" "+t.Icons.Arrow+" ") + t.Accent(fix)
	}
	fmt.Fprintln(t.Err, msg)
}

// Warn prints "▲ message" to stderr.
func (t *Term) Warn(format string, a ...any) {
	fmt.Fprintln(t.Err, t.Warnc(t.Icons.Warn+" ")+fmt.Sprintf(format, a...))
}

// Badge renders a small coloured pill: " regenerable ".
func (t *Term) Badge(text string, c RGB) string {
	if t.Level == ColorNone {
		return "[" + text + "]"
	}
	return S().Bold().Fg(c).RenderAt(t.Level, "▕") +
		S().Bold().Fg(Darken(c, 0.82)).Bg(c).RenderAt(t.Level, text) +
		S().Bold().Fg(c).RenderAt(t.Level, "▏")
}

// Rule prints a labelled horizontal rule.
func (t *Term) Rule(label string) {
	w := min(t.Width(), 100) - 2
	if label == "" {
		fmt.Fprintln(t.Out, t.Subtle(strings.Repeat("─", w)))
		return
	}
	left := "── " + label + " "
	fmt.Fprintln(t.Out, t.Subtle("── ")+t.Bold(label)+" "+t.Subtle(strings.Repeat("─", max(0, w-Width(left)))))
}

// Box draws a rounded box around lines (used by the welcome screen and summaries).
func (t *Term) Box(title string, lines []string, border RGB) string {
	inner := 0
	for _, l := range lines {
		inner = max(inner, Width(l))
	}
	inner = max(inner, Width(title)+2)
	maxInner := t.Width() - 4
	if maxInner > 10 && inner > maxInner {
		inner = maxInner
	}
	bc := func(s string) string { return S().Fg(border).RenderAt(t.Level, s) }
	var b strings.Builder
	if title != "" {
		tw := Width(title)
		b.WriteString(bc("╭") + bc("─ ") + title + bc(" "+strings.Repeat("─", max(0, inner-tw-1))) + bc("╮") + "\n")
	} else {
		b.WriteString(bc("╭"+strings.Repeat("─", inner+2)+"╮") + "\n")
	}
	for _, l := range lines {
		if Width(l) > inner {
			l = Truncate(StripANSI(l), inner)
		}
		b.WriteString(bc("│") + " " + PadRight(l, inner) + " " + bc("│") + "\n")
	}
	b.WriteString(bc("╰"+strings.Repeat("─", inner+2)+"╯") + "\n")
	return b.String()
}

// Bar renders a smooth gauge of width cells filled to frac, coloured along stops.
func (t *Term) Bar(frac float64, width int, stops ...RGB) string {
	frac = math.Max(0, math.Min(1, frac))
	if t.Level == ColorNone {
		n := int(math.Round(frac * float64(width)))
		return "[" + strings.Repeat("#", n) + strings.Repeat(".", width-n) + "]"
	}
	if len(stops) == 0 {
		stops = t.Pal.Brand
	}
	eighths := []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}
	full := frac * float64(width)
	whole := int(full)
	part := int((full - float64(whole)) * 8)
	var b strings.Builder
	for i := 0; i < width; i++ {
		c := Along(stops, float64(i)/math.Max(1, float64(width-1)))
		switch {
		case i < whole:
			b.WriteString(S().Fg(c).Bg(t.Pal.Surface).RenderAt(t.Level, "█"))
		case i == whole && part > 0:
			b.WriteString(S().Fg(c).Bg(t.Pal.Surface).RenderAt(t.Level, eighths[part]))
		default:
			b.WriteString(S().Fg(t.Pal.Surface).Bg(t.Pal.Surface).RenderAt(t.Level, " "))
		}
	}
	return b.String()
}

// HumanBytes formats bytes with binary units: 1.4 GiB.
func HumanBytes(n int64) string {
	if n < 0 {
		return "-" + HumanBytes(-n)
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit && exp < 5; m /= unit {
		div *= unit
		exp++
	}
	v := float64(n) / float64(div)
	f := "%.1f %ciB"
	if v >= 100 {
		f = "%.0f %ciB"
	}
	return fmt.Sprintf(f, v, "KMGTPE"[exp])
}

// HumanDuration formats short and long spans: 0.4s, 2h14m, 3d.
func HumanDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
