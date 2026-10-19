# Getting help

Start with the [README](../README.md) for installation and the quick tour, then the [docs](../docs/README.md) — especially the [power-user guide](../docs/guides/power-user.md), [configuration](../docs/configuration.md) and the [troubleshooting guide](../docs/guides/troubleshooting.md). See [CONTRIBUTING.md](CONTRIBUTING.md) for making changes.

Before opening an issue, run:

```sh
thine doctor
```

It checks the installed binary, your shell wiring (and which of your own aliases shadow thine's shortcuts), terminal colour and fonts, config and trust state, the state directory, companion tools and AI agents — and usually points at the fix. `thine bench` measures thine's startup cost on your machine, and `thine history` shows everything thine has changed.

Search [existing issues](https://github.com/YourAKShaw/.thinerc/issues) first. Use the bug, feature or documentation form when one fits. For a usage question, open a blank issue with what you ran, what you expected, what happened, and the output of `thine doctor --json`.

Share only sanitized examples. Do not post tokens, private paths or process command lines you cannot share publicly (thine sanitizes and redacts its own logs, but your terminal scrollback may not be). Report vulnerabilities privately as described in the [security policy](SECURITY.md).

If thine removed something you need, don't panic: `thine trash ls` lists what's quarantined and `thine undo` restores it.
