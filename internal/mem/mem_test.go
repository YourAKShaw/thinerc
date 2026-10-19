package mem

import (
	"testing"

	"github.com/YourAKShaw/thinerc/internal/procs"
)

const meminfo = `MemTotal:        7492936 kB
MemFree:          312000 kB
MemAvailable:    1807476 kB
Buffers:           11956 kB
Cached:          1014232 kB
SwapTotal:       7747228 kB
SwapFree:        2075468 kB
Shmem:             15304 kB
SReclaimable:     156468 kB
`

func TestSnapshotFromMeminfo(t *testing.T) {
	s := snapshotFromMeminfo(parseMeminfo(meminfo))
	if s.Total != 7492936*1024 || s.Available != 1807476*1024 {
		t.Fatalf("total/available wrong: %+v", s)
	}
	if want := int64(1014232+11956+156468-15304) * 1024; s.Cache != want {
		t.Errorf("cache = %d, want %d", s.Cache, want)
	}
	if s.SwapUsed != (7747228-2075468)*1024 {
		t.Errorf("swap used = %d", s.SwapUsed)
	}
	if p := s.UsedPct(); p < 75 || p > 77 {
		t.Errorf("used pct = %.1f", p)
	}
}

func TestParsePSI(t *testing.T) {
	p, ok := parsePSI("some avg10=12.34 avg60=1.00 avg300=0.00 total=1\nfull avg10=1.00 avg60=0 avg300=0 total=1\n")
	if !ok || p != 12.34 {
		t.Errorf("psi = %v %v", p, ok)
	}
	if _, ok := parsePSI(""); ok {
		t.Error("empty PSI should be unknown")
	}
}

func TestParseVMStatAndSwap(t *testing.T) {
	out := `Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                               12345.
Pages active:                            100000.
Pages inactive:                           50000.
Pages speculative:                         1000.
Pages purgeable:                            500.
File-backed pages:                        60000.
`
	pages, ps := parseVMStat(out)
	if ps != 16384 || pages["Pages free"] != 12345 || pages["Pages purgeable"] != 500 {
		t.Fatalf("vm_stat parse: %v %d", pages, ps)
	}
	total, used := parseSwapUsage("total = 2048.00M  used = 1024.50M  free = 1023.50M  (encrypted)")
	if total != 2048<<20 || used != int64(1024.5*(1<<20)) {
		t.Errorf("swap = %d %d", total, used)
	}
}

func TestTopConsumersGroupsAndKinds(t *testing.T) {
	self := procs.Self()
	all := []procs.Proc{
		{Identity: procs.Identity{PID: 1}, Name: "chrome", UID: self, RSS: 300 << 20},
		{Identity: procs.Identity{PID: 2}, Name: "chrome", UID: self, RSS: 200 << 20},
		{Identity: procs.Identity{PID: 3}, Name: "node", UID: self, RSS: 400 << 20, Cmdline: []string{"node", "/x/typescript/lib/tsserver.js"}},
		{Identity: procs.Identity{PID: 4}, Name: "node", UID: self, RSS: 100 << 20, Cmdline: []string{"node", "server.js"}},
	}
	got := TopConsumers(all, 10, []string{"node"}, []string{"tsserver"})
	kinds := map[string]Consumer{}
	for _, c := range got {
		kinds[c.Name] = c
	}
	if c := kinds["chrome"]; c.Count != 2 || c.Kind != "browser" {
		t.Errorf("chrome group = %+v", c)
	}
	if c := kinds["tsserver"]; c.Kind != "language-server" {
		t.Errorf("tsserver = %+v", c)
	}
	if c := kinds["node"]; c.Kind != "dev" || c.Count != 1 {
		t.Errorf("node = %+v", c)
	}
}
