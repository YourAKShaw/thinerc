package ui

import (
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"strings"
	"time"
)

// canvas is a small off-screen buffer redrawn in place on stderr.
type canvas struct {
	t      *Term
	w, h   int
	cells  [][]rune
	colors [][]RGB
	drawn  bool
}

func (t *Term) newCanvas(w, h int) *canvas {
	c := &canvas{t: t, w: w, h: h}
	c.clear()
	return c
}

func (c *canvas) clear() {
	c.cells = make([][]rune, c.h)
	c.colors = make([][]RGB, c.h)
	for y := range c.cells {
		c.cells[y] = []rune(strings.Repeat(" ", c.w))
		c.colors[y] = make([]RGB, c.w)
	}
}

func (c *canvas) set(x, y int, r rune, col RGB) {
	if x >= 0 && y >= 0 && x < c.w && y < c.h {
		c.cells[y][x] = r
		c.colors[y][x] = col
	}
}

func (c *canvas) flush() {
	var b strings.Builder
	if c.drawn {
		fmt.Fprintf(&b, "\x1b[%dA", c.h)
	}
	for y := 0; y < c.h; y++ {
		b.WriteString("\r\x1b[2K")
		for x := 0; x < c.w; x++ {
			r := c.cells[y][x]
			if r == ' ' {
				b.WriteByte(' ')
				continue
			}
			b.WriteString(S().Fg(c.colors[y][x]).RenderAt(c.t.Level, string(r)))
		}
		b.WriteByte('\n')
	}
	io.WriteString(c.t.Err, b.String())
	c.drawn = true
}

// erase wipes the canvas area and leaves the cursor at its top.
func (c *canvas) erase() {
	if !c.drawn {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\x1b[%dA", c.h)
	for y := 0; y < c.h; y++ {
		b.WriteString("\r\x1b[2K\n")
	}
	fmt.Fprintf(&b, "\x1b[%dA", c.h)
	io.WriteString(c.t.Err, b.String())
}

func (t *Term) fxSize(h int) (int, int) { return min(t.Width()-1, 96), h }

func (t *Term) palette() []RGB {
	p := append([]RGB{}, t.Pal.Brand...)
	return append(p, t.Pal.Success, t.Pal.Info, t.Pal.Accent, t.Pal.Warning)
}

// Confetti rains coloured particles for d (stderr only; no-op without animation).
func (t *Term) Confetti(d time.Duration, height int) {
	if !t.Animate {
		return
	}
	t.HideCursor()
	defer t.ShowCursor()
	w, h := t.fxSize(height)
	c := t.newCanvas(w, h)
	type p struct {
		x, y, vx, vy float64
		r            rune
		col          RGB
	}
	shapes := []rune("✦✧•◆▴▾●★✶∗")
	cols := t.palette()
	var ps []p
	for i := 0; i < w*2/3; i++ {
		ps = append(ps, p{
			x: rand.Float64() * float64(w), y: -rand.Float64() * float64(h),
			vx: (rand.Float64() - 0.5) * 0.8, vy: 0.3 + rand.Float64()*0.5,
			r: shapes[rand.IntN(len(shapes))], col: cols[rand.IntN(len(cols))],
		})
	}
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		c.clear()
		for i := range ps {
			ps[i].x += ps[i].vx + math.Sin(ps[i].y/2)*0.3
			ps[i].y += ps[i].vy
			ps[i].vy += 0.04
			if ps[i].y > float64(h) {
				ps[i].y = -1
				ps[i].vy = 0.3 + rand.Float64()*0.4
			}
			c.set(int(ps[i].x), int(ps[i].y), ps[i].r, ps[i].col)
		}
		c.flush()
		time.Sleep(50 * time.Millisecond)
	}
	c.erase()
}

// Rain is a "digital rain" in the theme colours.
func (t *Term) Rain(d time.Duration, height int) {
	if !t.Animate {
		return
	}
	t.HideCursor()
	defer t.ShowCursor()
	w, h := t.fxSize(height)
	c := t.newCanvas(w, h)
	glyphset := []rune("ｱｲｳｴｵｶｷｸｹｺｻｼｽｾｿﾀﾁﾂﾃﾄ0123456789✦thine")
	heads := make([]float64, w)
	speeds := make([]float64, w)
	for x := range heads {
		heads[x] = -rand.Float64() * float64(h) * 2
		speeds[x] = 0.3 + rand.Float64()*0.7
	}
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		c.clear()
		for x := 0; x < w; x += 2 {
			heads[x] += speeds[x]
			if heads[x]-8 > float64(h) {
				heads[x] = -rand.Float64() * float64(h)
			}
			for k := 0; k < 8; k++ {
				y := int(heads[x]) - k
				col := Lerp(Lighten(t.Pal.Success, 0.8), Darken(t.Pal.Success, 0.6), float64(k)/8)
				if k == 0 {
					col = Lighten(t.Pal.Success, 0.9)
				}
				c.set(x, y, glyphset[rand.IntN(len(glyphset))], col)
			}
		}
		c.flush()
		time.Sleep(60 * time.Millisecond)
	}
	c.erase()
}

