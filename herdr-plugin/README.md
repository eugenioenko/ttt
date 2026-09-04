# TTT Editor — Herdr Plugin

A [Herdr](https://herdr.dev) plugin for [TTT Editor](https://tttedit.dev), a full IDE that runs in your terminal. Opens TTT as a first-class pane inside your Herdr workspace.

## Install

```sh
herdr plugin install eugenioenko/ttt/herdr-plugin
```

This installs the `ttt` binary automatically if it's not already on your PATH.

To link a local copy for development instead:

```sh
herdr plugin link /path/to/ttt/herdr-plugin
```

## What it does

### Editor pane

Opens TTT in a new Herdr tab, pointed at the active worktree or workspace directory.

```sh
herdr plugin pane open --plugin ttt.editor --entrypoint editor
```

### Actions

| Action | Description |
|--------|-------------|
| `ttt.editor.open` | Open TTT in the current workspace |
| `ttt.editor.open-worktree` | Open TTT in the active worktree |

Invoke an action from the CLI:

```sh
herdr plugin action invoke ttt.editor.open
```

### Directory resolution

When opening, the plugin resolves the target directory in this order:

1. `checkout_path` — the active worktree's checkout directory
2. `focused_pane_cwd` — the working directory of the focused pane
3. `workspace_cwd` — the workspace root directory
4. `.` — fallback to current directory

## Keybinding

Add this to your Herdr `config.toml` to open TTT with `Ctrl+b e` (or your custom prefix key followed by `e`):

```toml
[[keys.command]]
key = "prefix+e"
type = "shell"
command = "herdr plugin pane open --plugin ttt.editor --entrypoint editor"
description = "open TTT editor"
```

## Requirements

- [Herdr](https://herdr.dev) 0.7.0 or newer
- Linux or macOS
- [Git](https://git-scm.com/) and [ripgrep](https://github.com/BurntSushi/ripgrep) (`rg`) for TTT's full feature set

## Links

- [TTT Editor](https://tttedit.dev) — documentation and features
- [TTT on GitHub](https://github.com/eugenioenko/ttt) — source code and releases
- [Herdr plugin docs](https://herdr.dev/docs/plugins) — plugin authoring reference

## License

Same as [TTT Editor](https://github.com/eugenioenko/ttt/blob/main/LICENSE).
