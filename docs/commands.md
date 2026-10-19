# Command reference

Every verb, its flags, and what it returns. Run `thine <command> --help` for the same information in your terminal; `thine completion <shell>` powers tab completion (installed by `thine setup`).

**Global flags** (work with every command):

| Flag | Effect |
|---|---|
| `--json` | Machine-readable output with `"schema_version": 1`; no colour, icons or animation |
| `--plain` | No colour, icons or animation — reads cleanly in screen readers and logs |
| `-q, --quiet` | Only results and errors |
| `--no-color` · `--no-anim` | Turn off colour · animation |
| `--theme <name>` | Use a colour theme for this run (`thine theme ls`) |
| `--config <path>` | Use a different global config file |
| `--no-project` | Ignore the project's `.thinerc.toml` |

## Shell shortcuts

`thine setup` (or `eval "$(thine init zsh)"`) defines these functions — unless you already own the name, in which case yours wins and `thine doctor` tells you where it's defined.

| Shortcut | Runs |
|---|---|
| `whoport <port…>` | `thine port` |
| `ports` | `thine ports` |
| `kp <port…>` · `killport <port…>` | `thine kill` |
| `killfb` (and `kill<group>` for every group with an `alias`) | `thine kill --group fb` |
| `freeup` | `thine freeup` |
| `cleanup` | `thine cleanup` |
| `whatsup` | `thine whatsup` |
| `nuke-nm` | `thine nm reset` |
| your recipe aliases (`gpa`, `treen`…) | `thine run <recipe> -- …` |

