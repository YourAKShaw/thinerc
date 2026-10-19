package cleanup

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/YourAKShaw/thinerc/internal/platform"
)

// Item is one planned action.
type Item struct {
	ID       string      `json:"id"`
	Category string      `json:"category"`
	Title    string      `json:"title"`
	Risk     string      `json:"risk"`
	Action   string      `json:"action"`
	Argv     []string    `json:"argv,omitempty"`
	Path     string      `json:"path,omitempty"`
	Paths    []string    `json:"paths,omitempty"` // sized paths for exec/delete actions
	Bytes    int64       `json:"bytes"`
	Shared   int64       `json:"shared_bytes,omitempty"` // hardlinked elsewhere; not freed
	IdleDays int         `json:"idle_days,omitempty"`
	Undoable bool        `json:"undoable"`
	FP       Fingerprint `json:"fingerprint"`
	Root     string      `json:"root,omitempty"`
	Note     string      `json:"note,omitempty"`
}

// Options control a scan.
type Options struct {
	Only, Skip, Include []string
	Roots               []string
	Exclude             []string
	StaleAfter          time.Duration
	Deadline            time.Duration
	Progress            func(category string, done, total int)
}

// Plan is the full proposal shown before anything happens.
type Plan struct {
	ID        string    `json:"plan_id"`
	CreatedAt time.Time `json:"created_at"`
	Items     []Item    `json:"items"`
	Partial   bool      `json:"partial"` // the soft deadline cut the scan short
	Skipped   []string  `json:"skipped,omitempty"`
}

// Totals is the honest accounting from plan §6.5.
type Totals struct {
	FreesNow       int64 `json:"frees_now"`
	AfterRetention int64 `json:"frees_after_retention"`
	NotReclaimable int64 `json:"not_reclaimable"`
}

// Totals sums a plan.
func (p Plan) Totals() Totals {
	var t Totals
	for _, it := range p.Items {
		t.NotReclaimable += it.Shared
		if it.Action == ActQuarantine {
			t.AfterRetention += it.Bytes
		} else {
			t.FreesNow += it.Bytes
		}
	}
	return t
}

// Scan builds a plan. Categories run concurrently (bounded); a soft deadline
// returns partial results rather than hanging on a pathological tree.
func Scan(ctx context.Context, o Options) Plan {
	if o.Deadline == 0 {
		o.Deadline = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, o.Deadline)
	defer cancel()
	var targets []Target
	for _, t := range Catalog() {
		if t.Selected(o.Only, o.Skip, o.Include) {
			targets = append(targets, t)
		}
	}
	var (
		mu      sync.Mutex
		items   []Item
		skipped []string
		done    int
		wg      sync.WaitGroup
		sem     = make(chan struct{}, 6)
	)
	for _, t := range targets {
		wg.Add(1)
		go func(t Target) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			got, why := scanTarget(ctx, t, o)
			mu.Lock()
			items = append(items, got...)
			if why != "" {
				skipped = append(skipped, t.ID+": "+why)
			}
			done++
			if o.Progress != nil {
				o.Progress(t.Title, done, len(targets))
			}
			mu.Unlock()
		}(t)
	}
	wg.Wait()
	sort.SliceStable(items, func(i, j int) bool {
		ri, rj := riskOrder(items[i].Risk), riskOrder(items[j].Risk)
		if ri != rj {
			return ri < rj
		}
		return items[i].Bytes > items[j].Bytes
	})
	sort.Strings(skipped)
	return Plan{CreatedAt: time.Now().UTC(), Items: items, Partial: ctx.Err() != nil, Skipped: skipped}
}

func riskOrder(r string) int {
	switch r {
	case Regenerable:
		return 0
	case Reversible:
		return 1
	}
	return 2
}

func scanTarget(ctx context.Context, t Target, o Options) ([]Item, string) {
	if t.Detect != "" && !platform.Have(t.Detect) {
		return nil, ""
	}
	switch t.Kind {
	case "artifact":
		return scanArtifacts(ctx, t, o), ""
	case "downloads":
		return scanDownloads(ctx, t, o), ""
	}
	it := Item{ID: t.ID, Category: t.ID, Title: t.Title, Risk: t.Risk, Action: t.Action, Argv: t.Argv}
	if t.SizeHint != nil {
		n := t.SizeHint(ctx)
		if n < 0 {
			return nil, "not running"
		}
		it.Bytes = n
	}
	if t.Paths != nil {
		for _, p := range t.Paths(ctx) {
			if _, err := os.Lstat(p); err != nil {
				continue
			}
			it.Paths = append(it.Paths, p)
			if t.SizeHint == nil {
				sz, _ := DirSize(ctx, p)
				it.Bytes += sz.Bytes
			}
		}
		if len(it.Paths) == 0 && t.SizeHint == nil {
			return nil, ""
		}
	}
	if it.Bytes < 1<<20 { // under 1 MiB is noise
		return nil, ""
	}
	if t.Action == ActDelete {
		for _, p := range it.Paths {
			if err := Guard(p, []string{platform.Home()}); err != nil {
				return nil, err.Error()
			}
		}
		it.FP, _ = FingerprintOf(it.Paths[0])
	}
	return []Item{it}, ""
}

// Size is allocated bytes, hardlink-deduplicated (pnpm/bun stores hardlink
// into node_modules; naive sizing overstates savings by up to 100%).
type Size struct {
	Bytes  int64 // freed if removed
	Shared int64 // hardlinks with links outside the tree
}

