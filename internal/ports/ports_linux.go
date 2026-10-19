//go:build linux

package ports

import (
	"context"
	"os"
	"strconv"
	"strings"
)

// All returns every listening TCP socket. PIDs are found by matching socket
// inodes in /proc/<pid>/fd, which only works for processes we may inspect;
// other users' sockets keep PID 0 but still report their UID.
func All(ctx context.Context) ([]Listener, error) {
	var ls []Listener
	for _, f := range []struct {
		path string
		v6   bool
	}{{"/proc/net/tcp", false}, {"/proc/net/tcp6", true}} {
		data, err := os.ReadFile(f.path)
		if err != nil {
			if f.v6 {
				continue // IPv6 disabled
			}
			return nil, err
		}
		got, err := parseProcNetTCP(string(data), f.v6)
		if err != nil {
			return nil, err
		}
		ls = append(ls, got...)
	}
	owners := socketOwners(ctx, ls)
	for i := range ls {
		if pid, ok := owners[ls[i].Inode]; ok {
			ls[i].PID = pid
			if b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm"); err == nil {
				ls[i].Command = strings.TrimSpace(string(b))
			}
		}
	}
	return dedupe(ls), nil
}

func socketOwners(ctx context.Context, ls []Listener) map[uint64]int {
	want := make(map[string]uint64, len(ls))
	for _, l := range ls {
		if l.Inode != 0 {
			want["socket:["+strconv.FormatUint(l.Inode, 10)+"]"] = l.Inode
		}
	}
	owners := map[uint64]int{}
	if len(want) == 0 {
		return owners
	}
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return owners
	}
	for _, e := range ents {
		if ctx.Err() != nil || len(owners) == len(want) {
			break
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		fdDir := "/proc/" + e.Name() + "/fd"
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue // not ours
		}
		for _, fd := range fds {
			link, err := os.Readlink(fdDir + "/" + fd.Name())
			if err != nil {
				continue
			}
			if inode, ok := want[link]; ok {
				if _, dup := owners[inode]; !dup {
					owners[inode] = pid
				}
			}
		}
	}
	return owners
}
