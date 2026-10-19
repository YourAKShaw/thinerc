# 🚀 Getting started

Five minutes from nothing to `killfb && pnpm dev`.

## 1 · Install

**With the installer** (Linux and macOS, x64 and arm64 — verifies the SHA-256, no sudo):

```sh
curl -fsSL https://raw.githubusercontent.com/YourAKShaw/.thinerc/main/scripts/install.sh | sh
```

**From source** (Go 1.27+):

```sh
git clone https://github.com/YourAKShaw/.thinerc.git && cd .thinerc
make install            # → ~/.local/bin/thine
```

Make sure `~/.local/bin` is on your `PATH`.

## 2 · Wire it into your shell

```sh
thine setup             # diff → confirm → one marked block at the end of your rc file (+ completions)
exec $SHELL
```

## 3 · Say hello

```sh
thine
```

## 4 · Try the verbs

```sh
whoport 3000            # who's listening?
ports                   # all your dev ports
kp 3000                 # free it, gracefully
killfb                  # free 3000 + 3001
freeup                  # honest memory report
cleanup                 # disk plan — nothing happens until you say yes
whatsup                 # the dashboard
thine fx all            # just for fun
```

## 5 · Check your setup

```sh
thine doctor
```

## Next

- [The power-user handbook](power-user.md) — everything, level by level
- [Fonts and themes](fonts-and-themes.md) — italics, Nerd Fonts, colour schemes
- [Recipes cookbook](recipes-cookbook.md) — your own verbs
- [Migrating from aliases](migrating-from-aliases.md)