// DirSize walks root without following symlinks or crossing filesystems.
func DirSize(ctx context.Context, root string) (Size, error) {
	var s Size
	rootFI, err := os.Lstat(root)
	if err != nil {
		return s, err
	}
	rootDev := uint64(rootFI.Sys().(*syscall.Stat_t).Dev)
	type key struct{ dev, ino uint64 }
	seen := map[key]bool{}
	linksInside := map[key]uint64{}
	nlink := map[key]uint64{}
	blocks := map[key]int64{}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries are skipped, not fatal
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			return nil
		}
		if d.IsDir() && uint64(st.Dev) != rootDev {
			return fs.SkipDir
		}
		k := key{uint64(st.Dev), uint64(st.Ino)}
		alloc := int64(st.Blocks) * 512
		if uint64(st.Nlink) > 1 && !d.IsDir() {
			linksInside[k]++
			nlink[k] = uint64(st.Nlink)
			blocks[k] = alloc
			return nil
		}
		if !seen[k] {
			seen[k] = true
			s.Bytes += alloc
		}
		return nil
	})
	for k, inside := range linksInside {
		if inside >= nlink[k] {
			s.Bytes += blocks[k]
		} else {
			s.Shared += blocks[k]
		}
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return s, err
	}
	return s, nil
}

func scanDownloads(ctx context.Context, t Target, o Options) []Item {
	var out []Item
	cutoff := time.Now().Add(-o.StaleAfter)
	for _, dir := range t.Paths(ctx) {
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if !slices.Contains(installerExts, ext) || !e.Type().IsRegular() {
				continue
			}
			fi, err := e.Info()
			if err != nil || fi.ModTime().After(cutoff) {
				continue
			}
			p := filepath.Join(dir, e.Name())
			if Guard(p, []string{dir}) != nil || excluded(p, o.Exclude) {
				continue
			}
			st := fi.Sys().(*syscall.Stat_t)
			fp, _ := FingerprintOf(p)
			out = append(out, Item{
				ID: "downloads:" + p, Category: t.ID, Title: e.Name(), Risk: t.Risk, Action: ActQuarantine, Path: p,
				Bytes: int64(st.Blocks) * 512, IdleDays: int(time.Since(fi.ModTime()).Hours() / 24), Undoable: true, FP: fp, Root: dir,
			})
		}
	}
	return out
}

func excluded(p string, globs []string) bool {
	for _, g := range globs {
		g = platform.ExpandHome(g)
		if ok, _ := filepath.Match(g, p); ok {
			return true
		}
		if base, ok := strings.CutSuffix(g, "/**"); ok && within(p, base) {
			return true
		}
	}
	return false
}

// scanArtifacts finds stale build directories (node_modules, .next, target…)
// under the configured roots. A directory qualifies only when a marker file
// sits beside it, the project has been idle longer than stale_after, and git
// tracks nothing inside it (borrowed from kondo).
func scanArtifacts(ctx context.Context, t Target, o Options) []Item {
	var out []Item
	cutoff := time.Now().Add(-o.StaleAfter)
	for _, root := range o.Roots {
		root = platform.ExpandHome(root)
		if _, err := os.Stat(root); err != nil {
			continue
		}
		rootDepth := strings.Count(filepath.Clean(root), string(filepath.Separator))
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() || ctx.Err() != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return nil
			}
			name := d.Name()
			depth := strings.Count(p, string(filepath.Separator)) - rootDepth
			if p != root && (name == ".git" || (strings.HasPrefix(name, ".") && name != t.Dir) || depth > 7) {
				return fs.SkipDir
			}
			if name == "node_modules" && name != t.Dir {
				return fs.SkipDir // never look inside dependencies
			}
			if name != t.Dir || p == root {
				return nil
			}
			proj := filepath.Dir(p)
			if !hasMarker(proj, t.Markers) || excluded(p, o.Exclude) {
				return fs.SkipDir
			}
			last := projectActivity(proj)
			if last.After(cutoff) {
				return fs.SkipDir
			}
			if gitTracksInside(ctx, proj, name) || Guard(p, o.Roots) != nil {
				return fs.SkipDir
			}
			sz, _ := DirSize(ctx, p)
			if sz.Bytes < 1<<20 {
				return fs.SkipDir
			}
			fp, _ := FingerprintOf(p)
			out = append(out, Item{
				ID: t.ID + ":" + p, Category: t.ID, Title: platform.Tilde(proj), Risk: t.Risk, Action: ActQuarantine, Path: p,
				Bytes: sz.Bytes, Shared: sz.Shared, IdleDays: int(time.Since(last).Hours() / 24), Undoable: true, FP: fp, Root: root,
			})
			return fs.SkipDir
		})
	}
	return out
}

func hasMarker(dir string, markers []string) bool {
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}
	return len(markers) == 0
}

// projectActivity approximates "last touched": the newest mtime among the
// project's top-level entries (artifact dirs excluded) and its git index.
func projectActivity(proj string) time.Time {
	var newest time.Time
	skip := map[string]bool{"node_modules": true, ".next": true, "target": true, ".venv": true, "dist": true, "build": true}
	ents, _ := os.ReadDir(proj)
	for _, e := range ents {
		if skip[e.Name()] {
			continue
		}
		if fi, err := e.Info(); err == nil && fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
	}
	for _, g := range []string{".git/index", ".git/HEAD", ".git/FETCH_HEAD"} {
		if fi, err := os.Stat(filepath.Join(proj, g)); err == nil && fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
	}
	return newest
}

// gitTracksInside is true if git tracks any file in proj/dir (2 s budget).
func gitTracksInside(ctx context.Context, proj, dir string) bool {
	if !platform.Have("git") {
		return false
	}
	r, err := platform.Run(ctx, 2*time.Second, "git", "-C", proj, "ls-files", "-z", "--", dir)
	if err != nil {
		return false // not a repo, or unreadable: rely on the other rules
	}
	return len(r.Stdout) > 0
}
