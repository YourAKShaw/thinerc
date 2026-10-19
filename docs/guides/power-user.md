# ⚡ The thine power-user handbook

From "I installed it" to "my whole team's terminal runs on it" in seven levels. Each level is useful on its own — stop wherever you like, come back when you want more.

| Level | You'll be able to… | Time |
|---|---|---|
| [0 · Wire it in](#level-0--wire-it-in) | Run thine from every shell | 2 min |
| [1 · Daily verbs](#level-1--daily-verbs) | Free ports, read memory, reclaim disk without fear | 10 min |
| [2 · Make it yours](#level-2--make-it-yours) | Port groups, shortcut names, theme, fonts | 15 min |
| [3 · Recipes](#level-3--recipes-your-own-verbs) | Turn your aliases into typed, shareable verbs | 20 min |
| [4 · Reclaim like a pro](#level-4--reclaim-like-a-pro) | Tune cleanup, read memory like an SRE, undo anything | 15 min |
| [5 · Script it, give it to agents](#level-5--script-it-and-give-it-to-agents) | JSON + exit codes in scripts, CI and AI agents | 20 min |
| [6 · Speed & hygiene](#level-6--speed-and-hygiene) | Prove it costs nothing; build the ideal terminal around it | 10 min |

A [one-page cheat sheet](#cheat-sheet) and [a day with thine](#a-day-with-thine) are at the end.

---

## Level 0 · Wire it in

```sh
curl -fsSL https://raw.githubusercontent.com/YourAKShaw/.thinerc/main/scripts/install.sh | sh   # or: make install
thine setup          # shows the diff, backs up your rc file, adds one marked block at the end
exec $SHELL          # reload
thine                # 👋 the animated welcome
```

`thine setup` only touches the block between `# >>> thine >>>` and `# <<< thine <<<`. It's idempotent, keeps a `.thine-bak-<timestamp>` copy, installs tab completion, and `thine setup --uninstall` removes everything byte for byte. Got zsh *and* bash? `thine setup --all`.

Then run the health check once:

```sh
thine doctor
```

Pay attention to **Shell integration → shortcuts**. If you already had `alias freeup=…` or a `killport()` function, thine leaves yours alone and tells you the file and line. You decide: delete yours, rename thine's, or force thine's ([Level 2](#shortcut-names)).

---

## Level 1 · Daily verbs

### Ports — the thing you do twenty times a day

```sh
whoport 3000            # who's on :3000? process, command line, cwd, uptime, a safety badge
ports                   # every dev port you have open
kp 3000                 # stop it: SIGTERM → visible grace bar → SIGKILL only if it's a dev process
killfb                  # stop the frontend + backend group (3000, 3001 by default)
killfb && pnpm dev      # exits 0 only when the ports are really free — so this chains reliably
```

What makes these safe enough to use without thinking:

- The badge tells you how thine will treat the process: **dev** (stops immediately), **confirm** (asks first — databases, IDEs, browsers, Docker, anything unknown), **protected** (never — your shell, PID 1, system daemons), **docker** (stops the container, not the proxy).
- `killfb` stops the whole *job* — `pnpm dev` and the `node` it spawned — so the runner can't restart the server behind your back. The plan lists every process first.
- A PID is only signalled if its start time still matches, so a recycled PID is never hit.
- Want to look first? `kp 3000 --dry-run`.

### Memory — the honest version

```sh
freeup
```

You'll see **available** memory (what apps can actually get), pressure, swap, and the page cache labelled *reclaimable on demand*. Below that: top consumers grouped by app, then numbered candidates — your idle dev servers and language servers — and a question: `Stop which? 1,3 · all · none`. After stopping, thine waits two seconds and shows the **measured** change.

> **Why doesn't it "free 2 GB" like my old alias?** Your old alias dropped the page cache. That memory was already available — the kernel hands it back instantly when an app needs it. Dropping it inflates "free", barely changes "available", and slows your next file reads. If you want to see that for yourself: `freeup --drop-caches` (it explains, prints the exact `sudo` command, and shows both numbers side by side).

### Disk — plan first, always

```sh
cleanup                 # scan → plan with three honest totals → "Apply this plan? [y/N]"
```

Read the plan from top to bottom: **regenerable** items (caches) are cleaned with each tool's own command; **reversible** items (stale `node_modules`, old installers in Downloads) are moved to quarantine and can be restored for 7 days. Say yes, and enjoy the confetti if you reclaimed more than a gigabyte. Changed your mind?

```sh
thine undo              # restores the most recent cleanup's quarantined items
```

### The dashboard

```sh
whatsup                 # ports · memory · disk · Docker · running AI agents · git — in ~90 ms
```

---

## Level 2 · Make it yours

```sh
thine config init       # annotated ~/.config/thine/config.toml
thine config edit       # opens $EDITOR, validates when you save
```

### Port groups

Every group with an `alias` becomes a shortcut:

```toml
[groups.fb]
ports = [5173, 8080]          # Vite + API
alias = "killfb"

[groups.db]
ports = [5432, 6379, 27017]
alias = "killdb"

[groups.all]
ports = [3000, 3001, 5173, 8080, 5432, 6379]
alias = "killall-dev"
```

Ports in a group are treated as dev ports even if the process name is unusual (a Rust binary, a custom server), so `killfb` never stops to ask about them.

### Shortcut names

```toml
[shortcuts]
rename  = { kp = "kport", cleanup = "tidy" }   # avoid clashes or match your muscle memory
disable = ["nuke-nm"]                          # don't define this one at all
force   = ["killport"]                         # replace your old killport() with thine's
```

Re-run `thine setup` (or just open a new shell) and check `thine doctor`.

### Theme and fonts

```sh
thine theme ls                      # nine palettes, live swatches
thine theme preview                 # cycle through a sample screen in each
thine theme set catppuccin-mocha    # make it the default
thine --theme dracula whatsup       # or just for one run (THINE_THEME works too)
```

thine renders hints and metadata in *italics*. Give your terminal a font with a **cursive italic** and those lines look hand-written:

```sh
thine fonts                         # what's recommended, what you have
thine fonts install victor-mono     # pinned Nerd Fonts release, SHA-256 verified, installed for your user
thine fonts test                    # specimen: italics, ligatures, Nerd icons, gradient
```

Then switch `icons = "nerd"` in `[ui]` for crisp glyph icons. The full guide — including theming the *whole* terminal with Gogh, iTerm2-Color-Schemes or Catppuccin — is in [fonts and themes](fonts-and-themes.md).

---

## Level 3 · Recipes: your own verbs

A recipe is an alias that grew up: typed parameters, a risk label, per-OS variants, a preview before anything risky, and an entry in the ops log.

### Import what you already have

```sh
thine import                 # reads ~/.bashrc, ~/.zshrc and alias files
```

You'll get a table: each alias as a suggested recipe with a guessed risk level; helpers thine already replaces (`killport` → `thine kill`); ones that must stay in your shell (`source ~/.zshrc`, `cd …`); and warnings like *"`gh` also names a program on your PATH — this alias hides it"*. When you're happy:

```sh
thine import --toml          # review the TOML
thine import --write         # append it (backed up, validated, rolled back if invalid)
```

### Or start from the starter pack

```sh
thine recipes starter          # gpa, gpff, gbr, glast, gwho, gtrack, gcor, gprune, treen, cpwd, myip, diskfree, biggest
thine recipes starter --write
```

### Write your own

```toml
[recipes.biggest]
description = "The 20 largest directories under a path"
risk        = "read-only"
alias       = "biggest"
params      = [{ name = "path", type = "path", default = "." }]
shell       = 'du -xh -d 2 "$1" 2>/dev/null | sort -rh | head -20'
args        = ["{{path}}"]

[recipes.db-reset]
description = "Drop, migrate and reseed the local database"
risk        = "destructive"                 # you'll be asked to type "run db-reset"
cwd         = "project"
params      = [{ name = "env", type = "enum", values = ["dev", "test"], default = "dev" }]
steps = [
  { run = ["docker", "compose", "exec", "-T", "db", "dropdb", "--if-exists", "app_{{env}}"] },
  { run = ["pnpm", "db:migrate"] },
  { run = ["pnpm", "db:seed"] },
]

[recipes.cpwd]
description = "Copy the current path"
risk        = "read-only"
alias       = "cpwd"
shell       = "pwd | tr -d '\\n' | xclip -selection clipboard"
darwin      = { shell = "pwd | tr -d '\\n' | pbcopy" }
```

```sh
biggest ~/Downloads                     # positional → the first parameter
thine run --param env=test db-reset     # named parameter (flags go before the recipe name)
thine run db-reset --dry-run            # see exactly what would run
thine recipes show db-reset             # the TOML and the commands for this OS
```

Parameters are validated before anything runs and substituted **only inside argv elements** — `$(rm -rf ~)` as a value is just a string. `shell` recipes get values as `$1`, `$2`… and are labelled *shell* so reviewers know to read them. More patterns: [recipes cookbook](recipes-cookbook.md).

### Share them with your team

Commit a `.thinerc.toml` at the repo root with the project's port groups and recipes. Each teammate reviews and trusts it once:

```sh
thine trust          # shows the file with line numbers and the recipes it defines, then pins its SHA-256
```

If someone changes the file later, thine ignores it again (with a one-line notice) until it's re-reviewed. A repository can add groups and recipes but can never loosen your agent policy or force shortcuts.

---

## Level 4 · Reclaim like a pro

### Tune cleanup

```sh
thine cleanup ls                               # every category, its risk and its command
cleanup --only npm,pnpm,go-build               # just caches
cleanup --skip node-modules
cleanup --include go-modcache,python-venv      # opt-in categories
cleanup --include system                       # apt/journald/dnf — each sudo command printed and confirmed
cleanup --roots ~/code,~/work --older 14d      # where and how stale
```

Make your defaults permanent:

```toml
[cleanup]
roots       = ["~/code", "~/work", "~/oss"]
stale_after = "21d"
retention   = "14d"                       # keep undo longer
exclude     = ["~/code/archive/**"]
skip        = ["docker-images"]
```

### Read the plan like an auditor

- **frees now** — caches and owning-tool commands; gone for good, rebuilt on demand.
- **frees after retention** — quarantined items; restorable until the date shown, purged by a later `cleanup --apply` (listed in that plan as *expired*).
- **not reclaimable** — hardlinks shared with a pnpm/Bun store; deleting `node_modules` wouldn't free them, so they're never counted.

### Undo, precisely

```sh
thine history -n 20                      # find the operation
thine trash ls                           # what's still restorable, and until when
thine undo 01M5AST1                      # restore by ID prefix
thine undo 01M5AST1 --item node-modules:/home/me/code/old/node_modules
thine trash purge --older 30d            # permanent; you'll type "purge N"
```

Restores never overwrite: if you recreated a path, the old copy lands next to it as `…thine-restored-1`.

### Reset a broken install

```sh
nuke-nm             # quarantine every node_modules, reinstall from the lockfile, roll back if it fails
nuke-nm --frozen    # npm ci / --frozen-lockfile / --immutable
```

---

## Level 5 · Script it, and give it to agents

### JSON everywhere

Every read verb and every plan speaks JSON with `"schema_version": 1`; exit codes are the same as for humans.

```sh
thine ports --json | jq -r '.listeners[] | "\(.port)\t\(.name)\t\(.cwd)"'
thine port 3000 --json | jq -e '.free | length == 1' && echo "3000 is free"
thine whatsup --json | jq '.dashboard.memory.available_bytes'
thine freeup --json | jq '.candidates'
thine doctor --json | jq '.checks[] | select(.status != "ok")'
```

### Exit codes you can branch on

| Code | Meaning |
|---|---|
| 0 | Done (including "nothing to do") |
| 4 | Partial — something failed or a port is still held |
| 5 | Refused by the safety policy |
| 6 | Needs approval — a plan was printed (and, for cleanup, saved) |

```sh
# package.json
"predev": "thine kill --group fb",       # dev processes on named ports: no prompt needed, even without a TTY
"dev": "vite"

# a nightly reminder instead of a nightly deletion
thine cleanup --json > ~/cleanup-plan.json; [ $? -eq 6 ] && thine notify "cleanup plan ready: $(jq -r .plan_id ~/cleanup-plan.json)"
```

Without a terminal, nothing risky happens unless you pass `--apply` (exit 6 otherwise) — so a mistyped script can't wipe anything.

### Agents (Claude Code, Codex, Gemini…)

```sh
thine ai doctor      # which agents you have, versions, hooks, this repo's AGENTS.md/CLAUDE.md
thine ai hooks       # paste-ready Stop/Notification hooks → desktop notifications when an agent finishes
```

When an agent runs thine (thine detects `CLAUDECODE`, `CODEX_SANDBOX`, `GEMINI_CLI`, or `THINE_AGENT=1`), anything risky only produces a plan. The agent can tell you *"I've prepared a cleanup — run `thine apply 01M5…` to apply it"*, and you stay in control. Details: [AI agents guide](ai-agents.md).

### Notifications for anything

```sh
pnpm build; thine notify "build finished"
cargo test && thine notify "tests passed ✅" || thine notify "tests failed ❌"
```

---

## Level 6 · Speed and hygiene

### Prove it costs nothing

```sh
thine bench
```

On an 8 GB laptop: cold start ≈ 2 ms, and ≈ 3 ms added to a new zsh. **Zero** per command — thine installs no prompt hooks. For whole-shell latency, use [zsh-bench](https://github.com/romkatv/zsh-bench).

### Build the ideal terminal around it

thine is a verbs layer; it deliberately doesn't replace the excellent tools that own other jobs. `thine doctor` suggests what's missing:

| Job | Tool |
|---|---|
| Prompt | [Starship](https://starship.rs) |
| History | [Atuin](https://atuin.sh) |
| Jumping around | [zoxide](https://github.com/ajeetdsouza/zoxide) |
| Fuzzy finding | [fzf](https://github.com/junegunn/fzf) |
| `ls` / `cat` / `find` | [eza](https://eza.rocks) · [bat](https://github.com/sharkdp/bat) · [fd](https://github.com/sharkdp/fd) |
| Runtime versions | [mise](https://mise.jdx.dev) |

### Keep it healthy

```sh
thine doctor         # after editing rc files, installing tools, or upgrading
thine config validate
thine trust ls       # which project files you've trusted (and which changed since)
```

### Make it greet you (optional)

```toml
[shell]
greeting = true      # a one-line hello in each new shell — no scans, ~3 ms
```

---

## Cheat sheet

| I want to… | Type |
|---|---|
| See who's on a port | `whoport 3000` |
| Free a port / my dev stack | `kp 3000` · `killfb` |
| Free it and start again | `killfb && pnpm dev` |
| See all my dev ports | `ports` |
| Understand my memory | `freeup` |
| Reclaim disk safely | `cleanup` → `y` |
| Undo the last cleanup | `thine undo` |
| Reinstall node_modules safely | `nuke-nm` |
| Get a one-screen status | `whatsup` |
| Turn my aliases into recipes | `thine import --write` |
| Run a recipe | `<alias>` or `thine run <name>` |
| Change the look | `thine theme set <name>` · `thine fonts install victor-mono` |
| Check everything | `thine doctor` |
| Get notified | `long-command; thine notify done` |
| Feed a script or an agent | add `--json` |
| Just have fun | `thine fx all` |

## A day with thine

```text
09:02  whatsup                     → 2 agents running, 81% disk, memory pressure calm
09:03  cleanup                     → 8 GiB npm cache + 3 stale node_modules → y → 🎉 Reclaimed 8.4 GiB
09:10  killfb && pnpm dev          → yesterday's zombie dev server stopped, ports free, dev starts
11:45  whoport 5432                → postgres, confirm-tier — leave it alone
14:20  nuke-nm --frozen            → lockfile install fails → old node_modules restored automatically
16:30  freeup                      → tsserver ×3 using 900 MiB → "3" → available +880 MiB (measured)
17:55  claude finishes a refactor  → desktop notification "Claude Code · shop (feat/cart) · finished"
18:00  thine history               → everything thine did today, with sizes and undo markers
```
