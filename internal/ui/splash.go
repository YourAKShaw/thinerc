package ui

import (
	"fmt"
	"io"
	"math"
	"strings"
	"time"
)

// Letterforms in the "ANSI Shadow" style, composed at runtime so the wordmark
// is always aligned no matter which letters are used.
var glyphs = map[rune][6]string{
	't': {"████████╗", "╚══██╔══╝", "   ██║   ", "   ██║   ", "   ██║   ", "   ╚═╝   "},
	'h': {"██╗  ██╗", "██║  ██║", "███████║", "██╔══██║", "██║  ██║", "╚═╝  ╚═╝"},
	'i': {"██╗", "██║", "██║", "██║", "██║", "╚═╝"},
	'n': {"███╗   ██╗", "████╗  ██║", "██╔██╗ ██║", "██║╚██╗██║", "██║ ╚████║", "╚═╝  ╚═══╝"},
	'e': {"███████╗", "██╔════╝", "█████╗  ", "██╔══╝  ", "███████╗", "╚══════╝"},
}

// Wordmark returns the six rows of the big "THINE" logo.
func Wordmark() []string {
	rows := make([]string, 6)
	for _, r := range "thine" {
		g := glyphs[r]
		for i := range rows {
			rows[i] += g[i]
		}
	}
	return rows
}

// smallMark is used when the terminal is too narrow for the wordmark.
const smallMark = "✦ t h i n e"

// Splash plays the startup animation on stderr:
//  1. a star bursts open in the centre of the logo row,
//  2. the wordmark wipes in column by column, painted with the theme gradient,
//  3. one shimmer sweep travels across it, and
//  4. the tagline types itself out.
//
// Without animation it prints the finished frame once (or nothing when quiet).
func (t *Term) Splash(tagline string) {
	w := io.Writer(t.Err)
	rows := Wordmark()
	logoW := Width(rows[0])
	if t.Width() < logoW+4 {
		fmt.Fprintln(w, "  "+t.Brand(smallMark)+"  "+t.Muted(tagline))
		return
	}
	pad := "  "
	if !t.Animate {
		for _, r := range rows {
			fmt.Fprintln(w, pad+GradientAt(t.Level, r, 0, t.Pal.Brand...))
		}
		fmt.Fprintln(w, pad+t.Muted(tagline))
		return
	}

	t.HideCursor()
	defer t.ShowCursor()

	// Reserve the block so we can redraw it in place.
	height := len(rows) + 1
	io.WriteString(w, strings.Repeat("\n", height))
	up := fmt.Sprintf("\x1b[%dA", height)

	// 1 — star burst.
	mid := len(rows) / 2
	for i, f := range StarFrames[:7] {
		var b strings.Builder
		b.WriteString(up)
		for r := 0; r < height; r++ {
			b.WriteString("\r\x1b[2K")
			if r == mid {
				c := Along(t.Pal.Brand, float64(i)/6)
				b.WriteString(pad + strings.Repeat(" ", logoW/2) + S().Bold().Fg(c).RenderAt(t.Level, f))
			}
			b.WriteString("\n")
		}
		io.WriteString(w, b.String())
		time.Sleep(38 * time.Millisecond)
	}

	// 2 — wipe-in from the centre outwards, with a bright leading edge.
	runeRows := make([][]rune, len(rows))
	for i, r := range rows {
		runeRows[i] = []rune(r)
	}
	cols := len(runeRows[0])
	centre := cols / 2
	steps := 18
	for s := 1; s <= steps; s++ {
		reach := int(math.Ceil(float64(centre+1) * easeOutCubic(float64(s)/float64(steps))))
		var b strings.Builder
		b.WriteString(up)
		for _, rr := range runeRows {
			b.WriteString("\r\x1b[2K" + pad)
			for c, ch := range rr {
				d := int(math.Abs(float64(c - centre)))
				if d > reach || ch == ' ' {
					b.WriteRune(' ')
					continue
				}
				col := Along(t.Pal.Brand, float64(c)/float64(cols-1))
				if reach-d < 2 && s < steps {
					col = Lighten(col, 0.7)
				}
				b.WriteString(S().Fg(col).RenderAt(t.Level, string(ch)))
			}
			b.WriteString("\n")
		}
		b.WriteString("\r\x1b[2K\n")
		io.WriteString(w, b.String())
		time.Sleep(22 * time.Millisecond)
	}

	// 3 — a single diagonal shimmer sweep.
	for s := -8; s <= cols+8; s += 3 {
		var b strings.Builder
		b.WriteString(up)
		for ri, rr := range runeRows {
			b.WriteString("\r\x1b[2K" + pad)
			for c, ch := range rr {
				if ch == ' ' {
					b.WriteRune(' ')
					continue
				}
				col := Along(t.Pal.Brand, float64(c)/float64(cols-1))
				if d := math.Abs(float64(c + ri*2 - s)); d < 4 {
					col = Lighten(col, 0.8*(1-d/4))
				}
				b.WriteString(S().Fg(col).RenderAt(t.Level, string(ch)))
			}
			b.WriteString("\n")
		}
		b.WriteString("\r\x1b[2K\n")
		io.WriteString(w, b.String())
		time.Sleep(14 * time.Millisecond)
	}

	// 4 — tagline types out on the reserved last row.
	io.WriteString(w, "\x1b[1A\r\x1b[2K"+pad)
	t.Typewriter(t.Muted(tagline), 9*time.Millisecond)
}

func easeOutCubic(x float64) float64 { return 1 - math.Pow(1-x, 3) }
