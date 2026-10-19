//go:build linux

package procs

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const clkTck = 100 // USER_HZ is 100 on every Linux ABI Go supports

var bootTime = func() time.Time {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return time.Time{}
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "btime "); ok {
			if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
				return time.Unix(n, 0)
			}
		}
	}
	return time.Time{}
}()

var pageSize = int64(os.Getpagesize())

// Lookup reads one process from /proc.
func Lookup(pid int) (Proc, error) {
	dir := "/proc/" + strconv.Itoa(pid)
	raw, err := os.ReadFile(dir + "/stat")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Proc{}, ErrGone
		}
		return Proc{}, err
	}
	st, err := parseStat(string(raw))
	if err != nil {
		return Proc{}, err
	}
	p := Proc{
		Identity: Identity{PID: pid, StartTime: st.StartTime},
		PPID:     st.PPID, PGID: st.PGID, Name: st.Comm,
		RSS: st.RSSPages * pageSize,
	}
	if !bootTime.IsZero() {
		p.Started = bootTime.Add(time.Duration(st.StartTime) * time.Second / clkTck)
	}
	if fi, err := os.Stat(dir); err == nil {
		if s, ok := fi.Sys().(*syscall.Stat_t); ok {
			p.UID = int(s.Uid)
		}
	}
	p.User = Username(p.UID)
	if b, err := os.ReadFile(dir + "/cmdline"); err == nil {
		p.Cmdline = parseCmdline(b)
	}
	if exe, err := os.Readlink(dir + "/exe"); err == nil {
		p.Exe = strings.TrimSuffix(exe, " (deleted)")
	}
	if cwd, err := os.Readlink(dir + "/cwd"); err == nil {
		p.Cwd = cwd
	}
	return p, nil
}

// List returns every visible process. Kernel threads (no cmdline, child of
// kthreadd) are skipped.
func List() ([]Proc, error) {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	out := make([]Proc, 0, len(ents))
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		p, err := Lookup(pid)
		if err != nil || (p.PPID == 2 && len(p.Cmdline) == 0) || pid == 2 {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// PSS refines memory for one of the user's own processes via smaps_rollup, so
// shared libraries are not double-counted. Returns 0 when unavailable.
func PSS(pid int) int64 {
	f, err := os.Open(filepath.Join("/proc", strconv.Itoa(pid), "smaps_rollup"))
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "Pss:"); ok {
			kb, _ := strconv.ParseInt(strings.Fields(v)[0], 10, 64)
			return kb * 1024
		}
	}
	return 0
}

// Alive reports whether exactly this identity is still running.
func Alive(id Identity) bool {
	p, err := Lookup(id.PID)
	if err != nil {
		return false
	}
	if p.StartTime != id.StartTime {
		return false
	}
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(id.PID) + "/stat")
	if err == nil {
		if st, err := parseStat(string(raw)); err == nil && st.State == 'Z' {
			return false // zombie: already dead, waiting to be reaped
		}
	}
	return true
}

// Signal delivers sig to exactly this identity. With pidfd (Linux 5.3+) the
// check and the signal are race-free; older kernels fall back to a start-time
// check immediately before kill(2).
func Signal(id Identity, sig syscall.Signal) error {
	if id.PID <= 1 {
		return ErrPermission
	}
	fd, err := unix.PidfdOpen(id.PID, 0)
	if err == nil {
		defer unix.Close(fd)
		p, lerr := Lookup(id.PID)
		if lerr != nil {
			return ErrGone
		}
		if p.StartTime != id.StartTime {
			return ErrIdentityChanged
		}
		return mapErr(unix.PidfdSendSignal(fd, sig, nil, 0))
	}
	if errors.Is(err, unix.ESRCH) {
		return ErrGone
	}
	p, lerr := Lookup(id.PID)
	if lerr != nil {
		return ErrGone
	}
	if p.StartTime != id.StartTime {
		return ErrIdentityChanged
	}
	return mapErr(unix.Kill(id.PID, sig))
}

func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, unix.ESRCH):
		return ErrGone
	case errors.Is(err, unix.EPERM):
		return ErrPermission
	}
	return err
}
