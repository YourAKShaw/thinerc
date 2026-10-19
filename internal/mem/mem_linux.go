//go:build linux

package mem

import (
	"context"
	"os"
)

// Read takes a memory snapshot from /proc.
func Read(_ context.Context) (Snapshot, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return Snapshot{}, err
	}
	s := snapshotFromMeminfo(parseMeminfo(string(data)))
	if psi, err := os.ReadFile("/proc/pressure/memory"); err == nil {
		s.Pressure, s.HasPressure = parsePSI(string(psi))
	}
	return s, nil
}

// DropCachesArgv is the privileged command behind `freeup --drop-caches`.
func DropCachesArgv() []string { return []string{"sysctl", "-w", "vm.drop_caches=1"} }
