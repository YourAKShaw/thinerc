package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/YourAKShaw/thinerc/internal/platform"
)

// TrustEntry pins one project config by content hash (invariant S12): any
// edit to the file — say, a pulled commit redefining killfb — needs re-trust.
type TrustEntry struct {
	Path      string    `json:"path"`
	SHA256    string    `json:"sha256"`
	TrustedAt time.Time `json:"trusted_at"`
}

type trustFile struct {
	Version int                   `json:"version"`
	Entries map[string]TrustEntry `json:"entries"`
}

func readTrust() (trustFile, error) {
	tf := trustFile{Version: 1, Entries: map[string]TrustEntry{}}
	data, err := os.ReadFile(platform.GetPaths().TrustFile)
	if errors.Is(err, os.ErrNotExist) {
		return tf, nil
	}
	if err != nil {
		return tf, err
	}
	if err := json.Unmarshal(data, &tf); err != nil {
		return tf, err
	}
	if tf.Entries == nil {
		tf.Entries = map[string]TrustEntry{}
	}
	return tf, nil
}

func writeTrust(tf trustFile) error {
	p := platform.GetPaths()
	if err := platform.EnsurePrivateDir(p.StateDir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return err
	}
	tmp := p.TrustFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p.TrustFile)
}

// FileHash is the hex SHA-256 of a file.
func FileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// TrustState is "trusted", "changed" (trusted once, edited since) or "untrusted".
func TrustState(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	tf, err := readTrust()
	if err != nil {
		return "", err
	}
	e, ok := tf.Entries[abs]
	if !ok {
		return "untrusted", nil
	}
	h, err := FileHash(abs)
	if err != nil {
		return "", err
	}
	if h != e.SHA256 {
		return "changed", nil
	}
	return "trusted", nil
}

// Trust pins the current content of path.
func Trust(path string) (TrustEntry, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return TrustEntry{}, err
	}
	h, err := FileHash(abs)
	if err != nil {
		return TrustEntry{}, err
	}
	tf, err := readTrust()
	if err != nil {
		return TrustEntry{}, err
	}
	e := TrustEntry{Path: abs, SHA256: h, TrustedAt: time.Now().UTC()}
	tf.Entries[abs] = e
	return e, writeTrust(tf)
}

// Untrust forgets path; it reports whether an entry existed.
func Untrust(path string) (bool, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	tf, err := readTrust()
	if err != nil {
		return false, err
	}
	if _, ok := tf.Entries[abs]; !ok {
		return false, nil
	}
	delete(tf.Entries, abs)
	return true, writeTrust(tf)
}

// TrustList returns all pinned files, sorted by path.
func TrustList() ([]TrustEntry, error) {
	tf, err := readTrust()
	if err != nil {
		return nil, err
	}
	out := make([]TrustEntry, 0, len(tf.Entries))
	for _, e := range tf.Entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
