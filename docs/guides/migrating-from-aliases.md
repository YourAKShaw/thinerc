# 🔁 Migrating from your aliases

Most `.bashrc`/`.zshrc` files grow the same helpers. Here's how a typical one maps onto thine — and what improves.

| What you have today | thine | What improves |
|---|---|---|
| `killport() { kill -9 $(lsof -t -i:$1); }` | `kp 3000` | Shows what it stops; SIGTERM first, SIGKILL only for dev processes; refuses your shell, PID 1 and system daemons; never hits a recycled PID; Docker-aware; `kp 3000 && pnpm dev` chains reliably |
| `kill -9 $(lsof -i tcp:3000 \| sed -n 2p \| awk '{print $2}')` | `kp 3000` | Same — and the `sed -n 2p` bug (only the first PID) is gone |
| `killfb` (3000 + 3001) | `killfb` | Stops the whole dev *job* (runner + child), per-project ports via `.thinerc.toml` |
| `lsof -i tcp:$1` | `whoport 3000` | Readable: process, command, cwd, uptime, safety tier; `--json` for scripts |
| `alias freeup='sudo … drop_caches'` | `freeup` | Shows *available* memory and pressure; frees real memory by stopping idle dev and language servers; cache drop is opt-in and explained |
| `alias availram="free -h && swapon --show"` | `freeup` / `whatsup` | Available, pressure, swap and the top consumers grouped by app |
| `alias rameaters="ps -eo pid,comm,rss --sort=-rss \| head -20"` | `freeup` | PSS instead of RSS (no double counting); grouped (Chrome ×33 is one line) |
| `sudo apt clean && … && rm -rf ~/.cache/thumbnails/*` | `cleanup` (`--include system`) | A plan with totals first; each tool's own clean command; quarantine and `thine undo` for your files |
| `rm -rf node_modules && npm i` | `nuke-nm` | Detects the package manager from the lockfile; rolls back automatically if the install fails |
| `alias gpa="git push -u origin HEAD"` and friends | recipes (`thine import`) | Typed, risk-labelled, cross-platform, shareable, previewed when risky |

## Do it

```sh
thine doctor             # 1. see which of your definitions shadow thine's shortcuts
thine import             # 2. see your aliases as recipes
thine import --write     # 3. keep the ones you like
```

Then decide about the overlaps `doctor` reported. Either delete your old `killport()` / `freeup` from your rc file, or keep them and let thine stay out of the way, or force thine's versions:

```toml
[shortcuts]
force = ["killport", "freeup"]
```

## What `thine import` won't convert (on purpose)

- Aliases that change your **current shell** — `source ~/.zshrc`, `cd …`, `export …`, `declare -f` — can't work as recipes, because recipes run in a child process. Keep them as aliases.
- Helpers thine already replaces are marked **built in** with the replacement.
- Names that aren't valid recipe names are skipped with a reason.

It will also warn you when an alias hides a real program — for example `alias gh="git log --oneline"` hides the GitHub CLI at `/usr/bin/gh`.
