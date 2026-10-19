# 🩺 Troubleshooting and FAQ

Start here:

```sh
thine doctor
```

It checks the install, shell wiring, shadowed shortcuts, colours and fonts, config and trust, the state directory, companions and agents — and prints a fix next to every problem.

## My shortcut doesn't do what thine says

`thine doctor` → **Shell integration → shortcuts** lists every name you already define, with file and line. Your definition wins on purpose. Delete yours, rename thine's (`[shortcuts] rename = { kp = "kport" }`), or force thine's (`force = ["kp"]`). Also make sure thine's block is **last** in your rc file — `thine setup` moves it there.

## `thine: command not found` / shortcuts call an old binary

`doctor` → **Install → PATH**. Install with `make install` or the installer (both use `~/.local/bin`), and make sure that directory is on your `PATH`.

## `killfb` says a port is still held

Run `whoport <port>`. Typical causes:

- **Another user's process** (often a system service) — thine can't see its PID without root. Use `thine kill <port> --sudo` or stop the service (`sudo systemctl stop …`).
- **Docker** — the badge says *docker*; use `thine kill <port> --docker`.
- **A non-dev process ignored SIGTERM** — thine doesn't SIGKILL databases or unknown processes automatically. Add `--force` if you're sure, or add the name to `[kill] dev_names`.
- **Something restarts it** — a supervisor (systemd, pm2, nodemon, `docker compose` restart policy). Stop the supervisor, or use a group so job scope stops the runner too.

## "refused" — why won't thine stop this?

It's protected: PID 1, your own shell/terminal/editor, a desktop or system daemon, or sshd carrying your session. This is deliberate and not configurable.

## `cleanup` frees less than the plan's "after retention" number

Quarantined items (stale `node_modules`, installers) are only *moved* — they free their space when the retention period ends and a later `cleanup --apply` purges them, or immediately with `thine trash purge`. Caches show up under **frees now**.

## Why doesn't `freeup` free as much RAM as my old alias?

Your old alias dropped the page cache. That cache is memory the kernel already gives back instantly when an application asks — it was effectively available. Dropping it makes "free" jump, barely moves "available", and makes your next file reads slower while the cache refills. `freeup` shows **available** memory and frees *real* memory by stopping idle dev servers and language servers, then reports the measured change. Try `freeup --drop-caches` once to see both numbers side by side.

## I deleted something by mistake

```sh
thine history       # find the operation
thine trash ls      # is it still in quarantine?
thine undo          # restore the latest — or `thine undo <id>`
```

Process stops can't be undone (start the server again); cache cleanups regenerate on demand.

## Everything is grey / no animations

Colour and animation turn off for `NO_COLOR`, `TERM=dumb`, CI, pipes, `--plain`, `--json`, `--no-anim`, `THINE_NO_ANIM` and `ui.animations = false`. If your terminal supports 24-bit colour but thine shows 256, `export COLORTERM=truecolor`.

## Icons show as boxes

You chose `icons = "nerd"` without a Nerd Font. `thine fonts install victor-mono` (then set the terminal's font), or go back to `icons = "emoji"`.

## The project's recipes don't show up

The project's `.thinerc.toml` isn't trusted, or it changed since you trusted it. You'll see a 🔒 notice. Review it and run `thine trust`.

## It says "agent mode" but I'm a human

You're running thine inside an AI agent's terminal (Claude Code sets `CLAUDECODE`), or `THINE_AGENT=1` is exported. Risky actions only plan there by design. Run the command in your own terminal, or apply the saved plan with `thine apply <id>`.

## Is thine slowing my shell down?

```sh
thine bench
```

Typical: ≈ 2 ms to generate init and ≈ 3 ms added per new shell — and nothing per command, because thine installs no hooks.

## Reporting a bug

Use the [bug report form](https://github.com/YourAKShaw/thinerc/issues/new/choose) with `thine doctor --json`. If thine deleted, moved or killed something it shouldn't have, tick the safety box — those reports block releases. Security issues go through the [private form](../../.github/SECURITY.md).
