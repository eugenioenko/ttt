---
title: Integrated Terminal
description: Built-in terminal emulator in TTT.
---

TTT includes a built-in terminal emulator. Press **Ctrl+T** to toggle the terminal panel.

## Usage

- **Ctrl+T** to toggle the terminal (half screen)
- **Alt+T** to toggle the terminal fullscreen
- **Ctrl+K T** to spawn a new terminal tab
- **Ctrl+V** or **Ctrl+Shift+V** to paste from the clipboard
- Multiple terminal tabs with a vertical inner tab bar on the left edge
- Close all terminals from the panel actions menu

## Editor Tabs

Choose **Terminal: New Terminal in Editor** from the command palette, or **New Terminal in Editor** from the View menu, to open a terminal alongside your file tabs. Closing its editor tab stops the program; when the program exits, its tab closes too.

While the terminal tab is focused, tab-switching shortcuts go to the program. Use the mouse, command palette, or quick open to switch tabs. **Ctrl+T** still toggles the terminal panel. **Terminal: Close All** closes panel terminals only, and editor terminal tabs are not restored after restarting TTT.

Plugins can open a program in an editor tab with `term.spawn{ shell = "program", location = "editor" }`. See the [terminal plugin API](./plugin-authoring/#tttterminal-module).

## Features

- Full VT escape sequence support via `eugenioenko/xterm-go` and PTY management via `aymanbagabas/go-pty`
- True color (24-bit) and 256-color rendering with direct RGB color support
- Reserved TTT shortcuts and configured chords take priority while the terminal is focused; other keys go through the terminal's local clipboard handling and then to the PTY
- Scrollback buffer with mouse wheel scrolling (3 lines), Shift+PgUp/PgDn (half page), and a draggable scrollbar
- Click the terminal content area to focus it; any keypress snaps back to the live view when scrolled up

With [`keyboard.modifierKey`](/reference/settings/#keyboard) set to `"control+super"` or `"super"`, Super+V also pastes and Super+C copies selected terminal text. Super is Command (⌘) on macOS. Super+C without a selection does nothing; Ctrl+C still copies selected text or sends an interrupt when nothing is selected. Super+V pastes even when `terminal.ctrlVPastes` is `false`, so Ctrl+V can remain available to terminal programs.

Ctrl+click opens URLs and file links in terminal output. When Super shortcuts are enabled, Super+click also opens links, shows the link cursor and underline, and permits local text selection when the terminal program captures the mouse. The outer terminal must report the modifier. These settings apply live to both panel terminals and editor terminal tabs; other shell key and mouse encodings keep their original modifiers.

## Shortcut forwarding

**Settings → General → Keyboard** contains both **Forward shortcuts to terminal** and the editable **Send next key to terminal** shortcut, beside **Shortcut modifier**. The shortcut is stored in `keybindings.json` as `terminal.sendNextKey`, with **Ctrl+K** as its default. It follows the selected Ctrl/Mod/Super preference.

With the terminal focused, press **Ctrl+K**, then:

- A plain or Shift-only key completes a TTT chord: **Ctrl+K P** opens Quick Open, **Ctrl+K T** creates a terminal, and **Ctrl+K Shift+P** also opens Quick Open.
- A Ctrl/Alt/Super combination is sent directly to the terminal using its normal key encoding: **Ctrl+K Ctrl+P** sends Ctrl+P for shell history, **Ctrl+K Ctrl+B** sends the tmux prefix, and **Ctrl+K Ctrl+K** sends Ctrl+K for shell kill-line. This bypasses reserved TTT shortcuts and terminal-local copy/paste handling once.

Clicking, changing focus or tabs, opening a dialog, or closing the terminal cancels forwarding. The command does nothing outside a focused terminal. Running **Terminal: Send Next Key** from the command palette forwards the next key without starting a chord.

Custom bindings with a modified second key after the forwarding prefix are overridden only while the terminal is focused. Rebind the forwarding command or use another prefix for those chords. Editor chord matching is unchanged.

`terminal.forwardShortcuts` defaults to `false`, giving reserved TTT shortcuts and chords priority. Enable it to pass those shortcuts directly to the terminal, including the first Ctrl+K. Terminal-local clipboard and scrolling behavior remains unchanged. Use the mouse menus to access TTT commands while direct forwarding is enabled. Changes apply immediately to panel and editor terminals.

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
    "padding_x": 1,
    "shell": "/bin/zsh",
    "scrollback": 1000,
    "ctrlVPastes": true,
    "forwardShortcuts": false,
    "selectionStyle": "background",
    "shellIntegration": true
  }
}
```

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `terminal.shell` | string | `""` | Shell command (empty uses system default) |
| `terminal.padding_x` | int | `1` | Blank columns on each side of the terminal; `0` removes both margins |
| `terminal.ctrlVPastes` | bool | `true` | Paste with Ctrl+V; set to `false` to pass `^V` through to the terminal program |
| `terminal.forwardShortcuts` | bool | `false` | Forward reserved TTT shortcuts and chord prefixes to the terminal. Available under General → Keyboard; applies immediately |
| `terminal.scrollback` | int | `1000` | Number of scrollback lines to retain |
| `terminal.selectionStyle` | string | `"background"` | Selection highlight style: `"background"` or `"invert"` |
| `terminal.shellIntegration` | bool | `true` | Start bash with ttt's prompt marks so resizing redraws the prompt cleanly |

Set `terminal.ctrlVPastes` to `false` for shell quoted insert or Vim visual-block mode. Ctrl+Shift+V still pastes when the outer terminal reports it distinctly. Some terminals handle Ctrl+Shift+V themselves and send a bracketed paste, which also works. Terminals that encode both shortcuts as `^V` cannot distinguish them. The setting applies immediately to open terminal tabs and is available under **Settings → Terminal → Ctrl+V pastes**.

Padding applies immediately to existing and new terminal tabs and is available under **Settings → Terminal → Horizontal padding**. The settings UI rejects negative values; negative values loaded from `settings.json` are treated as `0`. In narrow panels, padding is reduced equally on both sides to leave room for terminal content. Right padding separates terminal text from the scrollbar. The scrollbar takes a column at the panel edge only while visible. Full-screen programs using the alternate screen, such as btop, use that column for terminal text instead. Padding does not add vertical spacing.

### Shell integration

Resizing the terminal makes the shell redraw its prompt, and ttt clears the old one first so no copy is left behind. It finds the prompt through OSC 133 marks. fish sends them itself. For bash, ttt starts the shell with `bash --rcfile`, using a script that loads your `~/.bashrc` as usual and then adds the marks to `PS1` and `PS0`. Your prompt, aliases, and `PROMPT_COMMAND` keep working, including prompts that `PROMPT_COMMAND` rebuilds.

The marks need bash 4.4 or newer. ttt skips the integration when `terminal.args` is set, so a custom launch such as `["-l"]` runs unchanged. Set `terminal.shellIntegration` to `false` (**Settings → Terminal → Shell integration**) to start bash plainly; it applies to new terminal tabs. Other shells start unchanged.

Terminal ANSI colors are configurable via the `terminal` field in your theme file.
