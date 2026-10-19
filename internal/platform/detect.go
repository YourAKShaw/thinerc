package platform

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// IsWSL reports whether we run inside Windows Subsystem for Linux.
func IsWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	_, err := os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop")
	return err == nil
}

// InContainer reports Docker/Podman containers (sudo paths are refused there).
func InContainer() bool {
	for _, p := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// OSName is a friendly OS label: "Linux Mint 22", "macOS", "Ubuntu (WSL)".
func OSName() string {
	if IsDarwin() {
		return "macOS"
	}
	name := "Linux"
	if f, err := os.Open("/etc/os-release"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if v, ok := strings.CutPrefix(sc.Text(), "PRETTY_NAME="); ok {
				name = strings.Trim(v, `"`)
				break
			}
		}
	}
	if IsWSL() {
		name += " (WSL)"
	}
	return name
}

// ShellName is the basename of $SHELL ("zsh", "bash", "fish").
func ShellName() string { return filepath.Base(os.Getenv("SHELL")) }

// IsRoot reports whether thine runs as root (refused without --allow-root, S6).
func IsRoot() bool { return os.Geteuid() == 0 }

// AgentCaller names the AI agent driving this process, or "" for a human
// (plan §6.7). Agent mode is defence-in-depth, not a security boundary.
func AgentCaller() string {
	if os.Getenv("THINE_AGENT") == "1" {
		return "agent"
	}
	if os.Getenv("THINE_CALLER") == "mcp" {
		return "mcp"
	}
	markers := map[string]string{
		"CLAUDECODE":    "claude",
		"CODEX_SANDBOX": "codex",
		"GEMINI_CLI":    "gemini",
		"CURSOR_AGENT":  "cursor",
	}
	for env, name := range markers {
		if os.Getenv(env) != "" {
			return name
		}
	}
	return ""
}
