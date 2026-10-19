# Configuration

thine works with zero configuration. When you want to change something, there are two files:

| File | Where | Trusted? |
|---|---|---|
| **Global** | `~/.config/thine/config.toml` (`$XDG_CONFIG_HOME` respected, on macOS too) | Always — it's yours |
| **Project** | `.thinerc.toml`, found by walking up from the current directory to the git root | **Only after `thine trust`**, and only until the file changes |

```sh
thine config init       # write an annotated starter config (never overwrites)
thine config edit       # open it in $VISUAL/$EDITOR, validate on save
thine config validate   # strict check: unknown keys fail with line and column
thine config show       # the effective, merged configuration as JSON
thine config path       # where everything lives
```

## Precedence

Highest wins:

1. Command-line flags (`--theme`, `--grace`, `--roots`…)
2. Environment variables (`THINE_THEME`, …)
3. The **trusted** project `.thinerc.toml`
4. Your global `config.toml`
5. Built-in defaults

**Lists merge** (your `dev_names` are added to the defaults); **maps override by name** (a project's `[groups.fb]` replaces the global one). Some keys are honoured **only from the global file**, so a repository can never loosen them: `[agents.policy]`, `[shortcuts] force`, `[cleanup] roots` and `retention`, and `[shell]`.

## The global file, annotated

```toml
version = 1

[ui]
theme      = "thine"   # thine | catppuccin-mocha | catppuccin-latte | tokyo-night | dracula | nord | gruvbox | rose-pine | synthwave
icons      = "emoji"   # emoji | nerd (needs a Nerd Font: `thine fonts`) | ascii
color      = "auto"    # auto | always | never
animations = true      # spinners, splash, confetti — always off for pipes, CI, NO_COLOR
splash     = true      # the animated logo when you run bare `thine`

[shell]
greeting = false       # a one-line hello in every new interactive shell (no scans, ~3 ms)

[shortcuts]
disable = []                    # e.g. ["cleanup"] if you already own that name
rename  = { kp = "kport" }      # rename a shortcut
force   = ["killfb"]            # define it even though you already have one (yours is unaliased)

[kill]
grace     = "3s"                # SIGTERM → wait → SIGKILL (dev processes only)
scope     = "auto"              # auto (job for groups, listener for ports) | listener | job
dev_names = ["air", "hugo"]     # extra process names treated as dev servers
confirm   = ["minio"]           # extra names that always need confirmation

[groups.fb]                     # killfb
ports = [3000, 3001]
alias = "killfb"

[groups.db]                     # killdb
ports = [5432, 6379]
alias = "killdb"

[cleanup]
roots       = ["~/code", "~/projects"]   # where to look for stale node_modules, .next, target/, .venv
stale_after = "30d"                      # idle this long before an artifact is a candidate
retention   = "7d"                       # how long `thine undo` can restore quarantined items
exclude     = ["~/code/archive/**"]      # never touch these
skip        = ["docker-images"]          # categories to always skip

[freeup]
language_servers = ["tsserver", "gopls", "rust-analyzer", "pyright"]

[agents.policy]                 # global file only
kill    = "allow-groups"        # agents may stop dev processes on configured groups
cleanup = "plan"
freeup  = "plan"
recipes = "plan"                # "allow" lets agents run non-read-only recipes

[recipes.gpa]
description = "Push the current branch and set its upstream"
risk        = "reversible"
alias       = "gpa"
passthrough = true
steps       = [{ run = ["git", "push", "-u", "origin", "HEAD"] }]
```

Durations accept Go syntax (`3s`, `500ms`, `2h`) plus days (`30d`). Paths accept `~` and `$VARS`.

## A project file

Commit it so your team shares port groups and recipes; each teammate runs `thine trust` once after reviewing it.

```toml
version = 1

[groups.fb]
ports = [5173, 8080]          # this project's frontend + backend
alias = "killfb"

[groups.stack]
ports = [5173, 8080, 5432]
alias = "killstack"

[recipes.dev]
description = "Free the ports, then start everything"
risk        = "reversible"
alias       = "dev"
steps       = [{ run = ["thine", "kill", "--group", "fb"] }, { run = ["pnpm", "dev"] }]

[recipes.db-reset]
description = "Drop, migrate and reseed the local database"
risk        = "destructive"
cwd         = "project"
params      = [{ name = "env", type = "enum", values = ["dev", "test"], default = "dev" }]
steps = [
  { run = ["docker", "compose", "exec", "-T", "db", "dropdb", "--if-exists", "app_{{env}}"] },
  { run = ["pnpm", "db:migrate"] },
  { run = ["pnpm", "db:seed"] },
]
```

Until it's trusted, thine prints one line — `🔒 ignoring untrusted ~/code/app/.thinerc.toml — review it, then run thine trust` — and behaves as if the file weren't there.

## Recipes

A recipe is a typed, reviewable alias. Full guide: [recipes cookbook](guides/recipes-cookbook.md).

| Key | Meaning |
|---|---|
| `description` | One line shown in `thine recipes` |
| `risk` | **Required.** `read-only` · `reversible` · `regenerable` · `destructive` — decides confirmation |
| `alias` | Defines a shell function with this name (subject to the collision guard) |
| `steps` | `[{ run = ["argv", "…"] }, …]` — run in order, stop at the first failure unless `continue_on_error = true` |
| `shell` | A script run with `sh -c`, instead of `steps`. Parameters arrive as `$1`, `$2`… (never spliced). Shown as **shell** (unreviewed) |
| `args` | Positional arguments for a `shell` script, e.g. `["{{path}}"]` |
| `params` | `[{ name, type = "string\|int\|enum\|path\|port", default, values, required }]` — validated before anything runs; `{{name}}` is substituted **inside argv elements only** |
| `passthrough` | Forward extra command-line arguments to the last step (alias-like) |
| `cwd` | `current` (default) · `project` (git root) · `home` |
| `os` | Restrict to `["linux"]`, `["darwin"]`… |
| `[recipes.x.darwin]` / `[recipes.x.linux]` | Replace `steps` or `shell` on one OS |
| `env` | Extra environment variables (`{{param}}` allowed) |

## Environment variables

| Variable | Effect |
|---|---|
| `NO_COLOR` | No colour (and no animation) |
| `THINE_NO_ANIM=1` | No animation |
| `THINE_THEME=<name>` | Theme for this shell |
| `THINE_CONFIG=<path>` | Use a different global config |
| `THINE_STATE_DIR=<dir>` | Move state (ops log, plans, trust) |
| `THINE_HOME=<dir>` | Pretend this is your home — used by tests, great for safe experiments |
| `THINE_AGENT=1` | Force agent mode (plans only for anything risky) |
| `THINE_ALLOW_ROOT=1` | Allow running as root (not recommended) |
| `COLORTERM=truecolor` | Tell thine your terminal supports 24-bit colour |

## Files thine writes

| Path | What | Mode |
|---|---|---|
| `~/.config/thine/config.toml` | Your config (only when you run `config init`, `theme set`, `import --write`, `recipes starter --write`) | 0644 |
| `~/.local/state/thine/ops.jsonl` | Operations log (rotates at 5 MiB, keeps 3) | 0600 |
| `~/.local/state/thine/trust.json` | Trusted project files and their SHA-256 | 0600 |
| `~/.local/state/thine/plans/` | Saved cleanup plans (24 h) | 0600 |
| `~/.local/share/thine/quarantine/` | Undo buffer | 0700 |
| `~/.local/share/thine/zsh/_thine`, `~/.local/share/bash-completion/completions/thine`, `~/.config/fish/completions/thine.fish` | Completions (`thine setup`) | 0644 |
| Your rc file | One marked block (`thine setup`), with a `.thine-bak-<UTC>` backup | unchanged |
