//go:build linux

package platform

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"
)

// LoadAvg returns the 1, 5 and 15 minute load averages.
func LoadAvg(context.Context) [3]float64 {
	var out [3]float64
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return out
	}
	f := strings.Fields(string(b))
	for i := 0; i < 3 && i < len(f); i++ {
		out[i], _ = strconv.ParseFloat(f[i], 64)
	}
	return out
}

// Uptime is time since boot.
func Uptime(context.Context) time.Duration {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	s, _ := strconv.ParseFloat(strings.Fields(string(b))[0], 64)
	return time.Duration(s * float64(time.Second))
}
