//go:build darwin

package platform

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// LoadAvg returns the 1, 5 and 15 minute load averages ("{ 1.23 1.10 0.98 }").
func LoadAvg(ctx context.Context) [3]float64 {
	var out [3]float64
	s, err := Output(ctx, time.Second, "sysctl", "-n", "vm.loadavg")
	if err != nil {
		return out
	}
	f := strings.Fields(strings.Trim(s, "{} "))
	for i := 0; i < 3 && i < len(f); i++ {
		out[i], _ = strconv.ParseFloat(f[i], 64)
	}
	return out
}

// Uptime is time since boot, from kern.boottime ("{ sec = 1760870000, usec = 0 } …").
func Uptime(ctx context.Context) time.Duration {
	s, err := Output(ctx, time.Second, "sysctl", "-n", "kern.boottime")
	if err != nil {
		return 0
	}
	if i := strings.Index(s, "sec = "); i >= 0 {
		rest := s[i+6:]
		if j := strings.IndexAny(rest, ", "); j > 0 {
			if sec, err := strconv.ParseInt(rest[:j], 10, 64); err == nil {
				return time.Since(time.Unix(sec, 0))
			}
		}
	}
	return 0
}
