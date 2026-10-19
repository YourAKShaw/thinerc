# 🤖 thine with AI coding agents

thine is vendor-neutral glue for the agents you already use — Claude Code, Codex CLI, Gemini CLI, Copilot CLI, Grok Build, OpenCode, Aider. It never calls a model and ships no chat client; it makes your agents *safer and easier to live with*.

## See what you have

```sh
thine ai doctor
```

Lists each agent, its version, whether thine's notification hook is wired in, and this repository's context files (`AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `.github/copilot-instructions.md`, `.cursorrules`). `thine whatsup` also shows which agents are running right now and in which directories.

> 💡 Claude Code reads `CLAUDE.md`, not `AGENTS.md`. If you keep one canonical `AGENTS.md`, a one-line `CLAUDE.md` containing `@AGENTS.md` imports it.

## Get notified when an agent finishes or needs you

```sh
thine ai hooks
```

prints paste-ready configuration:

**Claude Code** — merge into `~/.claude/settings.json` (or add them via `/hooks` inside Claude Code):

```json
{
  "hooks": {
    "Stop":         [{ "hooks": [{ "type": "command", "command": "thine notify --from claude --event stop" }] }],
    "Notification": [{ "hooks": [{ "type": "command", "command": "thine notify --from claude --event attention" }] }]
  }
}
```

**Codex CLI** — at the **top** of `~/.codex/config.toml`, before any `[table]` (Codex reads only one top-level `notify`):

```toml
notify = ["thine", "notify", "--from", "codex"]
```

Test it: `thine notify --test`.

What you get: a desktop notification titled `Claude Code · shop (feat/cart)` with "finished" or "needs your input". Under the hood thine reads the hook's JSON (capped at 64 KiB), keeps only the event, sanitizes every string, coalesces duplicates within 3 seconds, and passes text to `notify-send` / `osascript` / a WSL toast **as arguments, never as script text** — so a prompt-injected message can't execute anything. As a hook it always exits 0, so it can never block your agent.

## Agent mode: safe verbs for non-human callers

Agents call shell commands, and they'll happily call thine's. When thine sees `CLAUDECODE`, `CODEX_SANDBOX`, `GEMINI_CLI`, `CURSOR_AGENT` or `THINE_AGENT=1`:

| Verb | What an agent gets |
|---|---|
| `port`, `ports`, `whatsup`, `doctor`, `freeup` (report), `history`, `--json` anything | Full answers |
| `kill` on configured groups (`killfb`) | Works for dev processes — agents restarting dev servers is a common, safe job |
| `kill` of anything else, `cleanup`, `freeup --stop`, `nm reset`, non-read-only recipes | **A plan only** (exit 6). Cleanup plans are saved |
| `apply`, `trust`, `trash purge` | Refused — these are human decisions |

So the conversation goes like this:

```text
agent:  I ran `thine cleanup --json` — it found 9.1 GiB (npm cache, 4 stale node_modules).
        Apply it with:  thine apply 01M5AST1TN3MSDDM9KS333N45R
you:    thine apply 01M5AST1TN3MSDDM9KS333N45R      ← you see the plan, every item is re-checked, then applied
```

Tune it in your **global** config (a repository can never loosen this):

```toml
[agents.policy]
kill    = "allow-groups"   # allow-groups | plan | deny
cleanup = "plan"
freeup  = "plan"
recipes = "plan"           # "allow" lets agents run non-read-only recipes
```

> Agent mode is defence-in-depth, not a sandbox. A compromised agent can bypass thine and run `rm` directly; your agent's own sandbox and permission system is the real boundary. thine's job is to make the safe path the easy one.

## Useful prompts

- *"Use `thine ports --json` to find what's on 3000, then stop it with `thine kill 3000`."*
- *"Run `thine whatsup --json` and tell me if memory pressure explains the slow build."*
- *"Plan a disk cleanup with `thine cleanup --json` and give me the apply command — don't apply it."*
- *"Add a recipe to `.thinerc.toml` for our db reset with risk = destructive."*

## Usage and cost

thine doesn't track tokens or spend. For Claude Code, [ccusage](https://www.npmjs.com/package/ccusage) does this well — `thine ai doctor` points to it when it's installed.
