// Package oplog is thine's write-ahead operations log (invariant S7): an
// "intent" record is fsynced before any state change and an "outcome" record
// after it, so even a crash leaves evidence that `thine history` can show.
package oplog

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/YourAKShaw/thinerc/internal/buildinfo"
	"github.com/YourAKShaw/thinerc/internal/platform"
)

// Item is one planned or executed action.
type Item struct {
	ID       string `json:"id"`
	Action   string `json:"action"` // signal | quarantine | delete | exec | restore
	Target   string `json:"target"`
	Bytes    int64  `json:"bytes,omitempty"`
	Result   string `json:"result,omitempty"` // done | skipped | failed | refused
	Error    string `json:"error,omitempty"`
	Undoable bool   `json:"undoable,omitempty"`
}

// Record is one JSONL line.
type Record struct {
	TS      time.Time `json:"ts"`
	OpID    string    `json:"op_id"`
	Phase   string    `json:"phase"` // intent | outcome
	Verb    string    `json:"verb"`
	Argv    []string  `json:"argv,omitempty"`
	Caller  string    `json:"caller"`
	Items   []Item    `json:"items,omitempty"`
	Result  string    `json:"result,omitempty"` // completed | partial | aborted | failed
	Bytes   int64     `json:"bytes,omitempty"`
	Version string    `json:"thine_version"`
}

const (
	maxSize  = 5 << 20
	maxFiles = 3
)

var mu sync.Mutex

// crockford is the ULID alphabet: sortable, unambiguous, URL-safe.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewID returns a 26-character time-ordered ID (ULID layout).
func NewID() string {
	var b [16]byte
	ms := uint64(time.Now().UnixMilli())
	for i := 5; i >= 0; i-- {
		b[i] = byte(ms)
		ms >>= 8
	}
	rand.Read(b[6:])
	// 128 bits, read as a 130-bit number, give 26 base32 symbols.
	var hi, lo uint64
	for i := 0; i < 8; i++ {
		hi = hi<<8 | uint64(b[i])
		lo = lo<<8 | uint64(b[i+8])
	}
	out := make([]byte, 26)
	for i := 25; i >= 0; i-- {
		out[i] = crockford[lo&0x1f]
		lo = lo>>5 | hi<<59
		hi >>= 5
	}
	return string(out)
}

var secretArg = regexp.MustCompile(`(?i)((?:token|secret|password|passwd|api[_-]?key|auth|bearer|cookie)[=:])\S+`)
var urlCreds = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)[^/@\s]+@`)

// Redact removes credentials from argv before it is written to disk (§6.6).
func Redact(argv []string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		a = secretArg.ReplaceAllString(a, "${1}REDACTED")
		a = urlCreds.ReplaceAllString(a, "${1}REDACTED@")
		out[i] = a
	}
	return out
}

// Append writes one record with fsync, rotating the log at 5 MiB.
func Append(r Record) error {
	mu.Lock()
	defer mu.Unlock()
	p := platform.GetPaths()
	if err := platform.EnsurePrivateDir(p.StateDir); err != nil {
		return err
	}
	if r.TS.IsZero() {
		r.TS = time.Now().UTC()
	}
	r.Argv = Redact(r.Argv)
	if r.Version == "" {
		r.Version = buildinfo.Get().Version
	}
	if st, err := os.Stat(p.OpsLog); err == nil && st.Size() > maxSize {
		rotate(p.OpsLog)
	}
	f, err := os.OpenFile(p.OpsLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

func rotate(path string) {
	os.Remove(fmt.Sprintf("%s.%d", path, maxFiles-1))
	for i := maxFiles - 2; i >= 1; i-- {
		os.Rename(fmt.Sprintf("%s.%d", path, i), fmt.Sprintf("%s.%d", path, i+1))
	}
	os.Rename(path, path+".1")
}

// Op is an operation reconstructed from its intent and outcome records.
type Op struct {
	ID       string
	Verb     string
	Caller   string
	Started  time.Time
	Result   string // completed | partial | aborted | failed | incomplete (crash)
	Bytes    int64
	Items    []Item
	Undoable bool
}

// Ops returns operations newest first, merging rotated files.
func Ops() ([]Op, error) {
	p := platform.GetPaths()
	files := []string{p.OpsLog}
	for i := 1; i < maxFiles; i++ {
		files = append(files, fmt.Sprintf("%s.%d", p.OpsLog, i))
	}
	byID := map[string]*Op{}
	for _, fn := range files {
		f, err := os.Open(fn)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64<<10), 8<<20)
		for sc.Scan() {
			var r Record
			if json.Unmarshal(sc.Bytes(), &r) != nil {
				continue
			}
			op, ok := byID[r.OpID]
			if !ok {
				op = &Op{ID: r.OpID, Verb: r.Verb, Caller: r.Caller, Started: r.TS, Result: "incomplete"}
				byID[r.OpID] = op
			}
			if r.Phase == "intent" {
				op.Started = r.TS
				if len(op.Items) == 0 {
					op.Items = r.Items
				}
			} else {
				op.Result, op.Bytes, op.Items = r.Result, r.Bytes, r.Items
			}
		}
		f.Close()
	}
	out := make([]Op, 0, len(byID))
	for _, op := range byID {
		for _, it := range op.Items {
			if it.Undoable && it.Result == "done" {
				op.Undoable = true
			}
		}
		out = append(out, *op)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

// Find returns an op by ID or unambiguous prefix.
func Find(prefix string) (Op, error) {
	ops, err := Ops()
	if err != nil {
		return Op{}, err
	}
	var hits []Op
	for _, op := range ops {
		if strings.HasPrefix(op.ID, strings.ToUpper(prefix)) {
			hits = append(hits, op)
		}
	}
	switch len(hits) {
	case 0:
		return Op{}, fmt.Errorf("no operation matches %q", prefix)
	case 1:
		return hits[0], nil
	}
	return Op{}, fmt.Errorf("%q matches %d operations; use more characters", prefix, len(hits))
}

// LogPath is shown in docs and doctor output.
func LogPath() string { return filepath.Clean(platform.GetPaths().OpsLog) }
