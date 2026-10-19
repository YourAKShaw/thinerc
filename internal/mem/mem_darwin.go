//go:build darwin

package mem

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/YourAKShaw/thinerc/internal/platform"
)

// Read takes a snapshot from sysctl, vm_stat and memory_pressure.
func Read(ctx context.Context) (Snapshot, error) {
	s := Snapshot{Source: "vm_stat"}
	total, err := platform.Output(ctx, 2*time.Second, "sysctl", "-n", "hw.memsize")
	if err != nil {
		return s, err
	}
	s.Total, _ = strconv.ParseInt(total, 10, 64)
	if out, err := platform.Output(ctx, 2*time.Second, "vm_stat"); err == nil {
		pages, ps := parseVMStat(out)
		// Available ≈ free + inactive + speculative + purgeable (what macOS can hand out without swapping).
		s.Available = (pages["Pages free"] + pages["Pages inactive"] + pages["Pages speculative"] + pages["Pages purgeable"]) * ps
		s.Cache = (pages["File-backed pages"]) * ps
	}
	if out, err := platform.Output(ctx, 2*time.Second, "sysctl", "-n", "vm.swapusage"); err == nil {
		s.SwapTotal, s.SwapUsed = parseSwapUsage(out)
	}
	if out, err := platform.Output(ctx, 3*time.Second, "memory_pressure", "-Q"); err == nil {
		for _, line := range strings.Split(out, "\n") {
			if i := strings.Index(line, "free percentage:"); i >= 0 {
				v := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line[i+len("free percentage:"):]), "%"))
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					s.Pressure, s.HasPressure = 100-f, true
				}
			}
		}
	}
	return s, nil
}

// DropCachesArgv is the privileged command behind `freeup --drop-caches`.
func DropCachesArgv() []string { return []string{"purge"} }
