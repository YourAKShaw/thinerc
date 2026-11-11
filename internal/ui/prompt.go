package ui

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

var stdinReader = bufio.NewReader(os.Stdin)

// promptCancel, once closed, makes pending and future prompts answer "no".
var promptCancel <-chan struct{}

// CancelPromptsOn stops prompts from blocking on stdin once done is closed, so
// an interrupt during protected work can't hang on a question.
func CancelPromptsOn(done <-chan struct{}) { promptCancel = done }

func (t *Term) readLine(prompt string) (string, bool) {
	select {
	case <-promptCancel:
		return "", false
	default:
	}
	fmt.Fprint(t.Err, prompt)
	type reply struct {
		line string
		err  error
	}
	got := make(chan reply, 1)
	go func() {
		line, err := stdinReader.ReadString('\n')
		got <- reply{line, err}
	}()
	select {
	case r := <-got:
		if r.err != nil { // EOF or Ctrl-D means "no"
			fmt.Fprintln(t.Err)
			return "", false
		}
		return strings.TrimSpace(r.line), true
	case <-promptCancel:
		fmt.Fprintln(t.Err)
		return "", false
	}
}

// Confirm asks a [y/N] question. The safe answer (no) is the default, and a
// non-interactive session always answers no.
func (t *Term) Confirm(question string) bool {
	if !t.Interactive() {
		return false
	}
	q := S().Bold().Fg(t.Pal.Accent).RenderAt(t.Level, "? ") + t.Text(question) + " " + t.Muted("[y/N]") + " "
	ans, ok := t.readLine(q)
	if !ok {
		return false
	}
	ans = strings.ToLower(ans)
	return ans == "y" || ans == "yes"
}

// ConfirmTyped requires the exact phrase for destructive actions (plan §6.2).
func (t *Term) ConfirmTyped(question, phrase string) bool {
	if !t.Interactive() {
		return false
	}
	fmt.Fprintln(t.Err, t.Bad(t.Icons.Warn+" ")+t.Text(question))
	ans, ok := t.readLine("  " + t.Muted("type ") + t.Code(phrase) + t.Muted(" to continue: "))
	return ok && ans == phrase
}

// Ask reads a free-form answer with a default.
func (t *Term) Ask(question, def string) string {
	if !t.Interactive() {
		return def
	}
	hint := ""
	if def != "" {
		hint = " " + t.Muted("("+def+")")
	}
	ans, ok := t.readLine(S().Bold().Fg(t.Pal.Accent).RenderAt(t.Level, "? ") + t.Text(question) + hint + " ")
	if !ok || ans == "" {
		return def
	}
	return ans
}

// ParseSelection turns "1,3-5", "all" or "none" into sorted 0-based indexes.
func ParseSelection(s string, n int) ([]int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "", "none", "n", "no":
		return nil, nil
	case "all", "a", "*":
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out, nil
	}
	seen := map[int]bool{}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		lo, hi := part, part
		if a, b, ok := strings.Cut(part, "-"); ok {
			lo, hi = a, b
		}
		l, err1 := strconv.Atoi(lo)
		h, err2 := strconv.Atoi(hi)
		if err1 != nil || err2 != nil || l < 1 || h > n || l > h {
			return nil, fmt.Errorf("%q is not a valid choice (use numbers 1–%d, ranges like 2-4, all or none)", part, n)
		}
		for i := l; i <= h; i++ {
			seen[i-1] = true
		}
	}
	out := make([]int, 0, len(seen))
	for i := range seen {
		out = append(out, i)
	}
	sort.Ints(out)
	return out, nil
}
