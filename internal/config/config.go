// Package config loads thine's TOML configuration: built-in defaults, the
// user's global file, and — only once trusted — a project's .thinerc.toml.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/YourAKShaw/thinerc/internal/platform"
)

// Config is the merged, validated configuration (schema version 1).
type Config struct {
	Version   int               `toml:"version"`
	UI        UI                `toml:"ui"`
	Shell     Shell             `toml:"shell"`
	Shortcuts Shortcuts         `toml:"shortcuts"`
	Kill      Kill              `toml:"kill"`
	Groups    map[string]Group  `toml:"groups"`
	Cleanup   Cleanup           `toml:"cleanup"`
	Freeup    Freeup            `toml:"freeup"`
	Recipes   map[string]Recipe `toml:"recipes"`
	Agents    Agents            `toml:"agents"`

	// Provenance, filled by Load (not part of the file format).
	GlobalPath   string `toml:"-"`
	ProjectPath  string `toml:"-"`
	ProjectState string `toml:"-"` // "", "trusted", "untrusted", "changed"
}

type UI struct {
	Icons      string `toml:"icons"`      // emoji | nerd | ascii
	Color      string `toml:"color"`      // auto | always | never
	Animations *bool  `toml:"animations"` // default true
	Splash     *bool  `toml:"splash"`     // animated logo on bare `thine`
	Theme      string `toml:"theme"`
}

type Shell struct {
	// Greeting prints a one-line animated hello when an interactive shell
	// starts. Off by default: it costs a process spawn at startup.
	Greeting bool `toml:"greeting"`
}

type Shortcuts struct {
	Disable []string          `toml:"disable"`
	Rename  map[string]string `toml:"rename"`
	Force   []string          `toml:"force"`
}

type Kill struct {
	Grace    string   `toml:"grace"`
	Scope    string   `toml:"scope"` // auto | listener | job
	DevNames []string `toml:"dev_names"`
	Confirm  []string `toml:"confirm"`
}

type Group struct {
	Ports []int  `toml:"ports"`
	Alias string `toml:"alias"`
}

type Cleanup struct {
	Roots      []string `toml:"roots"`
	StaleAfter string   `toml:"stale_after"`
	Retention  string   `toml:"retention"`
	Exclude    []string `toml:"exclude"`
	Skip       []string `toml:"skip"`
}

type Freeup struct {
	LanguageServers []string `toml:"language_servers"`
}

type Agents struct {
	Policy AgentPolicy `toml:"policy"`
}

type AgentPolicy struct {
	Kill    string `toml:"kill"`    // allow-groups | plan | deny
	Cleanup string `toml:"cleanup"` // plan | deny
	Freeup  string `toml:"freeup"`  // plan | deny
	Recipes string `toml:"recipes"` // plan | allow | deny
}

// Recipe is a typed, declarative user verb (plan §7.9).
type Recipe struct {
	Description string            `toml:"description"`
	Risk        string            `toml:"risk"` // read-only | reversible | regenerable | destructive
	Alias       string            `toml:"alias"`
	Cwd         string            `toml:"cwd"` // current | project | home
	OS          []string          `toml:"os"`
	Params      []Param           `toml:"params"`
	Steps       []Step            `toml:"steps"`
	Shell       string            `toml:"shell"`
	Args        []string          `toml:"args"`
	Passthrough bool              `toml:"passthrough"` // append extra CLI args to the last step
	Darwin      *RecipeOverride   `toml:"darwin"`
	Linux       *RecipeOverride   `toml:"linux"`
	Env         map[string]string `toml:"env"`
}

// RecipeOverride replaces steps or shell on one OS.
type RecipeOverride struct {
	Steps []Step   `toml:"steps"`
	Shell string   `toml:"shell"`
	Args  []string `toml:"args"`
}

type Step struct {
	Run             []string `toml:"run"`
	ContinueOnError bool     `toml:"continue_on_error"`
}

type Param struct {
	Name     string   `toml:"name"`
	Type     string   `toml:"type"` // string | int | enum | path | port
	Values   []string `toml:"values"`
	Default  string   `toml:"default"`
	Required bool     `toml:"required"`
}

