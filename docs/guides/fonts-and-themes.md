# 🎨 Fonts, italics and colour themes

thine is built to look its best in a modern terminal: truecolor gradients, Nerd Font icons, and *italic* hints that turn hand-written in a font with a cursive italic. This guide gets your terminal there in a few minutes — and explains where the community's best colour schemes live.

## 1 · Check what you have

```sh
thine fonts test
```

The specimen shows regular, bold, *italic* and ***bold italic*** text, ligatures (`=> != === <=`), Nerd Font icons, emoji, box drawing, a gradient and the current palette — plus the colour depth thine detected. Read it like this:

| You see | Meaning | Fix |
|---|---|---|
| Italic line looks upright | Your font (or terminal) has no italic | Install a font with italics (below), enable italics in the terminal profile |
| `=>` stays two characters | No ligatures | Use a ligature font; in iTerm2 tick *Use ligatures*; VS Code: `terminal.integrated.fontLigatures.enabled` |
| Boxes instead of icons | No Nerd Font | `thine fonts install …`, or keep `icons = "emoji"` |
| Gradient is banded/blocky | 256 or 16 colours detected | Use a truecolor terminal, or `export COLORTERM=truecolor` if yours supports it |

## 2 · Pick a font

```sh
thine fonts
```

| Font | Italics | Why you'd pick it |
|---|---|---|
| **Victor Mono** (`victor-mono`) | **cursive** | Slender, very readable, and the most hand-written italic — thine's hints look like margin notes |
| **Cascadia Code** (`cascadia-code`, *CaskaydiaCove*) | **cursive** | Microsoft's terminal font with a cursive italic and generous ligatures |
| **Monaspace Radon** (`monaspace`) | **cursive** | GitHub's superfamily; Radon is the handwritten member, with texture healing |
| **JetBrains Mono** (`jetbrains-mono`) | true italic | Tall x-height, crisp at small sizes |
| **Fira Code** (`fira-code`) | none | The classic ligature font. It has no italic — pair it with **Fira Code iScript** (a community variant that adds a script italic) if you love Fira |

Install one (it's thine's **only** network call, and it's explicit):

```sh
thine fonts install victor-mono
```

thine downloads the pinned [Nerd Fonts](https://github.com/ryanoasis/nerd-fonts) v3.5.1 archive, **verifies its SHA-256** against the digest GitHub publishes, unpacks it into `~/.local/share/fonts/thine/<font>` (Linux, then `fc-cache`) or `~/Library/Fonts` (macOS) — no sudo — and then prints the exact steps for *your* terminal:

| Terminal | How |
|---|---|
| GNOME Terminal / VTE | Preferences → profile → *Custom font*, or the printed `gsettings` commands |
| kitty | `font_family VictorMono Nerd Font` + `italic_font auto` in `kitty.conf`, then `ctrl+shift+F5` |
| Ghostty | `font-family = "VictorMono Nerd Font"` in `~/.config/ghostty/config` |
| WezTerm | `config.font = wezterm.font("VictorMono Nerd Font")` |
| Alacritty | `[font.normal] family = "VictorMono Nerd Font"` in `alacritty.toml` |
| iTerm2 | Settings → Profiles → Text → Font; tick *Use ligatures* |
| VS Code terminal | `"terminal.integrated.fontFamily": "VictorMono Nerd Font"` |
| Konsole | Edit Current Profile → Appearance → Font |

Then turn on glyph icons:

```toml
[ui]
icons = "nerd"     # emoji (default) | nerd | ascii
```

## 3 · Pick a thine theme

```sh
thine theme ls                    # every palette with a gradient wordmark and swatches
thine theme preview               # a sample screen in each, one after another
thine theme preview tokyo-night
thine theme set tokyo-night       # saved as ui.theme
thine --theme gruvbox ports       # one run only (or THINE_THEME=gruvbox)
```

| Theme | Feel |
|---|---|
| `thine` (Thine Aurora) | Violet → pink → amber; the default |
| `catppuccin-mocha` · `catppuccin-latte` | Soft pastels, dark · light |
| `tokyo-night` | Neon blues and magentas |
| `dracula` | High-contrast purple and pink |
| `nord` | Calm arctic blues |
| `gruvbox` | Warm retro earth tones |
| `rose-pine` | Muted, elegant rosé |
| `synthwave` | '84 neon on purple |

Themes only change hues — green still means done, yellow needs attention, red is refused. With `NO_COLOR` or `--plain`, the same information is carried by words and icons.

## 4 · Theme the whole terminal

thine draws in truecolor, so it looks right on any background. For the rest of your terminal — `ls`, `git`, your editor — install a matching **16-colour scheme**. The community keeps excellent collections:

| Collection | What it is | How you use it |
|---|---|---|
| [**Gogh**](https://github.com/Gogh-Co/Gogh) | Hundreds of schemes with an interactive installer for Linux terminals (GNOME Terminal, Xfce Terminal, Tilix, Konsole, kitty…); also usable from macOS | Run its installer, pick schemes by number; they appear as new terminal profiles |
| [**iTerm2-Color-Schemes**](https://github.com/mbadolato/iTerm2-Color-Schemes) | The largest collection, exported for iTerm2, kitty, Alacritty, WezTerm, Ghostty, Windows Terminal, Konsole, Xresources and more | Download the file for your terminal from its folder and import it (iTerm2: *Color Presets → Import*) |
| [**Catppuccin**](https://github.com/catppuccin/catppuccin) | One palette, official ports for nearly every app — `catppuccin/iterm`, `/kitty`, `/alacritty`, `/gnome-terminal`, `/wezterm`… | Follow the port's README; then `thine theme set catppuccin-mocha` |
| Dracula · Nord · Tokyo Night · Rosé Pine · Gruvbox | Each project publishes official terminal ports | Install the port, then `thine theme set` the matching name |

Some terminals have it built in — `thine theme terminal` detects yours and prints the shortcut:

```sh
kitty +kitten themes                            # kitty: browse and apply
ghostty +list-themes                            # Ghostty: then theme = "catppuccin-mocha"
config.color_scheme = "Catppuccin Mocha"        # WezTerm ships iTerm2-Color-Schemes
```

## 5 · Animations

thine animates the splash, spinners (the twinkling ✦ star), grace-period bars, memory gauges, the dashboard reveal and celebrations — always on stderr, at 12 fps, never at shell startup.

```sh
thine fx all                  # the whole show
thine fx fireworks --seconds 5
```

Turn them down or off any time: `ui.animations = false`, `ui.splash = false`, `--no-anim`, `THINE_NO_ANIM=1`, `NO_COLOR`, or `--plain`. They switch off automatically in pipes, CI and `--json`.
