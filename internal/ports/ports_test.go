package ports

import (
	"context"
	"net"
	"os"
	"runtime"
	"testing"
)

const procNetTCP = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:0BB8 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 4242 1 0000000000000000 100 0 0 10 0
   1: 00000000:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 620815 1 0000000000000000 100 0 0 10 0
   2: 0100007F:0BB8 0100007F:D431 01 00000000:00000000 00:00000000 00000000  1000        0 0 1 0000000000000000 20 4 30 10 -1
`

const procNetTCP6 = `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000001000000:0BB9 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 5151 1 0000000000000000 100 0 0 10 0
`

func TestParseProcNetTCP(t *testing.T) {
	ls, err := parseProcNetTCP(procNetTCP, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) != 2 {
		t.Fatalf("want 2 LISTEN sockets (established row skipped), got %d", len(ls))
	}
	if ls[0].Port != 3000 || ls[0].Addr != "127.0.0.1" || ls[0].UID != 1000 || ls[0].Inode != 4242 {
		t.Errorf("row 0 = %+v", ls[0])
	}
	if ls[1].Port != 22 || ls[1].Addr != "*" || ls[1].UID != 0 {
		t.Errorf("row 1 = %+v", ls[1])
	}
	v6, err := parseProcNetTCP(procNetTCP6, true)
	if err != nil || len(v6) != 1 || v6[0].Port != 3001 || v6[0].Addr != "::1" || v6[0].Proto != "tcp6" {
		t.Fatalf("v6 = %+v %v", v6, err)
	}
}

func FuzzParseProcNetTCP(f *testing.F) {
	f.Add(procNetTCP)
	f.Add(procNetTCP6)
	f.Fuzz(func(t *testing.T, s string) {
		parseProcNetTCP(s, false) // must never panic
		parseProcNetTCP(s, true)
	})
}

const lsofOut = `p4123
cnode
u501
f23
tIPv4
n127.0.0.1:3000
f24
tIPv6
n[::1]:3000
p911
cpostgres
u70
f7
tIPv6
n*:5432
`

func TestParseLsofF(t *testing.T) {
	ls := dedupe(parseLsofF(lsofOut))
	if len(ls) != 2 {
		t.Fatalf("want 2 listeners after dedupe, got %+v", ls)
	}
	if ls[0].Port != 3000 || ls[0].PID != 4123 || ls[0].Command != "node" || ls[0].UID != 501 {
		t.Errorf("node row = %+v", ls[0])
	}
	if ls[1].Port != 5432 || ls[1].PID != 911 || ls[1].Proto != "tcp6" {
		t.Errorf("postgres row = %+v", ls[1])
	}
}

func TestParsePort(t *testing.T) {
	if p, err := ParsePort(":3000"); err != nil || p != 3000 {
		t.Errorf("':3000' → %d %v", p, err)
	}
	for _, bad := range []string{"0", "65536", "http", "-1"} {
		if _, err := ParsePort(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestAllFindsOwnListener(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip()
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("cannot listen")
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	ls, err := All(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := Filter(ls, []int{port})
	if len(got) != 1 || got[0].PID != os.Getpid() {
		t.Fatalf("own listener on %d not resolved: %+v", port, got)
	}
}
