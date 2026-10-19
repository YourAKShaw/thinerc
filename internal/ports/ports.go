// Package ports answers "who is listening on this TCP port?" — from /proc on
// Linux (no subprocess) and from one machine-readable lsof call on macOS.
package ports

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/YourAKShaw/thinerc/internal/platform"
)

// Listener is one listening TCP socket.
type Listener struct {
	Port    int    `json:"port"`
	Proto   string `json:"proto"` // tcp4 | tcp6
	Addr    string `json:"addr"`
	PID     int    `json:"pid"` // 0 when owned by another user and not visible
	UID     int    `json:"uid"`
	Command string `json:"command,omitempty"`
	Inode   uint64 `json:"-"`
	// External is set when the socket is held on behalf of something else
	// (a Docker container); killing the holder would be wrong.
	External *External `json:"external,omitempty"`
}

// External describes a port held for a container.
type External struct {
	Kind      string `json:"kind"` // docker
	Container string `json:"container"`
	ID        string `json:"id"`
}

// proxyNames hold ports on behalf of containers or VMs (plan §6.4).
var proxyNames = []string{"docker-proxy", "com.docker.backend", "rootlessport", "vpnkit", "gvproxy", "wslrelay"}

// IsProxy reports whether a holder process is a container port proxy.
func IsProxy(name string) bool {
	for _, n := range proxyNames {
		if strings.EqualFold(name, n) {
			return true
		}
	}
	return false
}

// Filter keeps listeners on the given ports (all when ports is empty).
func Filter(ls []Listener, ports []int) []Listener {
	if len(ports) == 0 {
		return ls
	}
	want := map[int]bool{}
	for _, p := range ports {
		want[p] = true
	}
	var out []Listener
	for _, l := range ls {
		if want[l.Port] {
			out = append(out, l)
		}
	}
	return out
}

// dedupe collapses the tcp4/tcp6 twins of one listener, preferring the PID-known row.
func dedupe(ls []Listener) []Listener {
	type key struct{ port, pid int }
	seen := map[key]int{}
	var out []Listener
	for _, l := range ls {
		k := key{l.Port, l.PID}
		if i, ok := seen[k]; ok {
			if out[i].Proto == "tcp6" && l.Proto == "tcp4" {
				out[i].Addr = l.Addr + ", " + out[i].Addr
			}
			continue
		}
		seen[k] = len(out)
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Port != out[j].Port {
			return out[i].Port < out[j].Port
		}
		return out[i].PID < out[j].PID
	})
	return out
}

// parseProcNetTCP parses /proc/net/tcp or tcp6 and returns LISTEN sockets.
func parseProcNetTCP(data string, v6 bool) ([]Listener, error) {
	var out []Listener
	sc := bufio.NewScanner(strings.NewReader(data))
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		f := strings.Fields(sc.Text())
		if len(f) < 10 || f[3] != "0A" { // 0A = TCP_LISTEN
			continue
		}
		hostHex, portHex, ok := strings.Cut(f[1], ":")
		if !ok {
			continue
		}
		port, err := strconv.ParseUint(portHex, 16, 16)
		if err != nil {
			return nil, fmt.Errorf("bad port %q", portHex)
		}
		ip, err := decodeAddr(hostHex)
		if err != nil {
			return nil, err
		}
		uid, _ := strconv.Atoi(f[7])
		inode, _ := strconv.ParseUint(f[9], 10, 64)
		proto := "tcp4"
		if v6 {
			proto = "tcp6"
		}
		out = append(out, Listener{Port: int(port), Proto: proto, Addr: ip, UID: uid, Inode: inode})
	}
	return out, sc.Err()
}

// decodeAddr decodes the kernel's hex address: 32-bit words in host (little-endian) order.
func decodeAddr(h string) (string, error) {
	b, err := hex.DecodeString(h)
	if err != nil || (len(b) != 4 && len(b) != 16) {
		return "", fmt.Errorf("bad address %q", h)
	}
	for i := 0; i < len(b); i += 4 {
		b[i], b[i+1], b[i+2], b[i+3] = b[i+3], b[i+2], b[i+1], b[i]
	}
	ip := net.IP(b)
	if ip.IsUnspecified() {
		return "*", nil
	}
	return ip.String(), nil
}

// parseLsofF parses `lsof -nP -iTCP -sTCP:LISTEN -F pcutn` output.
func parseLsofF(out string) []Listener {
	var res []Listener
	var pid, uid int
	var cmd, typ string
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		v := line[1:]
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(v)
			cmd, typ = "", ""
		case 'c':
			cmd = v
		case 'u':
			uid, _ = strconv.Atoi(v)
		case 'f':
			typ = ""
		case 't':
			typ = v
		case 'n':
			i := strings.LastIndexByte(v, ':')
			if i < 0 {
				continue
			}
			port, err := strconv.Atoi(v[i+1:])
			if err != nil {
				continue
			}
			addr := strings.Trim(v[:i], "[]")
			proto := "tcp4"
			if typ == "IPv6" || strings.Contains(addr, ":") {
				proto = "tcp6"
			}
			res = append(res, Listener{Port: port, Proto: proto, Addr: addr, PID: pid, UID: uid, Command: cmd})
		}
	}
	return res
}

// DockerContainerFor maps a published port to its container with a 1 s budget.
func DockerContainerFor(ctx context.Context, port int) *External {
	if !platform.Have("docker") {
		return nil
	}
	out, err := platform.Output(ctx, time.Second, "docker", "ps", "--filter", "publish="+strconv.Itoa(port), "--format", "{{.ID}}\t{{.Names}}")
	if err != nil || out == "" {
		return nil
	}
	id, name, _ := strings.Cut(strings.Split(out, "\n")[0], "\t")
	return &External{Kind: "docker", ID: id, Container: name}
}

// ErrUnsupported is returned on platforms without a resolver.
var ErrUnsupported = errors.New("port resolution is not supported on this OS yet")

// ParsePort validates a TCP port argument.
func ParsePort(s string) (int, error) {
	s = strings.TrimPrefix(s, ":")
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 {
		return 0, fmt.Errorf("%q is not a TCP port (1–65535)", s)
	}
	return n, nil
}
