// Package platform isolates everything OS-shaped: paths, subprocesses, sudo,
// locks and environment detection. Domain packages never branch on GOOS.
package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Paths are thine's XDG-style locations (honoured on macOS too, for dev-tool
// consistency; see plan §4.5).
type Paths struct {
	Home       string
	ConfigDir  string
	ConfigFile string
	StateDir   string
	DataDir    string
	CacheDir   string
	Quarantine string
	OpsLog     string
	TrustFile  string
}

// Home returns the user's home, overridable with THINE_HOME for tests.
func Home() string {
	if h := os.Getenv("THINE_HOME"); h != "" {
		return h
	}
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "/"
}

func xdg(env, fallback string) string {
	if os.Getenv("THINE_HOME") == "" {
		if v := os.Getenv(env); v != "" && filepath.IsAbs(v) {
			return v
		}
	}
	return filepath.Join(Home(), fallback)
}

// GetPaths resolves all locations from the environment.
func GetPaths() Paths {
	home := Home()
	cfgDir := filepath.Join(xdg("XDG_CONFIG_HOME", ".config"), "thine")
	cfgFile := filepath.Join(cfgDir, "config.toml")
	if v := os.Getenv("THINE_CONFIG"); v != "" {
		cfgFile = v
	}
	state := filepath.Join(xdg("XDG_STATE_HOME", ".local/state"), "thine")
	if v := os.Getenv("THINE_STATE_DIR"); v != "" {
		state = v
	}
	data := filepath.Join(xdg("XDG_DATA_HOME", ".local/share"), "thine")
	return Paths{
		Home:       home,
		ConfigDir:  cfgDir,
		ConfigFile: cfgFile,
		StateDir:   state,
		DataDir:    data,
		CacheDir:   filepath.Join(xdg("XDG_CACHE_HOME", ".cache"), "thine"),
		Quarantine: filepath.Join(data, "quarantine"),
		OpsLog:     filepath.Join(state, "ops.jsonl"),
		TrustFile:  filepath.Join(state, "trust.json"),
	}
}

// EnsurePrivateDir creates dir with 0700 (state and quarantine are private, T13).
func EnsurePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}

// ExpandHome turns "~/x" and "$VAR/x" into absolute paths.
func ExpandHome(p string) string {
	p = os.ExpandEnv(p)
	if p == "~" {
		return Home()
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(Home(), p[2:])
	}
	return p
}

// Tilde shortens a path under $HOME for display: /home/a/code → ~/code.
func Tilde(p string) string {
	h := Home()
	if p == h {
		return "~"
	}
	if strings.HasPrefix(p, h+string(filepath.Separator)) {
		return "~" + p[len(h):]
	}
	return p
}

// GOOS is the only sanctioned access to runtime.GOOS outside *_<os>.go files.
func GOOS() string { return runtime.GOOS }

// IsDarwin and IsLinux are small conveniences for presentation decisions.
func IsDarwin() bool { return runtime.GOOS == "darwin" }
func IsLinux() bool  { return runtime.GOOS == "linux" }
