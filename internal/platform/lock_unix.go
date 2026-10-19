//go:build unix

package platform

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// ErrLocked means another thine apply holds the state lock (invariant S9).
var ErrLocked = errors.New("another thine operation is applying changes")

// Lock is an exclusive advisory lock on the state directory.
type Lock struct{ f *os.File }

// AcquireLock takes the per-user apply lock without blocking.
func AcquireLock(stateDir string) (*Lock, error) {
	if err := EnsurePrivateDir(stateDir); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(stateDir, "apply.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, err
	}
	return &Lock{f: f}, nil
}

// Release drops the lock.
func (l *Lock) Release() {
	if l != nil && l.f != nil {
		unix.Flock(int(l.f.Fd()), unix.LOCK_UN)
		l.f.Close()
	}
}

// SameFilesystem reports whether two existing paths share a device (quarantine
// moves must be atomic same-filesystem renames).
func SameFilesystem(a, b string) bool {
	var sa, sb unix.Stat_t
	if unix.Stat(a, &sa) != nil || unix.Stat(b, &sb) != nil {
		return false
	}
	return sa.Dev == sb.Dev
}

// DiskUsage returns total and available bytes on the filesystem holding path.
func DiskUsage(path string) (total, avail uint64, err error) {
	var st unix.Statfs_t
	if err = unix.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	return st.Blocks * uint64(st.Bsize), st.Bavail * uint64(st.Bsize), nil
}
