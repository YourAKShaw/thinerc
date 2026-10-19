// Package mem reports memory honestly: *available* memory and pressure, not
// the "free" number that cache-dropping scripts inflate (ADR-0005).
package mem

import (
	"bufio"
	"sort"
	"strconv"
	"strings"

	"github.com/YourAKShaw/thinerc/internal/procs"
)

// Snapshot is a point-in-time memory picture.
type Snapshot struct {
	Total       int64   `json:"total_bytes"`
	Available   int64   `json:"available_bytes"`
	Cache       int64   `json:"cache_bytes"` // reclaimable page cache the kernel gives back on demand
	SwapTotal   int64   `json:"swap_total_bytes"`
	SwapUsed    int64   `json:"swap_used_bytes"`
	Pressure    float64 `json:"pressure_pct"` // Linux PSI some avg10, or 100 - macOS free %
	HasPressure bool    `json:"pressure_known"`
	Source      string  `json:"source"`
}

// UsedPct is the share of memory not available.
func (s Snapshot) UsedPct() float64 {
	if s.Total == 0 {
		return 0
	}
	return 100 * float64(s.Total-s.Available) / float64(s.Total)
}

// parseMeminfo reads /proc/meminfo-style "Key:  123 kB" lines into bytes.
func parseMeminfo(data string) map[string]int64 {
	out := map[string]int64{}
	sc := bufio.NewScanner(strings.NewReader(data))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		f := strings.Fields(v)
		if len(f) == 0 {
			continue
		}
		n, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil {
			continue
		}
		if len(f) > 1 && f[1] == "kB" {
			n *= 1024
		}
		out[k] = n
	}
	return out
}

// snapshotFromMeminfo builds a Snapshot from parsed /proc/meminfo.
func snapshotFromMeminfo(m map[string]int64) Snapshot {
	s := Snapshot{
		Total:     m["MemTotal"],
		Available: m["MemAvailable"],
		Cache:     m["Cached"] + m["Buffers"] + m["SReclaimable"] - m["Shmem"],
		SwapTotal: m["SwapTotal"],
		SwapUsed:  m["SwapTotal"] - m["SwapFree"],
		Source:    "/proc/meminfo",
	}
	if s.Available == 0 { // kernels before 3.14
		s.Available = m["MemFree"] + s.Cache
	}
	if s.Cache < 0 {
		s.Cache = 0
	}
	return s
}

// parsePSI returns "some avg10" from /proc/pressure/memory.
func parsePSI(data string) (float64, bool) {
	for _, line := range strings.Split(data, "\n") {
		if !strings.HasPrefix(line, "some ") {
			continue
		}
		for _, f := range strings.Fields(line) {
			if v, ok := strings.CutPrefix(f, "avg10="); ok {
				p, err := strconv.ParseFloat(v, 64)
				return p, err == nil
			}
		}
	}
	return 0, false
}

// parseVMStat parses macOS vm_stat output into page counts and page size.
func parseVMStat(data string) (map[string]int64, int64) {
	pages := map[string]int64{}
	pageSize := int64(4096)
	for _, line := range strings.Split(data, "\n") {
		if strings.Contains(line, "page size of") {
			for _, f := range strings.Fields(line) {
				if n, err := strconv.ParseInt(f, 10, 64); err == nil {
					pageSize = n
				}
			}
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimSpace(v), "."), 10, 64)
		if err == nil {
			pages[strings.TrimSpace(k)] = n
		}
	}
	return pages, pageSize
}

// parseSwapUsage parses `sysctl -n vm.swapusage`: "total = 2048.00M  used = 1024.00M  free = …".
func parseSwapUsage(s string) (total, used int64) {
	f := strings.Fields(s)
	for i := 0; i+2 < len(f); i++ {
		if f[i+1] != "=" {
			continue
		}
		v := parseSizeSuffix(f[i+2])
		switch f[i] {
		case "total":
			total = v
		case "used":
			used = v
		}
	}
	return
}

