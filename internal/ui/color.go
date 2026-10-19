package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// RGB is a 24-bit colour.
type RGB struct{ R, G, B uint8 }

// Hex parses "#rrggbb"; invalid input yields mid-grey so a typo never panics.
func Hex(s string) RGB {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return RGB{128, 128, 128}
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return RGB{128, 128, 128}
	}
	return RGB{uint8(v >> 16), uint8(v >> 8), uint8(v)}
}

// Lerp blends a→b by t ∈ [0,1].
func Lerp(a, b RGB, t float64) RGB {
	t = math.Max(0, math.Min(1, t))
	mix := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*t)) }
	return RGB{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B)}
}

// Lighten moves a colour toward white by t.
func Lighten(c RGB, t float64) RGB { return Lerp(c, RGB{255, 255, 255}, t) }

// Darken moves a colour toward black by t.
func Darken(c RGB, t float64) RGB { return Lerp(c, RGB{0, 0, 0}, t) }

// Along samples a multi-stop gradient at t ∈ [0,1].
func Along(stops []RGB, t float64) RGB {
	switch len(stops) {
	case 0:
		return RGB{255, 255, 255}
	case 1:
		return stops[0]
	}
	t = math.Max(0, math.Min(1, t))
	seg := t * float64(len(stops)-1)
	i := int(seg)
	if i >= len(stops)-1 {
		return stops[len(stops)-1]
	}
	return Lerp(stops[i], stops[i+1], seg-float64(i))
}

// fgCode returns the SGR parameters for a foreground colour at a colour level.
func fgCode(c RGB, lvl ColorLevel) string {
	switch lvl {
	case ColorTrue:
		return fmt.Sprintf("38;2;%d;%d;%d", c.R, c.G, c.B)
	case Color256:
		return fmt.Sprintf("38;5;%d", to256(c))
	case Color16:
		return strconv.Itoa(to16(c))
	}
	return ""
}

func bgCode(c RGB, lvl ColorLevel) string {
	switch lvl {
	case ColorTrue:
		return fmt.Sprintf("48;2;%d;%d;%d", c.R, c.G, c.B)
	case Color256:
		return fmt.Sprintf("48;5;%d", to256(c))
	case Color16:
		return strconv.Itoa(to16(c) + 10)
	}
	return ""
}

// to256 maps to the xterm 6×6×6 cube or the grey ramp, whichever is closer.
func to256(c RGB) int {
	q := func(v uint8) int {
		if v < 48 {
			return 0
		}
		if v < 115 {
			return 1
		}
		return int((int(v) - 35) / 40)
	}
	r, g, b := q(c.R), q(c.G), q(c.B)
	levels := []int{0, 95, 135, 175, 215, 255}
	cube := 16 + 36*r + 6*g + b
	cr, cg, cb := levels[r], levels[g], levels[b]
	avg := (int(c.R) + int(c.G) + int(c.B)) / 3
	grey := 232
	if avg > 238 {
		grey = 255
	} else if avg > 8 {
		grey = 232 + (avg-8)/10
	}
	gv := 8 + (grey-232)*10
	dist := func(r2, g2, b2 int) int {
		dr, dg, db := int(c.R)-r2, int(c.G)-g2, int(c.B)-b2
		return dr*dr + dg*dg + db*db
	}
	if dist(gv, gv, gv) < dist(cr, cg, cb) {
		return grey
	}
	return cube
}

