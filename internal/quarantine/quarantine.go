// Package quarantine is thine's undo buffer: user files are moved (atomic,
// same-filesystem rename) instead of deleted, kept for a retention period,
// and restorable with `thine undo` (plan §6.5).
package quarantine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/YourAKShaw/thinerc/internal/platform"
)

// ErrCrossFS means the item lives on a different filesystem than the
// quarantine, so an atomic move is impossible.
var ErrCrossFS = errors.New("item is on a different filesystem than the quarantine")

// Entry records one quarantined item.
type Entry struct {
	ID            string      `json:"id"`
	Original      string      `json:"original"`
	Stored        string      `json:"stored"`
	Mode          fs.FileMode `json:"mode"`
	Bytes         int64       `json:"bytes"`
	QuarantinedAt time.Time   `json:"quarantined_at"`
	Restored      bool        `json:"restored,omitempty"`
	RestoredTo    string      `json:"restored_to,omitempty"`
}

// Manifest describes everything one operation quarantined.
type Manifest struct {
	OpID    string    `json:"op_id"`
	Verb    string    `json:"verb"`
	Created time.Time `json:"created"`
	Entries []Entry   `json:"entries"`
}

var mu sync.Mutex

// Root is the quarantine directory.
func Root() string { return platform.GetPaths().Quarantine }

func opDir(opID string) string { return filepath.Join(Root(), opID) }

func readManifest(opID string) (Manifest, error) {
	m := Manifest{OpID: opID}
	data, err := os.ReadFile(filepath.Join(opDir(opID), "manifest.json"))
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(data, &m)
}

func writeManifest(m Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	p := filepath.Join(opDir(m.OpID), "manifest.json")
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// CanHold reports whether path can be quarantined atomically.
func CanHold(path string) bool {
	if platform.EnsurePrivateDir(Root()) != nil {
		return false
	}
	return platform.SameFilesystem(filepath.Dir(path), Root())
}

// Move quarantines path for operation opID. The caller must already have
// re-validated the item (invariant S3).
func Move(opID, verb, itemID, path string, bytes int64) (Entry, error) {
	mu.Lock()
	defer mu.Unlock()
	fi, err := os.Lstat(path)
	if err != nil {
		return Entry{}, err
	}
	if !CanHold(path) {
		return Entry{}, ErrCrossFS
	}
	m, err := readManifest(opID)
	if err != nil {
		return Entry{}, err
	}
	if m.Created.IsZero() {
		m.Created, m.Verb = time.Now().UTC(), verb
	}
	slot := filepath.Join(opDir(opID), strconv.Itoa(len(m.Entries)))
	if err := platform.EnsurePrivateDir(slot); err != nil {
		return Entry{}, err
	}
	stored := filepath.Join(slot, filepath.Base(path))
	if err := os.Rename(path, stored); err != nil {
		os.Remove(slot)
		return Entry{}, err
	}
	e := Entry{ID: itemID, Original: path, Stored: stored, Mode: fi.Mode(), Bytes: bytes, QuarantinedAt: time.Now().UTC()}
	m.Entries = append(m.Entries, e)
	return e, writeManifest(m)
}

// Restored describes one undo.
type Restored struct {
	Entry
	To       string `json:"to"`
	Conflict bool   `json:"conflict"`
}

// Restore puts an operation's items back. If the original path is occupied,
// the item lands at <path>.thine-restored-<n> instead of overwriting anything.
func Restore(opID string, only map[string]bool) ([]Restored, error) {
	mu.Lock()
	defer mu.Unlock()
	m, err := readManifest(opID)
	if err != nil {
		return nil, err
	}
	if len(m.Entries) == 0 {
		return nil, fmt.Errorf("operation %s has nothing in quarantine (already restored or purged)", opID)
	}
	var out []Restored
	var errs []error
	for i, e := range m.Entries {
		if e.Restored || (len(only) > 0 && !only[e.ID]) {
			continue
		}
		if _, err := os.Lstat(e.Stored); err != nil {
			errs = append(errs, fmt.Errorf("%s: quarantined copy is missing (purged?)", e.Original))
			continue
		}
		to := e.Original
		conflict := false
		if _, err := os.Lstat(to); err == nil {
			conflict = true
			for n := 1; ; n++ {
				cand := fmt.Sprintf("%s.thine-restored-%d", e.Original, n)
				if _, err := os.Lstat(cand); errors.Is(err, os.ErrNotExist) {
					to = cand
					break
				}
			}
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := os.Rename(e.Stored, to); err != nil {
			errs = append(errs, err)
			continue
		}
		os.Remove(filepath.Dir(e.Stored))
		m.Entries[i].Restored, m.Entries[i].RestoredTo = true, to
		out = append(out, Restored{Entry: e, To: to, Conflict: conflict})
	}
	if err := writeManifest(m); err != nil {
		errs = append(errs, err)
	}
	return out, errors.Join(errs...)
}

// Summary is one operation's quarantine footprint.
type Summary struct {
	OpID    string    `json:"op_id"`
	Verb    string    `json:"verb"`
	Created time.Time `json:"created"`
	Items   int       `json:"items"`
	Bytes   int64     `json:"bytes"`
}

// List summarises quarantine contents, newest first.
func List() ([]Summary, error) {
	ents, err := os.ReadDir(Root())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Summary
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		m, err := readManifest(e.Name())
		if err != nil || m.Created.IsZero() {
			continue
		}
		s := Summary{OpID: m.OpID, Verb: m.Verb, Created: m.Created}
		for _, en := range m.Entries {
			if !en.Restored {
				s.Items++
				s.Bytes += en.Bytes
			}
		}
		if s.Items > 0 {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out, nil
}

// Purge permanently deletes quarantined operations selected by keep()==false.
// Deletion goes through os.Root so it can never escape the quarantine.
func Purge(selectFn func(Summary) bool) (int64, int, error) {
	mu.Lock()
	defer mu.Unlock()
	sums, err := List()
	if err != nil {
		return 0, 0, err
	}
	root, err := os.OpenRoot(Root())
	if err != nil {
		return 0, 0, err
	}
	defer root.Close()
	var freed int64
	n := 0
	var errs []error
	for _, s := range sums {
		if !selectFn(s) {
			continue
		}
		if err := root.RemoveAll(s.OpID); err != nil {
			errs = append(errs, err)
			continue
		}
		freed += s.Bytes
		n++
	}
	return freed, n, errors.Join(errs...)
}
