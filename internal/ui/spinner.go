package ui

import (
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"time"
)

// StarFrames is thine's logo in motion: a star that twinkles from a dot to a
// full burst and back — the counterpart of Claude Code's pulsing asterisk.
var StarFrames = []string{"·", "✧", "✦", "✶", "✷", "✸", "✹", "✸", "✷", "✶", "✦", "✧"}

// FrameInterval is 12 fps: smooth, cheap, and calm enough for long scans.
const FrameInterval = 83 * time.Millisecond

// Spinner is the animated "working…" line on stderr. When animation is off it
// prints nothing until Done, so piped output stays clean.
type Spinner struct {
	t       *Term
	mu      sync.Mutex
	msg     string
	detail  string
	start   time.Time
	stop    chan struct{}
	stopped chan struct{}
	active  bool
}

// Spin starts a spinner with a message such as "Scanning ports".
func (t *Term) Spin(msg string) *Spinner {
	s := &Spinner{t: t, msg: msg, start: time.Now(), stop: make(chan struct{}), stopped: make(chan struct{})}
	if !t.Animate || t.Opts.Quiet {
		close(s.stopped)
		return s
	}
	s.active = true
	t.HideCursor()
	go s.loop()
	return s
}

// Update replaces the message.
func (s *Spinner) Update(msg string) { s.mu.Lock(); s.msg = msg; s.mu.Unlock() }

// Detail sets the muted progress text after the message ("3/12 · 1.2 GiB").
func (s *Spinner) Detail(d string) { s.mu.Lock(); s.detail = d; s.mu.Unlock() }

func (s *Spinner) loop() {
	defer close(s.stopped)
	tick := time.NewTicker(FrameInterval)
	defer tick.Stop()
	frame := 0
	for {
		s.render(frame)
		select {
		case <-s.stop:
			io.WriteString(s.t.Err, "\r\x1b[2K")
			return
		case <-tick.C:
			frame++
		}
	}
}

func (s *Spinner) render(frame int) {
	s.mu.Lock()
	msg, detail := s.msg, s.detail
	s.mu.Unlock()
	t := s.t
	elapsed := time.Since(s.start)

	// The star walks along the brand gradient while it twinkles.
	phase := math.Mod(float64(frame)/float64(len(StarFrames)*3), 1)
	starColor := Along(t.Pal.Brand, 1-math.Abs(1-2*phase))
	star := S().Bold().Fg(starColor).RenderAt(t.Level, StarFrames[frame%len(StarFrames)])

	// A highlight sweeps across the message, like light catching foil.
	text := msg + "…"
	n := float64(len([]rune(text)))
	sweep := math.Mod(float64(frame)*0.55, n+8) - 4
	shim := Shimmer(t.Level, text, sweep, Darken(t.Pal.Accent, 0.15), Lighten(t.Pal.Accent, 0.75))

	meta := fmt.Sprintf("(%s · ctrl+c to cancel)", compactElapsed(elapsed))
	if detail != "" {
		meta = fmt.Sprintf("(%s · %s · ctrl+c to cancel)", detail, compactElapsed(elapsed))
	}
	line := star + " " + shim + " " + t.Muted(meta)
	if w := t.Width(); Width(line) > w-1 {
		line = star + " " + shim
	}
	t.mu.Lock()
	io.WriteString(t.Err, "\r\x1b[2K"+line)
	t.mu.Unlock()
}

func compactElapsed(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

// Stop clears the spinner line.
func (s *Spinner) Stop() {
	if s.active {
		select {
		case <-s.stop:
		default:
			close(s.stop)
		}
		<-s.stopped
		s.t.ShowCursor()
		s.active = false
	}
}

// Done stops the spinner and leaves a permanent step line on stdout.
func (s *Spinner) Done(k Kind, final string) {
	s.Stop()
	s.t.Step(k, final)
}

// Elapsed is how long the spinner has been running.
func (s *Spinner) Elapsed() time.Duration { return time.Since(s.start) }

// Typewriter prints text to stderr one rune at a time (skipped without animation).
func (t *Term) Typewriter(text string, perRune time.Duration) {
	if !t.Animate {
		fmt.Fprintln(t.Err, text)
		return
	}
	// Escape sequences must be written whole, so walk visible runes only.
	i := 0
	for i < len(text) {
		if loc := ansiRE.FindStringIndex(text[i:]); loc != nil && loc[0] == 0 {
			io.WriteString(t.Err, text[i:i+loc[1]])
			i += loc[1]
			continue
		}
		r := []rune(text[i:])[0]
		io.WriteString(t.Err, string(r))
		i += len(string(r))
		if r != ' ' {
			time.Sleep(perRune)
		}
	}
	io.WriteString(t.Err, "\n")
}

// Countdown renders an animated draining bar for a grace period; it returns
// early when done() reports true. Without animation it simply polls.
func (t *Term) Countdown(label string, total time.Duration, poll time.Duration, done func() bool) bool {
	start := time.Now()
	deadline := start.Add(total)
	for {
		if done() {
			if t.Animate {
				io.WriteString(t.Err, "\r\x1b[2K")
			}
			return true
		}
		now := time.Now()
		if !now.Before(deadline) {
			if t.Animate {
				io.WriteString(t.Err, "\r\x1b[2K")
			}
			return false
		}
		if t.Animate {
			left := deadline.Sub(now)
			frac := float64(left) / float64(total)
			frame := int(now.Sub(start) / FrameInterval)
			star := S().Fg(Along(t.Pal.Brand, frac)).RenderAt(t.Level, StarFrames[frame%len(StarFrames)])
			bar := t.Bar(frac, 16, t.Pal.Success, t.Pal.Warning, t.Pal.Error)
			t.mu.Lock()
			io.WriteString(t.Err, "\r\x1b[2K"+star+" "+t.Text(label)+" "+bar+" "+t.Muted(fmt.Sprintf("%.1fs", left.Seconds())))
			t.mu.Unlock()
		}
		time.Sleep(poll)
	}
}

// Pulse briefly animates a symbol fading out (◉ ◎ ○ ·), used when a port frees.
func (t *Term) Pulse(prefix string, c RGB) {
	if !t.Animate {
		return
	}
	frames := []string{"◉", "◎", "○", "◌", "·"}
	for i, f := range frames {
		col := Lerp(c, t.Pal.Subtle, float64(i)/float64(len(frames)-1))
		io.WriteString(t.Err, "\r\x1b[2K"+prefix+S().Bold().Fg(col).RenderAt(t.Level, f))
		time.Sleep(45 * time.Millisecond)
	}
	io.WriteString(t.Err, "\r\x1b[2K")
}

// Repeat is strings.Repeat for callers that only import ui.
func Repeat(s string, n int) string { return strings.Repeat(s, max(0, n)) }