// Fireworks launches rockets that burst into rings of theme-coloured sparks.
func (t *Term) Fireworks(d time.Duration, height int) {
	if !t.Animate {
		return
	}
	t.HideCursor()
	defer t.ShowCursor()
	w, h := t.fxSize(height)
	c := t.newCanvas(w, h)
	type spark struct {
		x, y, vx, vy, life float64
		col                RGB
	}
	type rocket struct {
		x, y, peak float64
		col        RGB
	}
	cols := t.palette()
	var rockets []rocket
	var sparks []spark
	end := time.Now().Add(d)
	frame := 0
	for time.Now().Before(end) || len(sparks) > 0 {
		if time.Now().Before(end) && frame%9 == 0 {
			rockets = append(rockets, rocket{x: 4 + rand.Float64()*float64(w-8), y: float64(h - 1),
				peak: 1 + rand.Float64()*float64(h)/2.5, col: cols[rand.IntN(len(cols))]})
		}
		c.clear()
		live := rockets[:0]
		for _, r := range rockets {
			r.y -= 0.9
			if r.y <= r.peak {
				for a := 0.0; a < 2*math.Pi; a += math.Pi / 9 {
					sparks = append(sparks, spark{x: r.x, y: r.y, vx: math.Cos(a) * 1.6, vy: math.Sin(a) * 0.7, life: 1, col: r.col})
				}
				continue
			}
			c.set(int(r.x), int(r.y), '│', Lighten(r.col, 0.5))
			live = append(live, r)
		}
		rockets = live
		ls := sparks[:0]
		for _, s := range sparks {
			s.x += s.vx
			s.y += s.vy
			s.vy += 0.08
			s.life -= 0.07
			if s.life <= 0 {
				continue
			}
			ch := '✦'
			if s.life < 0.5 {
				ch = '·'
			} else if s.life < 0.75 {
				ch = '•'
			}
			c.set(int(s.x), int(s.y), ch, Lerp(t.Pal.Subtle, Lighten(s.col, 0.2), s.life))
			ls = append(ls, s)
		}
		sparks = ls
		c.flush()
		time.Sleep(55 * time.Millisecond)
		frame++
		if frame > 400 {
			break
		}
	}
	c.erase()
}

// Wave animates text as a travelling gradient for d, then leaves it on stderr.
func (t *Term) Wave(text string, d time.Duration) {
	if !t.Animate {
		fmt.Fprintln(t.Err, t.Brand(text))
		return
	}
	t.HideCursor()
	defer t.ShowCursor()
	end := time.Now().Add(d)
	phase := 0.0
	for time.Now().Before(end) {
		io.WriteString(t.Err, "\r\x1b[2K"+GradientAt(t.Level, text, phase, t.Pal.Brand...))
		phase = math.Mod(phase+0.04, 1)
		time.Sleep(40 * time.Millisecond)
	}
	io.WriteString(t.Err, "\r\x1b[2K"+GradientAt(t.Level, text, 0, t.Pal.Brand...)+"\n")
}

// Celebrate is the payoff after a big win (e.g. cleanup freeing ≥ 1 GiB):
// a short confetti burst, then the message on stdout in the brand gradient.
func (t *Term) Celebrate(msg string) {
	if t.Animate {
		t.Confetti(900*time.Millisecond, 5)
	}
	fmt.Fprintln(t.Out, t.Brand(msg))
}
