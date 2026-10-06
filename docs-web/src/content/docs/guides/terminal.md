---
title: Integrated Terminal
description: Built-in terminal emulator in TTT.
---

TTT includes a built-in terminal emulator. Press **Ctrl+T** to toggle the terminal panel.

## Usage

- **Ctrl+T** to toggle the terminal (half screen)
- **Alt+T** to toggle the terminal fullscreen
- **Ctrl+K T** to spawn a new terminal tab
- Multiple terminal tabs with a vertical inner tab bar on the left edge
- Close all terminals from the panel actions menu

## Features

- Full VT escape sequence support via `eugenioenko/xterm-go` and PTY management via `aymanbagabas/go-pty`
- True color (24-bit) and 256-color rendering with direct RGB color support
- When the terminal is focused, all keys go to the PTY except force keys: Ctrl+T, Alt+T, Ctrl+Q, Ctrl+P, Ctrl+K P, Ctrl+B
- Scrollback buffer with mouse wheel scrolling (3 lines), Shift+PgUp/PgDn (half page), and a draggable scrollbar
- Click the terminal content area to focus it; any keypress snaps back to the live view when scrolled up

## Bottom Panel

Toggle the bottom panel with **Ctrl+K B**. It contains three tabs:

- **Terminal** for the integrated terminal
- **Problems** listing all LSP diagnostics grouped by file; click to jump to location
- **References** showing results from Find All References; click to jump to location

## Configuration

Terminal settings in `settings.json`:

```json
{
  "terminal": {
    "shell": "/bin/zsh",
    "scrollback": 1000
  }
}
```

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `terminal.shell` | string | `""` | Shell command (empty uses system default) |
| `terminal.scrollback` | int | `1000` | Number of scrollback lines to retain |

Terminal ANSI colors are configurable via the `terminal` field in your theme file.