Customise with `[shortcuts] disable`, `rename` and `force` in your [config](configuration.md#shortcuts).

---

## Ports & processes

### `thine port <port…>` — who is listening?

```sh
whoport 3000
whoport 3000,5173 8080 --json
```

Shows the process, PID, a dev/confirm/protected/docker badge, the full command line, its working directory, owner, uptime and bind address. A free port says so. Exit 0 whether or not anything listens ("found" is in the data); 1 only if listing sockets fails.

### `thine ports` — every listener you own

`--all` / `-a` includes system and other users' listeners (their PID is hidden without root).

### `thine kill [port…]` — free ports gracefully

```sh
kp 3000                       # one port
killfb && pnpm dev            # a group, chained
thine kill --group db --dry-run
thine kill 8080 --docker      # port published by a container → docker stop
thine kill --pid 4123
```

| Flag | Meaning |
|---|---|
| `-g, --group <name>` | Ports from a configured group (repeatable) |
| `--pid <pid>` | Target a process directly |
| `--scope listener\|job` | Stop just the listener, or its whole job (runner + children). Default: `job` for groups, `listener` for ports |
| `--grace <dur>` | Wait after SIGTERM before SIGKILL (default `3s`) |
| `--force` | Allow SIGKILL for non-dev processes that ignore SIGTERM |
| `--docker` | Stop the container that publishes the port |
| `--sudo` | Allow stopping another user's process (`sudo kill -TERM`, printed first) |
| `-y, --yes` · `--apply` | Confirm non-dev targets without asking · act without a terminal |
| `--dry-run` | Show the plan only |

**Exit codes are built for chaining:** 0 = every requested port is free (including "nothing was listening"), 4 = something is still held, 5 = refused (protected), 6 = needs approval.

---

## Reclaim disk & memory

### `thine freeup` — honest memory

Shows total, **available**, pressure, swap and page cache with an animated gauge; the top consumers grouped by application (PSS where possible); and numbered reclaim candidates — your own idle dev servers and language servers.

| Flag | Meaning |
|---|---|
| `--stop all\|none\|1,3-4` | Stop candidates without the interactive question |
| `--drop-caches` | Also drop the page cache (explained first; runs `sudo sysctl -w vm.drop_caches=1` or `sudo purge`) |
| `-y, --yes` | Skip the `--drop-caches` confirmation |
| `--top <n>` | How many consumers to list (default 8) |

After stopping anything it waits two seconds and reports the **measured** change in available memory.

### `thine cleanup` — plan, apply, undo

```sh
cleanup                                  # scan and show the plan; asks before acting
cleanup --apply --yes                    # act without asking
cleanup --only npm,docker-build
cleanup --include system,go-modcache     # opt-in categories
cleanup --roots ~/code --older 60d
thine cleanup ls                         # the catalog
thine cleanup --json                     # plan for scripts/agents (saved; apply with `thine apply <id>`)
```

The plan groups items by category with a risk badge, and ends with three honest totals: **frees now**, **frees after retention** (quarantined), and **not reclaimable** (hardlinks shared with a package store). Categories:

| ID | What | Action | Default |
|---|---|---|---|
| `npm` · `pnpm` · `yarn` · `bun` · `pip` · `uv` | Package-manager caches | The tool's own clean command | ✔ |
| `go-build` · `go-modcache` | Go caches | `go clean -cache` · `-modcache` | ✔ · optional |
| `docker-build` · `docker-images` | Build cache, dangling images | `docker builder prune -f` · `docker image prune -f` | ✔ (if the daemon runs) |
| `homebrew` | Old versions and downloads | `brew cleanup --prune=all` | ✔ |
| `thumbnails` (Linux) · `xcode-derived` (macOS) | Regenerable caches | Emptied in place | ✔ |
| `playwright-browsers` | Browser downloads | Emptied in place | optional |
| `downloads-installers` | `.dmg .pkg .deb .rpm .AppImage .exe .msi .iso` older than 30 days | Quarantine | ✔ |
| `node-modules` · `next-build` · `rust-target` · `python-venv` | Stale project artifacts under your roots | Quarantine | ✔ (venv optional) |
| `apt` · `journald` · `dnf` (`--include system`) | System caches and logs | Printed `sudo` command, confirmed per item | opt-in |

### `thine apply <plan-id>`

Applies a saved cleanup plan (for example one an agent proposed with `--json`). Plans expire after 24 hours; every item is re-validated first. Agents can't apply.

### `thine undo [op-id]` · `thine history` · `thine trash ls|purge`

```sh
thine history -n 10          # operations: time, verb, result, size, undoable
thine undo                   # restore the latest undoable operation
thine undo 01JB8Q --item downloads:/home/me/Downloads/tool.iso
thine trash ls               # quarantine by operation, with expiry
thine trash purge --older 7d # permanent — asks you to type "purge N"
```

### `thine whatsup` — the dashboard

Host, uptime and load; your listening ports; memory and disk gauges; Docker containers; running AI-agent sessions and where; and the current repo's git state. Every section is gathered concurrently under a one-second budget (≈ 90 ms in practice).

### `thine nm reset` (`nuke-nm`) — reinstall with rollback

Finds the project root and package manager (pnpm, bun, Yarn classic/Berry, npm), moves every `node_modules` into quarantine, runs the install with its output streamed, and **restores the old tree automatically if the install fails**. `--frozen` uses the lockfile-strict install; `--purge-old` deletes the old tree right away; `-y` skips the question.

---

## Recipes

### `thine run <recipe> [args…] [-- extra]`

Runs a recipe from your config. Read-only and reversible recipes run immediately; regenerable ones ask `[y/N]`; destructive ones need you to type `run <name>`. `--param k=v` (before the recipe name) sets a parameter; positional arguments fill parameters in order; `--dry-run` shows the commands. The last step's exit code is returned.

### `thine recipes [ls|show <name>|starter [--write]]`

Lists your recipes, shows one as TOML with what it runs on this OS, or previews/installs the starter pack.

### `thine import [rc-file…] [--write|--toml]`

Reads your `.bashrc`/`.zshrc` (and alias files), proposes a typed recipe for each alias and function, flags ones thine already replaces (`killport` → `thine kill`), skips ones that must stay in your shell (`source`, `cd`, `export`), and warns about aliases that hide real programs (`alias gh=…` hides the GitHub CLI). `--toml` prints the suggestions; `--write` appends them after confirmation (backed up, validated, rolled back if invalid).

---

## Look & feel

| Command | What it does |
|---|---|
| `thine` / `thine hello` | The animated splash, welcome box, live status and a tip |
| `thine hello --greeting` | The instant one-liner used by the opt-in `[shell] greeting` |
| `thine theme ls` | Every palette with a gradient wordmark and swatches |
| `thine theme preview [name]` | A sample screen in one theme (or all, animated) |
| `thine theme set <name>` | Save `ui.theme` in your config |
| `thine theme terminal` | How to theme the whole terminal (Gogh, iTerm2-Color-Schemes, Catppuccin…) |
| `thine fonts` | Recommended fonts: ligatures, Nerd Font icons, cursive italics, and what's installed |
| `thine fonts install <font>` | Download a pinned Nerd Fonts release, verify SHA-256, install for your user, print terminal setup steps |
| `thine fonts test` | A specimen: italics, ligatures, icons, emoji, box drawing, gradient |
| `thine fx [splash\|spinner\|wave\|confetti\|rain\|fireworks\|all]` | Play animations (`--seconds`) |

---

## Setup & health

| Command | What it does |
|---|---|
| `thine setup [--shell s…] [--all] [--uninstall] [-y]` | Add (or remove) the managed block in your rc files, with a diff, a backup and completions |
| `thine init <bash\|zsh\|fish>` | Print the shell integration (what the rc block evaluates) |
| `thine doctor` | Health check: install, shell wiring, **shadowed shortcuts with file:line**, colours, fonts, config, state, companions, agents |
| `thine bench [-n runs]` | Measure cold start, `init` and the sourcing delta (p50/p95) |
| `thine config path\|init\|validate\|edit\|show` | Locate, create, check, edit, or print the effective configuration |
| `thine trust [path]` · `trust ls` · `untrust` | Review and pin a project's `.thinerc.toml` |
| `thine notify [message]` | Desktop notification (`--from claude\|codex` for agent hooks, `--test`) |
| `thine ai doctor` · `thine ai hooks` | Installed agents, hook status, context files · ready-to-paste hook config |
| `thine version [--json]` | Version, commit, build date, Go version, target |

---

## Exit codes

| Code | Meaning | Typical `error.code` in `--json` |
|---|---|---|
| 0 | Desired end state reached (including "nothing to do") | — |
| 1 | Unexpected failure (OS, tool, bug) | `E_RESOLVER`, `E_IO`, `E_EXTERNAL_TOOL` |
| 2 | Usage or config error | `E_USAGE`, `E_CONFIG_INVALID` |
| 3 | Not found (group, recipe, plan, project) | `E_NOT_FOUND` |
| 4 | Partial — some items failed/skipped, or a port is still held | `E_PARTIAL` |
| 5 | Refused by the safety policy | `E_PROTECTED`, `E_AGENT_POLICY`, `E_ROOT` |
| 6 | Needs approval — the plan was printed; re-run with `--apply` or `thine apply <id>` | `E_NEEDS_APPROVAL` |
| 7 | Another thine operation holds the state lock | `E_LOCKED` |
| 130 | Interrupted (Ctrl-C) | — |

## JSON contracts

Every `--json` document carries `"schema_version": 1`. Additions are backwards-compatible; removals or renames will bump the schema version and be called out in the [changelog](../CHANGELOG.md). Excerpts:

```json
// thine kill --group fb --json
{ "schema_version": 1, "op_id": "01M5ATKV…", "mode": "applied",
  "requested_ports": [3000, 3001], "ports_free": [3000, 3001], "ports_held": [],
  "targets": [ { "port": 3000, "pid": 4123, "start_time": 46153232, "name": "node", "tier": "dev",
                 "group_members": [4101, 4123], "result": "terminated", "signal": "TERM", "elapsed_ms": 312 } ],
  "exit_code": 0 }
```

```json
// thine cleanup --json   (exit 6: plan saved, nothing changed)
{ "schema_version": 1, "plan_id": "01M5AST1…", "mode": "dry-run",
  "totals": { "frees_now": 8589934592, "frees_after_retention": 9771089920, "not_reclaimable": 0 },
  "items": [ { "id": "npm", "category": "npm", "risk": "regenerable", "action": "exec",
               "argv": ["npm", "cache", "clean", "--force"], "bytes": 8589934592, "undoable": false } ],
  "next": "thine apply 01M5AST1…" }
```

```json
// any error
{ "schema_version": 1, "error": { "code": "E_NOT_FOUND", "message": "no port group named \"db\"", "hint": "define it under [groups.db] …" }, "exit_code": 3 }
```
