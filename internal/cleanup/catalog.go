// Package cleanup plans and applies disk reclamation from a data-driven
// catalog. Caches are cleaned by their owning tool's own command; user files
// are quarantined (undoable); nothing outside an allowlisted root is touched.
package cleanup

import (
	"context"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/YourAKShaw/thinerc/internal/platform"
)

// Risk levels (plan §6.2).
const (
	Regenerable = "regenerable" // not undoable, but rebuilt automatically
	Reversible  = "reversible"  // quarantined, restorable with `thine undo`
	Destructive = "destructive" // neither — typed confirmation
)

// Action kinds.
const (
	ActExec       = "exec"       // run the owning tool's command
	ActSudo       = "sudo"       // run a registered privileged command
	ActDelete     = "delete"     // remove the contents of a cache directory
	ActQuarantine = "quarantine" // move to quarantine
)

// Target is one catalog category.
type Target struct {
	ID       string
	Title    string
	OS       []string // empty = all
	Risk     string
	Default  bool // included without --include
	Detect   string
	Action   string
	Argv     []string
	Paths    func(ctx context.Context) []string // what to size (and delete, for ActDelete)
	Kind     string                             // cache | artifact | downloads | system
	Dir      string                             // artifact directory name
	Markers  []string                           // sibling files that mark a real project
	MinAge   bool                               // apply cleanup.stale_after
	Group    string                             // --include group name ("system")
	SizeHint func(ctx context.Context) int64    // tools that report their own size
}

func home(p ...string) string { return filepath.Join(append([]string{platform.Home()}, p...)...) }

func static(paths ...string) func(context.Context) []string {
	return func(context.Context) []string { return paths }
}

// cacheDir returns the per-OS user cache base.
func cacheDir(sub ...string) string {
	if platform.IsDarwin() {
		return filepath.Join(append([]string{platform.Home(), "Library", "Caches"}, sub...)...)
	}
	return filepath.Join(append([]string{platform.Home(), ".cache"}, sub...)...)
}

// toolPath asks a tool where its cache lives (2 s budget).
func toolPath(argv ...string) func(context.Context) []string {
	return func(ctx context.Context) []string {
		out, err := platform.Output(ctx, 2*time.Second, argv...)
		if err != nil || out == "" || !filepath.IsAbs(strings.TrimSpace(out)) {
			return nil
		}
		return []string{strings.TrimSpace(strings.Split(out, "\n")[0])}
	}
}

// Catalog is the built-in list of categories (plan Appendix C).
func Catalog() []Target {
	return []Target{
		{ID: "npm", Title: "npm cache", Risk: Regenerable, Default: true, Detect: "npm", Action: ActExec,
			Argv: []string{"npm", "cache", "clean", "--force"}, Paths: static(home(".npm", "_cacache")), Kind: "cache"},
		{ID: "pnpm", Title: "pnpm store (unreferenced)", Risk: Regenerable, Default: true, Detect: "pnpm", Action: ActExec,
			Argv: []string{"pnpm", "store", "prune"}, Paths: toolPath("pnpm", "store", "path"), Kind: "cache"},
		{ID: "yarn", Title: "Yarn cache", Risk: Regenerable, Default: true, Detect: "yarn", Action: ActExec,
			Argv: []string{"yarn", "cache", "clean"}, Paths: toolPath("yarn", "cache", "dir"), Kind: "cache"},
		{ID: "bun", Title: "Bun install cache", Risk: Regenerable, Default: true, Detect: "bun", Action: ActExec,
			Argv: []string{"bun", "pm", "cache", "rm"}, Paths: static(home(".bun", "install", "cache")), Kind: "cache"},
		{ID: "pip", Title: "pip cache", Risk: Regenerable, Default: true, Detect: "python3", Action: ActExec,
			Argv: []string{"python3", "-m", "pip", "cache", "purge"}, Paths: static(cacheDir("pip")), Kind: "cache"},
		{ID: "uv", Title: "uv cache", Risk: Regenerable, Default: true, Detect: "uv", Action: ActExec,
			Argv: []string{"uv", "cache", "prune"}, Paths: static(cacheDir("uv")), Kind: "cache"},
		{ID: "go-build", Title: "Go build cache", Risk: Regenerable, Default: true, Detect: "go", Action: ActExec,
			Argv: []string{"go", "clean", "-cache"}, Paths: toolPath("go", "env", "GOCACHE"), Kind: "cache"},
		{ID: "go-modcache", Title: "Go module cache (re-downloads)", Risk: Regenerable, Default: false, Detect: "go", Action: ActExec,
			Argv: []string{"go", "clean", "-modcache"}, Paths: toolPath("go", "env", "GOMODCACHE"), Kind: "cache"},
		{ID: "docker-build", Title: "Docker build cache", Risk: Regenerable, Default: true, Detect: "docker", Action: ActExec,
			Argv: []string{"docker", "builder", "prune", "-f"}, Kind: "cache", SizeHint: dockerReclaimable("Build Cache")},
		{ID: "docker-images", Title: "Docker dangling images", Risk: Regenerable, Default: true, Detect: "docker", Action: ActExec,
			Argv: []string{"docker", "image", "prune", "-f"}, Kind: "cache", SizeHint: dockerReclaimable("Images")},
		{ID: "homebrew", Title: "Homebrew old versions & downloads", Risk: Regenerable, Default: true, Detect: "brew", Action: ActExec,
			Argv: []string{"brew", "cleanup", "--prune=all"}, Paths: toolPath("brew", "--cache"), Kind: "cache"},
		{ID: "thumbnails", Title: "Thumbnail cache", OS: []string{"linux"}, Risk: Regenerable, Default: true, Action: ActDelete,
			Paths: static(home(".cache", "thumbnails")), Kind: "cache"},
		{ID: "xcode-derived", Title: "Xcode DerivedData", OS: []string{"darwin"}, Risk: Regenerable, Default: true, Action: ActDelete,
			Paths: static(home("Library", "Developer", "Xcode", "DerivedData")), Kind: "cache"},
		{ID: "playwright-browsers", Title: "Playwright browsers", Risk: Regenerable, Default: false, Action: ActDelete,
			Paths: static(cacheDir("ms-playwright")), Kind: "cache"},
		{ID: "downloads-installers", Title: "Old installers in Downloads", Risk: Reversible, Default: true, Action: ActQuarantine,
			Paths: static(home("Downloads")), Kind: "downloads", MinAge: true},
		{ID: "node-modules", Title: "Stale node_modules", Risk: Reversible, Default: true, Action: ActQuarantine,
			Kind: "artifact", Dir: "node_modules", Markers: []string{"package.json"}, MinAge: true},
		{ID: "next-build", Title: "Stale Next.js .next builds", Risk: Regenerable, Default: true, Action: ActQuarantine,
			Kind: "artifact", Dir: ".next", Markers: []string{"next.config.js", "next.config.mjs", "next.config.ts"}, MinAge: true},
		{ID: "rust-target", Title: "Stale Rust target/ dirs", Risk: Regenerable, Default: true, Action: ActQuarantine,
			Kind: "artifact", Dir: "target", Markers: []string{"Cargo.toml"}, MinAge: true},
		{ID: "python-venv", Title: "Stale .venv dirs", Risk: Reversible, Default: false, Action: ActQuarantine,
			Kind: "artifact", Dir: ".venv", Markers: []string{"pyproject.toml", "requirements.txt"}, MinAge: true},
		{ID: "apt", Title: "APT package cache", OS: []string{"linux"}, Risk: Regenerable, Group: "system", Detect: "apt-get", Action: ActSudo,
			Argv: []string{"apt-get", "clean"}, Paths: static("/var/cache/apt/archives"), Kind: "system"},
		{ID: "journald", Title: "systemd journal (keep 7 days)", OS: []string{"linux"}, Risk: Regenerable, Group: "system", Detect: "journalctl", Action: ActSudo,
			Argv: []string{"journalctl", "--vacuum-time=7d"}, Kind: "system", SizeHint: journalSize},
		{ID: "dnf", Title: "DNF package cache", OS: []string{"linux"}, Risk: Regenerable, Group: "system", Detect: "dnf", Action: ActSudo,
			Argv: []string{"dnf", "clean", "packages"}, Paths: static("/var/cache/dnf"), Kind: "system"},
	}
}

