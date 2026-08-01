---
title: Keybindings
description: Keyboard shortcuts and customization.
---

All keybindings are customizable via `keybindings.json`. A complete example is available at [`config/keybindings.json`](https://github.com/eugenioenko/ttt/blob/main/config/keybindings.json) in the repository. TTT supports chord sequences (e.g. `ctrl+k ctrl+t`) for two-step shortcuts.

You can open your keybindings file directly from the command palette (**Ctrl+P**) with **Preferences: Open Keyboard Shortcuts**, or press **Ctrl+K Y**.

## Default Keybindings

### General

| Shortcut | Action |
|----------|--------|
| Ctrl+Q | Quit |
| Ctrl+P | Command palette |
| Ctrl+K P | Quick open file |
| Ctrl+O | Open folder / workspace |
| Escape | Focus editor |

### File

| Shortcut | Action |
|----------|--------|
| Ctrl+N | New file |
| Ctrl+S | Save |
| Ctrl+K S | Save as |

### Editor

| Shortcut | Action |
|----------|--------|
| Ctrl+Z | Undo |
| Ctrl+Y | Redo |
| Ctrl+A | Select all |
| Ctrl+C | Copy |
| Ctrl+X | Cut |
| Ctrl+V | Paste |
| Ctrl+G | Go to line |
| Ctrl+/ | Toggle comment |
| Ctrl+K J | Join lines |
| Ctrl+K K | Delete line |
| Ctrl+K O | Sort lines ascending |
| Ctrl+K M | Go to matching bracket |
| Ctrl+Enter | Insert line below |
| Alt+Up | Move line up |
| Alt+Down | Move line down |

### Word Navigation

| Shortcut | Action |
|----------|--------|
| Ctrl+Left | Move word left |
| Ctrl+Right | Move word right |
| Ctrl+Shift+Left | Select word left |
| Ctrl+Shift+Right | Select word right |
| Alt+Backspace | Delete word left |
| Alt+Delete | Delete word right |
| Ctrl+Delete | Delete word right |

### Code Folding

| Shortcut | Action |
|----------|--------|
| Ctrl+K [ | Toggle fold |
| Ctrl+K 0 | Collapse all folds |
| Ctrl+K 9 | Expand all folds |

### Multi-Cursor

| Shortcut | Action |
|----------|--------|
| Ctrl+D | Select next occurrence |
| Ctrl+K L | Select all occurrences |
| Alt+Click | Add cursor at click position |
| Ctrl+K U | Undo last cursor addition |
| Escape | Collapse to single cursor |

### Search

| Shortcut | Action |
|----------|--------|
| Ctrl+F | Find |
| Ctrl+R | Find and replace |
| F3 / Shift+F3 | Find next / previous |

### View

| Shortcut | Action |
|----------|--------|
| Ctrl+B | Toggle sidebar |
| Ctrl+K B | Toggle bottom panel |
| Alt+Shift+M | Show/hide the menu bar |
| Ctrl+K E | Show file explorer |
| Ctrl+K F | Search across files |
| Ctrl+K R | Search and replace across files |
| Ctrl+K C | Show changes |
| Ctrl+0 | Focus sidebar |
| Ctrl+K Y | Open keyboard shortcuts |
| Alt+Shift+Up | Panel taller |
| Alt+Shift+Down | Panel shorter |
| Alt+Shift+Left | Sidebar narrower |
| Alt+Shift+Right | Sidebar wider |

### Tabs

| Shortcut | Action |
|----------|--------|
| Alt+. / Alt+, | Next / previous tab |
| Ctrl+W | Close tab |

### LSP

| Shortcut | Action |
|----------|--------|
| Ctrl+U | Autocomplete |
| F12 | Go to definition |
| Shift+F12 | Go to implementation |
| F2 | Rename symbol |
| Ctrl+K I | Hover info |
| Ctrl+L F | Format document (LSP) |
| Ctrl+L E | Format document (external formatter) |
| Ctrl+L S | Format selection |
| Ctrl+L O | Organize imports |
| Ctrl+L X | Fix all |
| Ctrl+L R | Find references |
| Ctrl+L I | Go to implementation |
| Ctrl+L T | Go to type definition |

### Terminal

| Shortcut | Action |
|----------|--------|
| Ctrl+T | Toggle terminal |
| Alt+T | Terminal fullscreen |
| Ctrl+K T | New terminal tab |

### Changes Panel

| Shortcut | Action |
|----------|--------|
| Space | Toggle stage/unstage file |
| A | Stage all |
| U | Unstage all |
| R | Refresh |
| Enter | Open diff / activate |

### Menu Bar

| Shortcut | Action |
|----------|--------|
| F10 / Alt+F | File menu |
| Alt+E | Edit menu |
| Alt+S | Selection menu |
| Alt+V | View menu |
| Alt+O | Options menu |
| Alt+H | Help menu |
| Alt+Shift+M | Show/hide the menu bar |

The menu shortcuts above keep working while the menu bar is hidden — the dropdown opens on its own from the top edge.

## Customizing Keybindings

Create a `keybindings.json` in `~/.config/ttt/` to override defaults. The format follows VS Code conventions:

```json
[
  { "key": "ctrl+d", "command": "editor.goToDefinition" },
  { "key": "ctrl+k ctrl+s", "command": "file.saveAs" }
]
```

### Chord Sequences

TTT supports two-step chord sequences. Press the first key combination, then the second:

- `ctrl+k e` means press Ctrl+K, release, then press E
- `ctrl+l f` means press Ctrl+L, release, then press F

### Keyboard Shortcuts Editor

Use **View > Keyboard Shortcuts** from the menu bar (or the command palette) to browse and edit keybindings. The editor shows a searchable list of all commands with their current shortcuts. Select a command to see action buttons:

- **Edit** - record a new key combination for the command
- **Reset** - restore the default keybinding
- **Clear** - remove the keybinding

Changes are saved to `keybindings.json` and take effect immediately.
