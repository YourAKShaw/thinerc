package cleanup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/YourAKShaw/thinerc/internal/platform"
)

// ErrOutsideRoots is returned when a path escapes every allowlisted root.
var ErrOutsideRoots = errors.New("path is outside the allowed cleanup roots")

// denied are never touched, whatever a catalog or user config says (plan §6.5).
func denied() []string {
	h := platform.Home()
	p := platform.GetPaths()
	return []string{"/", h, "/System", "/usr", "/bin", "/sbin", "/etc", "/var/lib", "/Library", "/boot", "/opt",
		filepath.Join(h, ".ssh"), filepath.Join(h, ".gnupg"), filepath.Join(h, ".password-store"),
		filepath.Join(h, "Library", "Keychains"), filepath.Join(h, ".local", "share", "keyrings"),
		p.StateDir, p.Quarantine, p.ConfigDir}
}

// Guard is the single choke point every file action passes (invariant S2):
// the item's parent is canonicalised, the item must sit strictly inside an
// allowed root, outside the deny-list, never be a .git directory, and is
// examined with Lstat so a symlink is handled as a link, never followed.
func Guard(path string, roots []string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%s: not an absolute path", path)
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return err
	}
	canon := filepath.Join(parent, filepath.Base(path))
	if filepath.Base(canon) == ".git" || strings.Contains(canon, string(filepath.Separator)+".git"+string(filepath.Separator)) {
		return fmt.Errorf("%s: refusing to touch git metadata", canon)
	}
	for _, d := range denied() {
		if canon == filepath.Clean(d) {
			return fmt.Errorf("%s: protected location", canon)
		}
		if d != "/" && d != platform.Home() && within(canon, d) {
			return fmt.Errorf("%s: inside protected %s", canon, d)
		}
	}
	for _, r := range roots {
		cr, err := filepath.EvalSymlinks(r)
		if err != nil {
			continue
		}
		if within(canon, cr) && canon != cr {
			return nil
		}
	}
	return ErrOutsideRoots
}

func within(p, root string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// Fingerprint identifies a filesystem item at plan time (invariant S3).
type Fingerprint struct {
	Dev   uint64 `json:"dev"`
	Ino   uint64 `json:"ino"`
	MTime int64  `json:"mtime_ns"`
	Mode  uint32 `json:"mode"`
}

// FingerprintOf takes an Lstat fingerprint.
func FingerprintOf(path string) (Fingerprint, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return Fingerprint{}, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return Fingerprint{}, errors.New("no stat data")
	}
	return Fingerprint{Dev: uint64(st.Dev), Ino: uint64(st.Ino), MTime: fi.ModTime().UnixNano(), Mode: uint32(fi.Mode())}, nil
}

// Revalidate reports whether path is still exactly the item that was planned.
func Revalidate(path string, fp Fingerprint) error {
	now, err := FingerprintOf(path)
	if err != nil {
		return fmt.Errorf("gone since the plan was made")
	}
	if now.Dev != fp.Dev || now.Ino != fp.Ino || now.Mode != fp.Mode {
		return fmt.Errorf("replaced since the plan was made")
	}
	if now.MTime != fp.MTime {
		return fmt.Errorf("modified since the plan was made")
	}
	return nil
}
