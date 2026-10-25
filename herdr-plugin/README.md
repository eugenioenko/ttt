# TTT Editor — Herdr Plugin

A [Herdr](https://herdr.dev) plugin for [TTT Editor](https://tttedit.dev). Opens TTT as a first-class pane inside your Herdr workspace.

## About TTT

TTT (Terminal Text Tool) is a real alternative to VS Code, Zed, and Sublime that runs entirely in your terminal. Single Go binary, zero config.

- **GUI in your terminal** — menus, dialogs, mouse support, right-click context menus, and a file explorer
- **LSP intelligence** — completions, diagnostics, hover info, rename, references, and formatting for 23+ languages
- **Git built in** — stage, commit, push, inline diffs, blame, and GitHub PR review directly from the terminal
- **Integrated terminal** — full terminal emulator with true color rendering and multiple tabs
- **Multi-cursor and search** — Ctrl+D selections, Alt+Click, and ripgrep-powered workspace search
- **Workspaces** — multi-folder projects, quick open, tabs, and saveable layouts

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

Herdr's own flags override the manifest for that call: `--placement split --direction
right --target-pane w1:p3` puts the editor beside a specific pane instead.

The pane routes right-click to TTT, so its context menus work. Right-clicking the pane frame still opens Herdr's menu.

### Configuration

`~/.config/herdr/plugins/config/ttt.editor/config.toml`, which `herdr plugin config-dir ttt.editor` prints:

```toml
open_placement = "split"  # split | overlay | zoomed | tab
open_direction = "right"  # right | down, for split and zoomed
```

Both keys are optional. With neither, the plugin asks Herdr for nothing and the manifest
placement decides, which is `tab`. `open_direction` needs `open_placement` set to `split`
or `zoomed`. The values are the ones the reviewr plugin takes for `toggle_placement` and
`toggle_direction`; the key names follow ttt's `open` action. An unknown key, an unquoted
value, or a value outside the list fails the action rather than quietly keeping the
default, and `herdr plugin log list --plugin ttt.editor` says which.

Only the actions read this file, because they re-open the pane through Herdr and can
name the placement. A `type = "shell"` keybinding that calls `plugin pane open` itself
always gets the manifest placement.

### Actions

| Action | Description |
|--------|-------------|
| `ttt.editor.open` | Open TTT in the current workspace |
| `ttt.editor.open-worktree` | Open TTT in the active worktree |

Invoke an action from the CLI:

```sh
herdr plugin action invoke ttt.editor.open
```

Actions run headless, so they open the editor by delegating to the `editor`
pane entrypoint rather than launching the TUI in the action process.

### Directory resolution

When opening, the plugin resolves the target directory in this order:

1. `checkout_path` — the active worktree's checkout directory
2. `focused_pane_cwd` — the working directory of the focused pane
3. `workspace_cwd` — the workspace root directory
4. `.` — fallback to current directory

## Keybinding

Add this to your Herdr `config.toml` to open TTT with `Ctrl+b e` (or your custom prefix key followed by `e`), honouring the `open_placement` above:

```toml
[[keys.command]]
key = "prefix+e"
type = "plugin_action"
command = "ttt.editor.open"
description = "open TTT editor"
```

The `type = "shell"` form opens the pane through Herdr directly and so keeps the manifest
placement, whichever way the plugin's defaults change:

```toml
[[keys.command]]
key = "prefix+e"
type = "shell"
command = "herdr plugin pane open --plugin ttt.editor --entrypoint editor"
description = "open TTT editor"
```

## Requirements

- [Herdr](https://herdr.dev) 0.8.2 or newer — the plugin routes right-click to TTT with
  `herdr pane input --right-click pane`, which shipped in 0.8.2
- Linux or macOS
- [Git](https://git-scm.com/) and [ripgrep](https://github.com/BurntSushi/ripgrep) (`rg`) for TTT's full feature set

## Links

- [TTT Editor](https://tttedit.dev) — documentation and features
- [TTT on GitHub](https://github.com/eugenioenko/ttt) — source code and releases
- [Herdr plugin docs](https://herdr.dev/docs/plugins) — plugin authoring reference

## License

Same as [TTT Editor](https://github.com/eugenioenko/ttt/blob/main/LICENSE).