// Defaults returns the built-in configuration.
func Defaults() *Config {
	t := true
	return &Config{
		Version: 1,
		UI:      UI{Icons: "emoji", Color: "auto", Animations: &t, Splash: &t, Theme: "thine"},
		Kill: Kill{
			Grace: "3s", Scope: "auto",
			DevNames: []string{"node", "bun", "deno", "python", "python3", "ruby", "php", "go", "cargo",
				"vite", "next-server", "next-router-worker", "esbuild", "webpack", "tsx", "ts-node", "nodemon",
				"uvicorn", "gunicorn", "flask", "rails", "puma", "air", "storybook", "astro", "nuxt", "remix"},
			Confirm: []string{"postgres", "mysqld", "mariadbd", "mongod", "redis-server", "java", "docker",
				"dockerd", "containerd", "code", "cursor", "idea", "chrome", "firefox", "brave"},
		},
		Groups: map[string]Group{"fb": {Ports: []int{3000, 3001}, Alias: "killfb"}},
		Cleanup: Cleanup{
			Roots:      []string{"~/code", "~/projects", "~/dev", "~/work", "~/src", "~/repos", "~/Documents/dev"},
			StaleAfter: "30d", Retention: "7d",
		},
		Freeup:  Freeup{LanguageServers: []string{"tsserver", "typescript-language-server", "gopls", "rust-analyzer", "pyright", "jdtls", "vscode-eslint", "tailwindcss-language-server"}},
		Recipes: map[string]Recipe{},
		Agents:  Agents{Policy: AgentPolicy{Kill: "allow-groups", Cleanup: "plan", Freeup: "plan", Recipes: "plan"}},
	}
}

// ProjectFile is the per-project config name.
const ProjectFile = ".thinerc.toml"

// Load reads defaults → global → trusted project config. A missing global file
// is fine; an invalid one is an error with line and column.
func Load(globalPath string, cwd string, useProject bool) (*Config, error) {
	c := Defaults()
	c.GlobalPath = globalPath
	if data, err := os.ReadFile(globalPath); err == nil {
		var g Config
		if err := decodeStrict(data, &g); err != nil {
			return c, fmt.Errorf("%s: %w", platform.Tilde(globalPath), err)
		}
		c.merge(&g, true)
	} else if !errors.Is(err, os.ErrNotExist) {
		return c, err
	}
	if useProject && cwd != "" {
		if p := FindProject(cwd); p != "" {
			c.ProjectPath = p
			state, err := TrustState(p)
			if err != nil {
				return c, err
			}
			c.ProjectState = state
			if state == "trusted" {
				data, _ := os.ReadFile(p)
				var pc Config
				if err := decodeStrict(data, &pc); err != nil {
					return c, fmt.Errorf("%s: %w", platform.Tilde(p), err)
				}
				c.merge(&pc, false)
			}
		}
	}
	return c, c.Validate()
}

func decodeStrict(data []byte, v *Config) error {
	d := toml.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		var de *toml.DecodeError
		if errors.As(err, &de) {
			row, col := de.Position()
			msg := strings.TrimPrefix(de.Error(), "toml: ")
			if k := de.Key(); len(k) > 0 {
				msg += " " + strconv.Quote(strings.Join(k, "."))
			}
			return fmt.Errorf("line %d, column %d: %s", row, col, msg)
		}
		var sm *toml.StrictMissingError
		if errors.As(err, &sm) {
			return fmt.Errorf("unknown key: %s", strings.TrimSpace(sm.String()))
		}
		return err
	}
	return nil
}

// merge overlays o onto c. Lists merge; maps override by key. Restricted keys
// (agent policy, shortcut forcing, cleanup roots) are honoured only from the
// global file, so a cloned repository can never loosen them (plan §9).
func (c *Config) merge(o *Config, global bool) {
	if o.UI.Icons != "" {
		c.UI.Icons = o.UI.Icons
	}
	if o.UI.Color != "" {
		c.UI.Color = o.UI.Color
	}
	if o.UI.Theme != "" {
		c.UI.Theme = o.UI.Theme
	}
	if o.UI.Animations != nil {
		c.UI.Animations = o.UI.Animations
	}
	if o.UI.Splash != nil {
		c.UI.Splash = o.UI.Splash
	}
	if o.Kill.Grace != "" {
		c.Kill.Grace = o.Kill.Grace
	}
	if o.Kill.Scope != "" {
		c.Kill.Scope = o.Kill.Scope
	}
	c.Kill.DevNames = union(c.Kill.DevNames, o.Kill.DevNames)
	c.Kill.Confirm = union(c.Kill.Confirm, o.Kill.Confirm)
	for k, v := range o.Groups {
		c.Groups[k] = v
	}
	for k, v := range o.Recipes {
		c.Recipes[k] = v
	}
	c.Cleanup.Exclude = union(c.Cleanup.Exclude, o.Cleanup.Exclude)
	c.Cleanup.Skip = union(c.Cleanup.Skip, o.Cleanup.Skip)
	if o.Cleanup.StaleAfter != "" {
		c.Cleanup.StaleAfter = o.Cleanup.StaleAfter
	}
	c.Freeup.LanguageServers = union(c.Freeup.LanguageServers, o.Freeup.LanguageServers)
	c.Shortcuts.Disable = union(c.Shortcuts.Disable, o.Shortcuts.Disable)
	if o.Shortcuts.Rename != nil {
		if c.Shortcuts.Rename == nil {
			c.Shortcuts.Rename = map[string]string{}
		}
		for k, v := range o.Shortcuts.Rename {
			c.Shortcuts.Rename[k] = v
		}
	}
	if !global {
		return
	}
	c.Shell = o.Shell
	c.Shortcuts.Force = union(c.Shortcuts.Force, o.Shortcuts.Force)
	if len(o.Cleanup.Roots) > 0 {
		c.Cleanup.Roots = o.Cleanup.Roots
	}
	if o.Cleanup.Retention != "" {
		c.Cleanup.Retention = o.Cleanup.Retention
	}
	p := o.Agents.Policy
	for dst, src := range map[*string]string{&c.Agents.Policy.Kill: p.Kill, &c.Agents.Policy.Cleanup: p.Cleanup,
		&c.Agents.Policy.Freeup: p.Freeup, &c.Agents.Policy.Recipes: p.Recipes} {
		if src != "" {
			*dst = src
		}
	}
}