// to16 maps to the nearest basic ANSI colour so the user's theme picks the hue.
func to16(c RGB) int {
	type named struct {
		code int
		rgb  RGB
	}
	basics := []named{
		{30, RGB{0, 0, 0}}, {31, RGB{205, 49, 49}}, {32, RGB{13, 188, 121}}, {33, RGB{229, 229, 16}},
		{34, RGB{36, 114, 200}}, {35, RGB{188, 63, 188}}, {36, RGB{17, 168, 205}}, {37, RGB{229, 229, 229}},
		{90, RGB{102, 102, 102}}, {91, RGB{241, 76, 76}}, {92, RGB{35, 209, 139}}, {93, RGB{245, 245, 67}},
		{94, RGB{59, 142, 234}}, {95, RGB{214, 112, 214}}, {96, RGB{41, 184, 219}}, {97, RGB{255, 255, 255}},
	}
	best, bestD := 37, math.MaxInt
	for _, n := range basics {
		dr, dg, db := int(c.R)-int(n.rgb.R), int(c.G)-int(n.rgb.G), int(c.B)-int(n.rgb.B)
		if d := dr*dr + dg*dg + db*db; d < bestD {
			best, bestD = n.code, d
		}
	}
	return best
}

// Style is an immutable text style. The zero value renders text unchanged.
type Style struct {
	fg, bg                              *RGB
	bold, dim, italic, underline, blink bool
}

// S starts a new style.
func S() Style { return Style{} }

func (s Style) Fg(c RGB) Style       { s.fg = &c; return s }
func (s Style) Bg(c RGB) Style       { s.bg = &c; return s }
func (s Style) Bold() Style          { s.bold = true; return s }
func (s Style) Dim() Style           { s.dim = true; return s }
func (s Style) Italic() Style        { s.italic = true; return s }
func (s Style) Underline() Style     { s.underline = true; return s }
func (s Style) FgHex(h string) Style { return s.Fg(Hex(h)) }

// Render applies the style for the standard terminal.
func (s Style) Render(text string) string { return s.RenderAt(Std().Level, text) }

// RenderAt applies the style at an explicit colour level (used by tests).
func (s Style) RenderAt(lvl ColorLevel, text string) string {
	if lvl == ColorNone || text == "" {
		return text
	}
	var p []string
	if s.bold {
		p = append(p, "1")
	}
	if s.dim {
		p = append(p, "2")
	}
	if s.italic {
		p = append(p, "3")
	}
	if s.underline {
		p = append(p, "4")
	}
	if s.fg != nil {
		p = append(p, fgCode(*s.fg, lvl))
	}
	if s.bg != nil {
		p = append(p, bgCode(*s.bg, lvl))
	}
	if len(p) == 0 {
		return text
	}
	return "\x1b[" + strings.Join(p, ";") + "m" + text + "\x1b[0m"
}

// Gradient colours each visible rune of text along the given stops.
func Gradient(text string, stops ...RGB) string {
	return GradientAt(Std().Level, text, 0, stops...)
}

// GradientAt colours text along stops, shifted by phase ∈ [0,1) for animation.
// Spaces are left uncoloured so the escape overhead stays small.
func GradientAt(lvl ColorLevel, text string, phase float64, stops ...RGB) string {
	if lvl == ColorNone || len(stops) == 0 {
		return text
	}
	runes := []rune(text)
	n := len(runes)
	var b strings.Builder
	for i, r := range runes {
		if r == ' ' {
			b.WriteRune(r)
			continue
		}
		t := 0.0
		if n > 1 {
			t = float64(i) / float64(n-1)
		}
		t = math.Mod(t+phase, 1)
		// Fold so a shifted gradient wraps smoothly instead of jumping.
		t = 1 - math.Abs(1-2*t)
		b.WriteString("\x1b[" + fgCode(Along(stops, t), lvl) + "m")
		b.WriteRune(r)
	}
	b.WriteString("\x1b[0m")
	return b.String()
}

// Shimmer renders text in base colour with a bright highlight centred at pos
// (a rune index); this is the "thinking" sweep used by the spinner.
func Shimmer(lvl ColorLevel, text string, pos float64, base, hi RGB) string {
	if lvl == ColorNone {
		return text
	}
	var b strings.Builder
	for i, r := range []rune(text) {
		d := math.Abs(float64(i) - pos)
		t := math.Max(0, 1-d/3.0)
		b.WriteString("\x1b[" + fgCode(Lerp(base, hi, t), lvl) + "m")
		b.WriteRune(r)
	}
	b.WriteString("\x1b[0m")
	return b.String()
}