func parseSizeSuffix(s string) int64 {
	if s == "" {
		return 0
	}
	mult := map[byte]float64{'K': 1 << 10, 'M': 1 << 20, 'G': 1 << 30, 'T': 1 << 40}
	m, ok := mult[s[len(s)-1]]
	if ok {
		s = s[:len(s)-1]
	} else {
		m = 1
	}
	v, _ := strconv.ParseFloat(s, 64)
	return int64(v * m)
}

// Consumer is an application's total memory, grouped by name.
type Consumer struct {
	Name    string       `json:"name"`
	Bytes   int64        `json:"bytes"`
	Count   int          `json:"count"`
	Exact   bool         `json:"pss"` // PSS (no double counting) rather than RSS
	Procs   []procs.Proc `json:"-"`
	Kind    string       `json:"kind"` // dev | language-server | browser | ide | docker | other
	Own     bool         `json:"own"`
	Example string       `json:"example,omitempty"`
}

var (
	browsers = []string{"chrome", "chromium", "firefox", "brave", "msedge", "safari", "opera", "vivaldi", "google chrome", "zen"}
	ides     = []string{"code", "cursor", "idea", "webstorm", "pycharm", "goland", "zed", "sublime_text", "windsurf", "electron"}
	dockers  = []string{"dockerd", "containerd", "com.docker.backend", "qemu-system-aarch64", "qemu-system-x86_64", "vpnkit", "docker"}
)

func kindOf(names []string, dev, ls []string) string {
	has := func(list []string) bool {
		for _, n := range names {
			for _, l := range list {
				if n == l || strings.HasPrefix(n, l+"-") || strings.HasPrefix(n, l+" ") {
					return true
				}
			}
		}
		return false
	}
	switch {
	case has(ls):
		return "language-server"
	case has(browsers):
		return "browser"
	case has(ides):
		return "ide"
	case has(dockers):
		return "docker"
	case has(dev):
		return "dev"
	}
	return "other"
}

// TopConsumers groups processes by application and returns the n largest.
// On Linux, the user's own top processes are refined to PSS.
func TopConsumers(all []procs.Proc, n int, devNames, langServers []string) []Consumer {
	self := procs.Self()
	sort.Slice(all, func(i, j int) bool { return all[i].RSS > all[j].RSS })
	for i := range all {
		if i < 40 && all[i].UID == self {
			if pss := procs.PSS(all[i].PID); pss > 0 {
				all[i].PSS = pss
			}
		}
	}
	groups := map[string]*Consumer{}
	for _, p := range all {
		if p.RSS == 0 {
			continue
		}
		names := p.MatchNames()
		kind := kindOf(names, devNames, langServers)
		key := strings.ToLower(p.Name)
		// tsserver runs as `node …/tsserver.js`: name it by its script.
		for _, a := range p.Cmdline {
			for _, l := range langServers {
				if strings.Contains(a, l) {
					key, kind = l, "language-server"
				}
			}
		}
		if kind == "browser" || kind == "ide" {
			key = kindKey(names, kind)
		}
		g, ok := groups[key]
		if !ok {
			g = &Consumer{Name: key, Kind: kind, Exact: true, Own: true}
			groups[key] = g
		}
		b := p.RSS
		if p.PSS > 0 {
			b = p.PSS
		} else {
			g.Exact = false
		}
		g.Bytes += b
		g.Count++
		g.Procs = append(g.Procs, p)
		g.Own = g.Own && p.UID == self
		if g.Example == "" && len(p.Cmdline) > 0 {
			g.Example = strings.Join(p.Cmdline, " ")
		}
	}
	out := make([]Consumer, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Bytes > out[j].Bytes })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func kindKey(names []string, kind string) string {
	list := browsers
	if kind == "ide" {
		list = ides
	}
	for _, n := range names {
		for _, l := range list {
			if strings.HasPrefix(n, l) {
				return l
			}
		}
	}
	return names[0]
}
