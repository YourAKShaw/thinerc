//go:build darwin

package procs

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/YourAKShaw/thinerc/internal/platform"
)

const psCols = "pid=,ppid=,pgid=,uid=,rss=,lstart=,comm="

// Lookup reads one process with ps(1); macOS has no /proc and kinfo_proc
// carries no RSS without cgo.
func Lookup(pid int) (Proc, error) {
	ctx := context.Background()
	out, err := platform.Output(ctx, 2*time.Second, "ps", "-o", psCols, "-p", strconv.Itoa(pid))
	if err != nil || out == "" {
		return Proc{}, ErrGone
	}
	p, err := parsePSRow(out)
	if err != nil {
		return Proc{}, err
	}
	p.User = Username(p.UID)
	if args, err := platform.Output(ctx, 2*time.Second, "ps", "-o", "args=", "-p", strconv.Itoa(pid)); err == nil {
		p.Cmdline = strings.Fields(args)
	}
	p.Cwd = cwdOf(pid)
	return p, nil
}

func cwdOf(pid int) string {
	out, err := platform.Output(context.Background(), 2*time.Second, "lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn")
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "n") {
			return l[1:]
		}
	}
	return ""
}

// List returns all processes from a single ps call.
func List() ([]Proc, error) {
	out, err := platform.Output(context.Background(), 5*time.Second, "ps", "-axo", psCols)
	if err != nil {
		return nil, err
	}
	var procs []Proc
	for _, line := range strings.Split(out, "\n") {
		p, err := parsePSRow(line)
		if err != nil {
			continue
		}
		p.User = Username(p.UID)
		p.Cmdline = []string{p.Exe}
		procs = append(procs, p)
	}
	return procs, nil
}

// PSS is unavailable on macOS; callers fall back to RSS.
func PSS(int) int64 { return 0 }

// Alive reports whether exactly this identity is still running.
func Alive(id Identity) bool {
	p, err := Lookup(id.PID)
	return err == nil && p.StartTime == id.StartTime
}

// Signal checks the start time, then signals. A microsecond window remains
// between check and kill(2); macOS offers no pidfd equivalent without cgo.
func Signal(id Identity, sig syscall.Signal) error {
	if id.PID <= 1 {
		return ErrPermission
	}
	p, err := Lookup(id.PID)
	if err != nil {
		return ErrGone
	}
	if p.StartTime != id.StartTime {
		return ErrIdentityChanged
	}
	switch err := unix.Kill(id.PID, sig); {
	case err == nil:
		return nil
	case errors.Is(err, unix.ESRCH):
		return ErrGone
	case errors.Is(err, unix.EPERM):
		return ErrPermission
	default:
		return err
	}
}
