// Package procs inspects and signals processes. A process is addressed by its
// Identity — PID plus start time — so a recycled PID is never signalled
// (invariant S3/S4).
package procs

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Identity defeats PID reuse: a PID only means the same process while its
// start time matches.
type Identity struct {
	PID       int    `json:"pid"`
	StartTime uint64 `json:"start_time"` // Linux: clock ticks since boot; macOS: unix seconds
}

// Proc is a snapshot of one process.
type Proc struct {
	Identity
	PPID    int       `json:"ppid"`
	PGID    int       `json:"pgid"`
	UID     int       `json:"uid"`
	User    string    `json:"user"`
	Name    string    `json:"name"`
	Exe     string    `json:"exe,omitempty"`
	Cmdline []string  `json:"cmdline"`
	Cwd     string    `json:"cwd,omitempty"`
	RSS     int64     `json:"rss_bytes"`
	PSS     int64     `json:"pss_bytes,omitempty"`
	Started time.Time `json:"started"`
}

var (
	// ErrGone means the process no longer exists.
	ErrGone = errors.New("process has exited")
	// ErrIdentityChanged means the PID now belongs to a different process.
	ErrIdentityChanged = errors.New("PID was reused by a different process")
	// ErrPermission means the process belongs to someone else.
	ErrPermission = errors.New("operation not permitted")
)

// Uptime is how long the process has been running.
func (p Proc) Uptime() time.Duration {
	if p.Started.IsZero() {
		return 0
	}
	return time.Since(p.Started)
}

// CommandLine joins argv for display, falling back to the name.
func (p Proc) CommandLine() string {
	if len(p.Cmdline) == 0 {
		return p.Name
	}
	return strings.Join(p.Cmdline, " ")
}

// MatchNames are the names used for classification: the kernel name and the
// basename of argv[0] ("node", "next-server", "python3.12").
func (p Proc) MatchNames() []string {
	names := []string{strings.ToLower(p.Name)}
	if len(p.Cmdline) > 0 {
		b := strings.ToLower(filepath.Base(p.Cmdline[0]))
		if b != "" && !slices.Contains(names, b) {
			names = append(names, b)
		}
	}
	return names
}

var (
	userMu    sync.Mutex
	userCache = map[int]string{}
)

// Username resolves a UID (cached; pure Go, no cgo).
func Username(uid int) string {
	userMu.Lock()
	defer userMu.Unlock()
	if n, ok := userCache[uid]; ok {
		return n
	}
	n := strconv.Itoa(uid)
	if u, err := user.LookupId(n); err == nil {
		n = u.Username
	}
	userCache[uid] = n
	return n
}

// Self is this process's UID.
func Self() int { return os.Getuid() }

// Ancestors returns thine's own ancestry (shell, terminal, multiplexer, IDE…)
// so they can be protected from signals.
func Ancestors() map[int]bool {
	out := map[int]bool{}
	pid := os.Getpid()
	for i := 0; i < 64 && pid > 1; i++ {
		out[pid] = true
		p, err := Lookup(pid)
		if err != nil || p.PPID == pid {
			break
		}
		pid = p.PPID
	}
	return out
}

// GroupMembers returns processes sharing pgid, from a full listing.
func GroupMembers(all []Proc, pgid int) []Proc {
	var out []Proc
	for _, p := range all {
		if p.PGID == pgid {
			out = append(out, p)
		}
	}
	return out
}

// WaitGone polls until the identity is gone or the deadline passes.
func WaitGone(id Identity, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if !Alive(id) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}
