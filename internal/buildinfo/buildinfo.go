// Package buildinfo carries version metadata stamped in at link time.
package buildinfo

import (
	"runtime"
	"runtime/debug"
)

// Set with -ldflags "-X github.com/YourAKShaw/thinerc/internal/buildinfo.Version=…".
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Info is the machine-readable form printed by `thine version --json`.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"go_version"`
	Target    string `json:"target"`
}

// Get returns the build metadata, falling back to module info for `go install` builds.
func Get() Info {
	v := Version
	if v == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			v = bi.Main.Version
		}
	}
	return Info{
		Version:   v,
		Commit:    Commit,
		Date:      Date,
		GoVersion: runtime.Version(),
		Target:    runtime.GOOS + "/" + runtime.GOARCH,
	}
}
