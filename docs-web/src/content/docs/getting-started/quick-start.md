---
title: Quick Start
description: Get up and running with TTT in minutes.
sidebar:
  order: 3
---

## Opening Files and Folders

```sh
ttt                             # opens the current directory
ttt .                           # also opens the current directory
ttt /path/to/dir                # opens that directory as the workspace
ttt /path/to/file.go            # opens the file
ttt dir1 dir2                   # opens multiple folders
ttt --workspace project.ttt     # loads a saved workspace file
ttt https://github.com/owner/repo/pull/123  # review a PR
```

## Basic Navigation

- **Ctrl+P** opens the command palette
- **Ctrl+K P** opens quick file open
- **Ctrl+B** toggles the sidebar
- **Ctrl+O** opens a folder
- **Ctrl+T** toggles the terminal (half screen)
- **Alt+T** toggles the terminal fullscreen
- **Ctrl+G** opens Go to Line

## Editing

- **Ctrl+Z / Ctrl+Y** for undo/redo
- **Ctrl+C / Ctrl+X / Ctrl+V** for copy/cut/paste
- **Ctrl+F** to find, **Ctrl+R** to find and replace
- **Ctrl+A** to select all
- **Ctrl+D** to select the next occurrence (multi-cursor)
- **Alt+Click** to add cursors at multiple positions

## Tabs

- **Alt+.** / **Alt+,** to switch tabs
- **Ctrl+W** to close a tab
- Opening a file replaces the current unpinned tab
- Clicking an already-open tab pins it

## Command Palette

Press **Ctrl+P** to open the command palette. By default it searches files. Type a `>` prefix to switch to command mode, which lists all available editor commands. Type `?` to browse orientation topics for the workspace, panels, navigation, changes, terminal, and keybinding chords; continue typing to search the help entries, or use `>` when you want the complete command list.

## Configuration

Config files live in `~/.config/ttt/`:

| File | Purpose |
|------|---------|
| [`settings.json`](https://github.com/eugenioenko/ttt/blob/main/config/settings.json) | Editor settings |
| [`keybindings.json`](https://github.com/eugenioenko/ttt/blob/main/config/keybindings.json) | Custom keybindings |
| `themes/*.json` | Custom themes |

You can also open these from the command palette (**Ctrl+P**): **Settings: Open Editor Settings** (also **Ctrl+K ,**), **Settings: Open settings.json** and **Settings: Open keybindings.json**.
