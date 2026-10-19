package platform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ErrNotFound means the executable is not on PATH.
var ErrNotFound = errors.New("command not found")

// Have reports whether an executable is on PATH.
func Have(name string) bool { _, err := exec.LookPath(name); return err == nil }

// Which returns the absolute path of an executable, or "".
func Which(name string) string {
	p, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return p
}

// Result is the captured outcome of a subprocess.
type Result struct {
	Stdout, Stderr []byte
	ExitCode       int
}

// Run executes argv (never through a shell — invariant S5) with a deadline and
// captures its output. A non-zero exit returns an *ExitError-wrapping error.
func Run(ctx context.Context, timeout time.Duration, argv ...string) (Result, error) {
	if len(argv) == 0 {
		return Result{}, errors.New("empty argv")
	}
	if !Have(argv[0]) && !strings.ContainsRune(argv[0], os.PathSeparator) {
		return Result{ExitCode: -1}, fmt.Errorf("%s: %w", argv[0], ErrNotFound)
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	err := cmd.Run()
	r := Result{Stdout: out.Bytes(), Stderr: errb.Bytes()}
	if cmd.ProcessState != nil {
		r.ExitCode = cmd.ProcessState.ExitCode()
	}
	if ctx.Err() == context.DeadlineExceeded {
		return r, fmt.Errorf("%s timed out after %s", argv[0], timeout)
	}
	return r, err
}

// Output is Run returning trimmed stdout.
func Output(ctx context.Context, timeout time.Duration, argv ...string) (string, error) {
	r, err := Run(ctx, timeout, argv...)
	return strings.TrimSpace(string(r.Stdout)), err
}

// Stream runs argv in dir with output streamed to w (both stdout and stderr).
// Used for long installs where the user should see progress.
func Stream(ctx context.Context, dir string, w io.Writer, argv ...string) (int, error) {
	if len(argv) == 0 {
		return -1, errors.New("empty argv")
	}
	if !Have(argv[0]) {
		return -1, fmt.Errorf("%s: %w", argv[0], ErrNotFound)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = w, w
	err := cmd.Run()
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	return code, err
}

// Interactive runs argv attached to the user's terminal (sudo prompts, editors,
// user recipes). env entries are appended to the inherited environment.
func Interactive(ctx context.Context, dir string, env []string, argv ...string) (int, error) {
	if len(argv) == 0 {
		return -1, errors.New("empty argv")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), env...)
	err := cmd.Run()
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	return code, err
}

// SudoCallSites is the exhaustive registry of privileged argv prefixes
// (plan §6.8). Sudo refuses anything not listed, and a test pins this table.
var SudoCallSites = [][]string{
	{"sysctl", "-w", "vm.drop_caches=1"},
	{"purge"},
	{"apt-get", "clean"},
	{"apt-get", "autoremove", "--purge", "-y"},
	{"journalctl", "--vacuum-time="},
	{"dnf", "clean", "packages"},
	{"kill", "-TERM"},
}

func sudoAllowed(argv []string) bool {
	for _, site := range SudoCallSites {
		if len(argv) < len(site) {
			continue
		}
		ok := true
		for i, s := range site {
			if strings.HasSuffix(s, "=") {
				ok = ok && strings.HasPrefix(argv[i], s)
			} else {
				ok = ok && argv[i] == s
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// Sudo runs one registered privileged argv interactively (invariant S6). The
// caller must already have printed exactly this argv to the user.
func Sudo(ctx context.Context, argv ...string) (int, error) {
	if !sudoAllowed(argv) {
		return -1, fmt.Errorf("refusing unregistered sudo call: %s", strings.Join(argv, " "))
	}
	if InContainer() {
		return -1, errors.New("sudo paths are disabled inside containers")
	}
	return Interactive(ctx, "", nil, append([]string{"sudo"}, argv...)...)
}
