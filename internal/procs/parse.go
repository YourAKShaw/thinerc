package procs

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// statFields is what thine needs from /proc/<pid>/stat.
type statFields struct {
	Comm      string
	State     byte
	PPID      int
	PGID      int
	StartTime uint64
	RSSPages  int64
}

// parseStat parses /proc/<pid>/stat. The comm field may contain spaces and
// parentheses, so it is delimited by the first "(" and the LAST ")".
func parseStat(s string) (statFields, error) {
	open := strings.IndexByte(s, '(')
	end := strings.LastIndexByte(s, ')')
	if open < 0 || end < open {
		return statFields{}, errors.New("malformed stat")
	}
	f := strings.Fields(s[end+1:])
	// f[0] is field 3 (state); field N is f[N-3].
	if len(f) < 22 {
		return statFields{}, errors.New("short stat")
	}
	ppid, _ := strconv.Atoi(f[1])
	pgid, _ := strconv.Atoi(f[2])
	start, err := strconv.ParseUint(f[19], 10, 64)
	if err != nil {
		return statFields{}, err
	}
	rss, _ := strconv.ParseInt(f[21], 10, 64)
	return statFields{Comm: s[open+1 : end], State: f[0][0], PPID: ppid, PGID: pgid, StartTime: start, RSSPages: rss}, nil
}

// parseCmdline splits NUL-separated /proc/<pid>/cmdline.
func parseCmdline(b []byte) []string {
	s := strings.TrimRight(string(b), "\x00")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\x00")
}

// psRow is one line of macOS `ps -o pid=,ppid=,pgid=,uid=,rss=,lstart=,comm=`.
// lstart is always five tokens ("Mon Oct 19 21:55:01 2026"); comm is the rest.
func parsePSRow(line string) (Proc, error) {
	f := strings.Fields(line)
	if len(f) < 11 {
		return Proc{}, errors.New("short ps row")
	}
	nums := make([]int64, 5)
	for i := 0; i < 5; i++ {
		v, err := strconv.ParseInt(f[i], 10, 64)
		if err != nil {
			return Proc{}, err
		}
		nums[i] = v
	}
	started, err := time.ParseInLocation("Mon Jan _2 15:04:05 2006", strings.Join(f[5:10], " "), time.Local)
	if err != nil {
		// Some locales pad the day differently; retry with a single space.
		started, err = time.ParseInLocation("Mon Jan 2 15:04:05 2006", strings.Join(f[5:10], " "), time.Local)
		if err != nil {
			return Proc{}, err
		}
	}
	exe := strings.Join(f[10:], " ")
	name := exe
	if i := strings.LastIndexByte(exe, '/'); i >= 0 {
		name = exe[i+1:]
	}
	return Proc{
		Identity: Identity{PID: int(nums[0]), StartTime: uint64(started.Unix())},
		PPID:     int(nums[1]), PGID: int(nums[2]), UID: int(nums[3]),
		RSS: nums[4] * 1024, Started: started, Name: name, Exe: exe,
	}, nil
}
