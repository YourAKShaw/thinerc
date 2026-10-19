# 🧪 Recipes cookbook

Recipes are thine's "much higher-level aliases": typed parameters, a risk label that decides confirmation, per-OS variants, a preview for anything risky, and a line in `thine history`. This page is a collection of patterns to copy. The key reference is in [configuration](../configuration.md#recipes).

## The shape of a recipe

```toml
[recipes.<name>]                # lowercase letters, digits, - or _
description = "…"               # shown in `thine recipes`
risk        = "reversible"      # read-only | reversible | regenerable | destructive  (required)
alias       = "<shell name>"    # optional shell function
steps       = [{ run = ["argv", "…"] }, …]   # or: shell = '…' (+ args = [...])
```

How risk maps to behaviour:

| Risk | Runs… |
|---|---|
| `read-only` | Immediately (agents may run these too) |
| `reversible` | Immediately; logged |
| `regenerable` | After a `[y/N]` (or `--yes`) |
| `destructive` | After you type `run <name>` |

## Git

```toml
[recipes.gpa]
description = "Push the current branch and set its upstream"
risk        = "reversible"
alias       = "gpa"
passthrough = true                                   # gpa --no-verify works
steps       = [{ run = ["git", "push", "-u", "origin", "HEAD"] }]

[recipes.gcor]
description = "Fetch a remote branch, check it out and track it"
risk        = "reversible"
alias       = "gcor"
params      = [{ name = "branch", type = "string", required = true }]
steps = [
  { run = ["git", "fetch", "origin", "{{branch}}:{{branch}}"] },
  { run = ["git", "checkout", "{{branch}}"] },
  { run = ["git", "branch", "--set-upstream-to", "origin/{{branch}}"] },
]

[recipes.gprune]
description = "Delete local branches already merged into this one"
risk        = "destructive"
alias       = "gprune"
shell       = "git branch --merged | grep -vE '^\\*|^[[:space:]]*(main|master|develop)$' | xargs -r git branch -d"

[recipes.gundo]
description = "Undo the last commit but keep its changes staged"
risk        = "reversible"
alias       = "gundo"
steps       = [{ run = ["git", "reset", "--soft", "HEAD~1"] }]
```

> 💡 Prefer `reset --soft` recipes over `reset --hard` aliases — and if you keep a hard reset, label it `destructive` so it asks first.

## Node and package managers

```toml
[recipes.dev]
description = "Free the ports, then start the dev stack"
risk        = "reversible"
alias       = "dev"
cwd         = "project"
steps       = [{ run = ["thine", "kill", "--group", "fb"] }, { run = ["pnpm", "dev"] }]

[recipes.t]
description = "Run tests, optionally filtered"
risk        = "read-only"
alias       = "t"
passthrough = true                          # t src/cart  →  pnpm test src/cart
steps       = [{ run = ["pnpm", "test"] }]

[recipes.outdated]
description = "Outdated dependencies, newest majors first"
risk        = "read-only"
alias       = "outdated"
steps       = [{ run = ["npx", "--yes", "npm-check-updates", "--format", "group"] }]
```

## Parameters, validated

```toml
[recipes.serve]
description = "Serve a folder over HTTP on a port"
risk        = "read-only"
alias       = "serve"
params = [
  { name = "port", type = "port", default = "8000" },          # 1–65535, checked before running
  { name = "dir",  type = "path", default = "." },
]
steps = [{ run = ["python3", "-m", "http.server", "{{port}}", "--directory", "{{dir}}"] }]

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

```sh
serve 9000 ./dist                        # positionals fill params in order
thine run --param env=test db-reset      # named params go before the recipe name
thine run db-reset --dry-run             # print the steps, run nothing
```

**Why this is safe:** values are validated by type and substituted only *inside* argv elements. Passing `$(touch pwned)` as a parameter gives a program that literal string — it is never interpreted by a shell. (There's a test for exactly that.)

## Shell recipes (when you need pipes)

```toml
[recipes.biggest]
description = "The 20 largest directories under a path"
risk        = "read-only"
alias       = "biggest"
params      = [{ name = "path", type = "path", default = "." }]
shell       = 'du -xh -d 2 "$1" 2>/dev/null | sort -rh | head -20'
args        = ["{{path}}"]               # → $1
```

`shell` recipes run as `sh -c '<script>' thine-recipe "$1" "$2"…`. Always quote `"$1"` in the script. They show a **shell** badge in `thine recipes` so reviewers know to read them, and agents treat them as plan-only unless they're read-only.

## Cross-platform

```toml
[recipes.cpwd]
description = "Copy the current path to the clipboard"
risk        = "read-only"
alias       = "cpwd"
shell       = "pwd | tr -d '\\n' | (wl-copy 2>/dev/null || xclip -selection clipboard)"
darwin      = { shell = "pwd | tr -d '\\n' | pbcopy" }

[recipes.open]
description = "Open a file or URL with the default app"
risk        = "read-only"
alias       = "o"
params      = [{ name = "target", type = "string", default = "." }]
steps       = [{ run = ["xdg-open", "{{target}}"] }]
darwin      = { steps = [{ run = ["open", "{{target}}"] }] }

[recipes.flushdns]
description = "Flush the DNS cache"
risk        = "reversible"
os          = ["darwin"]                 # only offered on macOS
steps       = [{ run = ["sudo", "dscacheutil", "-flushcache"] }, { run = ["sudo", "killall", "-HUP", "mDNSResponder"] }]
```

## Keep going after a failure

```toml
[recipes.lint-all]
description = "Run every linter and report all of them"
risk        = "read-only"
steps = [
  { run = ["pnpm", "lint"], continue_on_error = true },
  { run = ["pnpm", "typecheck"], continue_on_error = true },
  { run = ["pnpm", "test", "--run"] },
]
```

## Team recipes in the repo

Put project recipes and port groups in `.thinerc.toml` at the repository root and commit it. Every teammate runs `thine trust` once after reading it; any later change to the file needs a fresh review. A project file can add groups and recipes, but it can't change agent policy or force shortcut names.

## From your aliases

```sh
thine import            # suggestions from ~/.bashrc, ~/.zshrc, alias files
thine import --toml     # just the TOML, to edit by hand
thine import --write    # append (with backup and validation)
```

See [migrating from aliases](migrating-from-aliases.md) for a worked example.