// installerExts are the Downloads files considered installer leftovers. .zip
// is excluded on purpose: it is often a document bundle.
var installerExts = []string{".dmg", ".pkg", ".deb", ".rpm", ".appimage", ".exe", ".msi", ".iso"}

// Applies reports whether a target runs on this OS.
func (t Target) Applies() bool { return len(t.OS) == 0 || slices.Contains(t.OS, platform.GOOS()) }

// Selected decides inclusion from --only / --skip / --include.
func (t Target) Selected(only, skip, include []string) bool {
	if !t.Applies() || slices.Contains(skip, t.ID) {
		return false
	}
	if len(only) > 0 {
		return slices.Contains(only, t.ID) || (t.Group != "" && slices.Contains(only, t.Group))
	}
	if slices.Contains(include, t.ID) || (t.Group != "" && slices.Contains(include, t.Group)) {
		return true
	}
	return t.Default && t.Group == ""
}

func dockerReclaimable(typ string) func(context.Context) int64 {
	return func(ctx context.Context) int64 {
		out, err := platform.Output(ctx, 2*time.Second, "docker", "system", "df", "--format", "{{.Type}}\t{{.Reclaimable}}")
		if err != nil {
			return -1 // daemon down
		}
		for _, line := range strings.Split(out, "\n") {
			k, v, ok := strings.Cut(line, "\t")
			if ok && k == typ {
				return ParseHumanSize(strings.Fields(v)[0])
			}
		}
		return 0
	}
}

func journalSize(ctx context.Context) int64 {
	out, err := platform.Output(ctx, 2*time.Second, "journalctl", "--disk-usage")
	if err != nil {
		return 0
	}
	// "Archived and active journals take up 101.8M in the file system."
	for _, f := range strings.Fields(out) {
		if n := ParseHumanSize(f); n > 0 {
			return n
		}
	}
	return 0
}

// ParseHumanSize parses Docker/journald sizes: "2.1GB", "512MB", "101.8M", "0B".
func ParseHumanSize(s string) int64 {
	s = strings.TrimSpace(s)
	units := []struct {
		suf  string
		mult float64
	}{{"TB", 1e12}, {"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3}, {"KB", 1e3}, {"T", 1 << 40}, {"G", 1 << 30}, {"M", 1 << 20}, {"K", 1 << 10}, {"B", 1}}
	for _, u := range units {
		if v, ok := strings.CutSuffix(s, u.suf); ok {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return 0
			}
			return int64(f * u.mult)
		}
	}
	return 0
}
