package cleanup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YourAKShaw/thinerc/internal/platform"
	"github.com/YourAKShaw/thinerc/internal/quarantine"
)

// Outcome is what happened to one item.
type Outcome struct {
	Item
	Result string `json:"result"` // done | skipped | failed
	Freed  int64  `json:"freed_bytes"`
	Error  string `json:"error,omitempty"`
}

// Hooks let the CLI animate progress without the engine knowing about UI.
type Hooks struct {
	Start func(it Item, i, n int)
	Done  func(o Outcome)
	Sudo  func(argv []string) bool // print the argv and confirm; false skips the item
}

// Apply executes a plan item by item. Each item is re-validated right before
// it is acted on (S3); a cancelled context stops after the current item (S10).
func Apply(ctx context.Context, opID string, p Plan, roots []string, h Hooks) []Outcome {
	out := make([]Outcome, 0, len(p.Items))
	for i, it := range p.Items {
		if ctx.Err() != nil {
			out = append(out, Outcome{Item: it, Result: "skipped", Error: "interrupted"})
			continue
		}
		if h.Start != nil {
			h.Start(it, i, len(p.Items))
		}
		o := applyItem(ctx, opID, it, roots, h)
		if h.Done != nil {
			h.Done(o)
		}
		out = append(out, o)
	}
	return out
}

func applyItem(ctx context.Context, opID string, it Item, roots []string, h Hooks) Outcome {
	o := Outcome{Item: it}
	failf := func(err error) Outcome {
		o.Result, o.Error = "failed", err.Error()
		return o
	}
	switch it.Action {
	case ActQuarantine:
		allowed := roots
		if it.Root != "" {
			allowed = append([]string{it.Root}, roots...)
		}
		if err := Guard(it.Path, allowed); err != nil {
			return failf(err)
		}
		if err := Revalidate(it.Path, it.FP); err != nil {
			o.Result, o.Error = "skipped", err.Error()
			return o
		}
		if _, err := quarantine.Move(opID, "cleanup", it.ID, it.Path, it.Bytes); err != nil {
			if errors.Is(err, quarantine.ErrCrossFS) {
				o.Result, o.Error = "skipped", "on another filesystem — can't quarantine atomically (use --no-undo to delete)"
				return o
			}
			return failf(err)
		}
		o.Result, o.Freed = "done", 0 // frees after retention
	case ActDelete:
		before := sizeOf(ctx, it.Paths)
		for _, p := range it.Paths {
			if err := Guard(p, []string{platform.Home()}); err != nil {
				return failf(err)
			}
			if err := deleteContents(p); err != nil {
				return failf(err)
			}
		}
		o.Result, o.Freed = "done", max(0, before-sizeOf(ctx, it.Paths))
	case ActExec, ActSudo:
		before := sizeOf(ctx, it.Paths)
		var err error
		if it.Action == ActSudo {
			if h.Sudo != nil && !h.Sudo(it.Argv) {
				o.Result, o.Error = "skipped", "declined"
				return o
			}
			_, err = platform.Sudo(ctx, it.Argv...)
		} else {
			var r platform.Result
			r, err = platform.Run(ctx, 10*time.Minute, it.Argv...)
			if err != nil && len(r.Stderr) > 0 {
				err = fmt.Errorf("%v: %s", err, lastLine(string(r.Stderr)))
			}
		}
		if err != nil {
			return failf(err)
		}
		o.Result = "done"
		if len(it.Paths) > 0 {
			o.Freed = max(0, before-sizeOf(ctx, it.Paths))
		} else {
			o.Freed = it.Bytes // tool-reported (docker, journald)
		}
	default:
		return failf(fmt.Errorf("unknown action %q", it.Action))
	}
	return o
}

// deleteContents empties a cache directory through os.Root, which refuses any
// path that would escape it (no symlink or ".." traversal).
func deleteContents(dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer root.Close()
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range ents {
		if err := root.RemoveAll(e.Name()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func sizeOf(ctx context.Context, paths []string) int64 {
	var n int64
	for _, p := range paths {
		if s, err := DirSize(ctx, p); err == nil {
			n += s.Bytes
		}
	}
	return n
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

// RemoveTrashedDir is used by nm reset to drop a failed tree into quarantine.
func RemoveTrashedDir(opID, path string) error {
	_, err := quarantine.Move(opID, "nm-reset", "failed:"+filepath.Base(path), path, 0)
	return err
}