func union(a, b []string) []string {
	out := slices.Clone(a)
	for _, s := range b {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

var (
	validIcons = []string{"emoji", "nerd", "ascii"}
	validRisk  = []string{"read-only", "reversible", "regenerable", "destructive"}
	validScope = []string{"auto", "listener", "job"}
	validParam = []string{"string", "int", "enum", "path", "port"}
)

// Validate checks values the decoder cannot.
func (c *Config) Validate() error {
	var errs []string
	add := func(f string, a ...any) { errs = append(errs, fmt.Sprintf(f, a...)) }
	if c.Version != 1 && c.Version != 0 {
		add("version = %d is not supported (expected 1)", c.Version)
	}
	if !slices.Contains(validIcons, c.UI.Icons) {
		add("ui.icons = %q (want emoji, nerd or ascii)", c.UI.Icons)
	}
	if !slices.Contains(validScope, c.Kill.Scope) {
		add("kill.scope = %q (want auto, listener or job)", c.Kill.Scope)
	}
	for _, d := range []struct{ key, v string }{{"kill.grace", c.Kill.Grace}, {"cleanup.stale_after", c.Cleanup.StaleAfter}, {"cleanup.retention", c.Cleanup.Retention}} {
		if _, err := ParseDuration(d.v); err != nil {
			add("%s = %q: %v", d.key, d.v, err)
		}
	}
	for name, g := range c.Groups {
		if !validName(name) {
			add("groups.%s: name must be lowercase letters, digits or dashes", name)
		}
		if len(g.Ports) == 0 {
			add("groups.%s.ports is empty", name)
		}
		for _, p := range g.Ports {
			if p < 1 || p > 65535 {
				add("groups.%s.ports: %d is not a TCP port", name, p)
			}
		}
		if g.Alias != "" && !validName(g.Alias) {
			add("groups.%s.alias = %q is not a valid shell function name", name, g.Alias)
		}
	}
	for name, r := range c.Recipes {
		if !validName(name) {
			add("recipes.%s: name must be lowercase letters, digits or dashes", name)
		}
		if r.Risk == "" {
			add("recipes.%s.risk is required (read-only, reversible, regenerable or destructive)", name)
		} else if !slices.Contains(validRisk, r.Risk) {
			add("recipes.%s.risk = %q is not a risk level", name, r.Risk)
		}
		if len(r.Steps) == 0 && r.Shell == "" {
			add("recipes.%s needs steps or shell", name)
		}
		if len(r.Steps) > 0 && r.Shell != "" {
			add("recipes.%s: use steps or shell, not both", name)
		}
		for i, s := range r.Steps {
			if len(s.Run) == 0 {
				add("recipes.%s.steps[%d].run is empty", name, i)
			}
		}
		if r.Alias != "" && !validName(r.Alias) {
			add("recipes.%s.alias = %q is not a valid shell function name", name, r.Alias)
		}
		for _, p := range r.Params {
			if !slices.Contains(validParam, p.Type) {
				add("recipes.%s param %q: type %q (want string, int, enum, path or port)", name, p.Name, p.Type)
			}
			if p.Type == "enum" && len(p.Values) == 0 {
				add("recipes.%s param %q: enum needs values", name, p.Name)
			}
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func validName(s string) bool {
	if s == "" || len(s) > 48 {
		return false
	}
	for i, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || ((r == '-' || r == '_') && i > 0)
		if !ok {
			return false
		}
	}
	return true
}

// ValidName exposes the shell-function name rule.
func ValidName(s string) bool { return validName(s) }

// ParseDuration accepts Go durations plus a "d" (days) suffix: "30d", "3s".
func ParseDuration(s string) (time.Duration, error) {
	if v, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("invalid days %q", s)
		}
		return time.Duration(n * 24 * float64(time.Hour)), nil
	}
	return time.ParseDuration(s)
}

// Bool reads an optional bool with a default.
func Bool(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// FindProject walks up from dir to the git root (or $HOME) looking for .thinerc.toml.
func FindProject(dir string) string {
	home := platform.Home()
	for d := dir; ; d = filepath.Dir(d) {
		p := filepath.Join(d, ProjectFile)
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return p
		}
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return ""
		}
		if d == home || filepath.Dir(d) == d {
			return ""
		}
	}
}
