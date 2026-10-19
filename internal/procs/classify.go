package procs

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// Tier is how carefully a process must be treated (plan §6.4).
type Tier string

const (
	TierProtected Tier = "protected" // never signalled
	TierConfirm   Tier = "confirm"   // preview, then explicit confirmation; no automatic SIGKILL
	TierDev       Tier = "dev"       // user-owned dev process: stop immediately, escalate after grace
)

// protectedNames are system processes thine refuses to signal.
var protectedNames = []string{
	"init", "systemd", "launchd", "kthreadd", "kernel_task", "xorg", "xwayland", "gnome-shell",
	"plasmashell", "kwin_x11", "kwin_wayland", "xfwm4", "xfce4-session", "xfce4-panel", "cinnamon",
	"windowserver", "loginwindow", "dbus-daemon", "dbus-broker", "pipewire", "wireplumber",
	"pulseaudio", "networkmanager", "polkitd", "gdm", "lightdm", "sddm", "login", "cron", "containerd-shim",
}

// shellish processes must never become a "job" leader we signal as a group.
var shellish = []string{
	"bash", "zsh", "fish", "sh", "dash", "ksh", "tcsh", "nu", "pwsh", "tmux", "tmux: server", "screen",
	"sshd", "login", "systemd", "init", "launchd", "code", "cursor", "idea", "kitty", "alacritty",
	"wezterm", "wezterm-gui", "gnome-terminal-server", "xfce4-terminal", "konsole", "terminal", "iterm2",
	"ghostty", "warp", "tilix", "terminator",
}

// Policy holds the inputs that classification needs.
type Policy struct {
	DevNames  []string
	Confirm   []string
	Ancestors map[int]bool
	SelfUID   int
	SelfPGID  int
}

// NewPolicy builds a policy for the current process.
func NewPolicy(devNames, confirm []string) Policy {
	return Policy{DevNames: devNames, Confirm: confirm, Ancestors: Ancestors(), SelfUID: Self(), SelfPGID: selfPGID()}
}

func selfPGID() int {
	if p, err := Lookup(os.Getpid()); err == nil {
		return p.PGID
	}
	return -1
}

func matchAny(names, patterns []string) bool {
	for _, n := range names {
		for _, p := range patterns {
			p = strings.ToLower(p)
			if n == p {
				return true
			}
			// python → python3, python3.12; systemd → systemd-journald
			if strings.HasPrefix(n, p) && len(n) > len(p) && strings.ContainsAny(n[len(p):len(p)+1], "0123456789.-") {
				return true
			}
		}
	}
	return false
}

// Classify decides the tier. groupPort says the process holds a port from a
// configured group, which marks it as dev-like even under an unknown name.
func (pol Policy) Classify(p Proc, groupPort bool) (Tier, string) {
	names := p.MatchNames()
	switch {
	case p.PID <= 1:
		return TierProtected, "PID 1 is the system's init process"
	case pol.Ancestors[p.PID]:
		return TierProtected, "it is thine's own parent (your shell, terminal or editor)"
	case p.PGID == pol.SelfPGID && pol.SelfPGID > 0:
		return TierProtected, "it is in thine's own process group"
	case matchAny(names, protectedNames):
		return TierProtected, fmt.Sprintf("%s is a system process", p.Name)
	case matchAny(names, []string{"sshd"}) && os.Getenv("SSH_CONNECTION") != "":
		return TierProtected, "sshd carries your current SSH session"
	case p.UID != pol.SelfUID:
		return TierConfirm, fmt.Sprintf("it is owned by user %q", p.User)
	case matchAny(names, pol.Confirm):
		return TierConfirm, fmt.Sprintf("%s is a database, daemon, IDE or browser", p.Name)
	case matchAny(names, pol.DevNames):
		return TierDev, "user-owned dev process"
	case groupPort:
		return TierDev, "holds a port from a configured group"
	}
	return TierConfirm, fmt.Sprintf("%s is not a known dev process", p.Name)
}

// IsShellish reports whether p is a shell, multiplexer, terminal or IDE.
func IsShellish(p Proc) bool { return matchAny(p.MatchNames(), shellish) }

// JobScope returns the members of p's process group when it is safe to stop
// the whole job (the runner plus its children, e.g. `pnpm dev` → node):
// the leader is not a shell or IDE, the group is not ours, and every member
// is user-owned. Otherwise it returns just p.
func (pol Policy) JobScope(p Proc, all []Proc) []Proc {
	if p.PGID <= 1 || p.PGID == pol.SelfPGID {
		return []Proc{p}
	}
	members := GroupMembers(all, p.PGID)
	if len(members) == 0 {
		return []Proc{p}
	}
	for _, m := range members {
		if m.UID != pol.SelfUID || pol.Ancestors[m.PID] {
			return []Proc{p}
		}
		if m.PID == p.PGID && IsShellish(m) {
			return []Proc{p}
		}
	}
	if !slices.ContainsFunc(members, func(m Proc) bool { return m.PID == p.PID }) {
		members = append(members, p)
	}
	return members
}
