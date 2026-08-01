---
title: Default Keybindings
description: Complete list of default keyboard shortcuts.
sidebar:
  order: 2
---

All keybindings can be customized in `~/.config/ttt/keybindings.json`. See [`config/keybindings.json`](https://github.com/eugenioenko/ttt/blob/main/config/keybindings.json) for a complete example.

You can open your keybindings file from the command palette (**Ctrl+P**) with **Preferences: Open Keyboard Shortcuts**, or press **Ctrl+K Y**.

## General

| Shortcut | Command | Description |
|----------|---------|-------------|
| Ctrl+Q | `editor.quit` | Quit the editor |
| Ctrl+P | `command.palette` | Open command palette |
| Ctrl+K P | `file.quickOpen` | Quick open file |
| Escape | *(built-in)* | Dismiss overlay / collapse cursors / focus the editor |
| F6 | `focus.nextGroup` | Focus next group (editor → panel → sidebar) |
| Shift+F6 | `focus.prevGroup` | Focus previous group |

## File

| Shortcut | Command | Description |
|----------|---------|-------------|
| Ctrl+N | `file.new` | New file |
| Ctrl+O | `workspace.openFolder` | Open folder |
| Ctrl+S | `file.save` | Save |
| Ctrl+K S | `file.saveAs` | Save as |

## Editor

| Shortcut | Command | Description |
|----------|---------|-------------|
| Ctrl+Z | `editor.undo` | Undo |
| Ctrl+Y | `editor.redo` | Redo |
| Ctrl+A | `editor.selectAll` | Select all |
| Ctrl+C | `editor.copy` | Copy |
| Ctrl+X | `editor.cut` | Cut |
| Ctrl+V | `editor.paste` | Paste |
| Ctrl+G | `editor.goToLine` | Go to line |
| Ctrl+/ | `editor.toggleComment` | Toggle line comment |
| Ctrl+Enter | `editor.insertLineBelow` | Insert line below |
| Alt+Up | `editor.moveLineUp` | Move line up |
| Alt+Down | `editor.moveLineDown` | Move line down |
| Ctrl+K K | `editor.deleteLine` | Delete line |
| Ctrl+K J | `editor.joinLines` | Join lines |
| Ctrl+K O | `editor.sortLinesAsc` | Sort lines ascending |
| Ctrl+K M | `editor.goToMatchingBracket` | Go to matching bracket |

## Word Navigation

| Shortcut | Command | Description |
|----------|---------|-------------|
| Ctrl+Left | `editor.moveWordLeft` | Move cursor one word left |
| Ctrl+Right | `editor.moveWordRight` | Move cursor one word right |
| Ctrl+Shift+Left | `editor.selectWordLeft` | Select one word left |
| Ctrl+Shift+Right | `editor.selectWordRight` | Select one word right |
| Alt+Backspace | `editor.deleteWordLeft` | Delete word left |
| Alt+Delete | `editor.deleteWordRight` | Delete word right |
| Ctrl+Delete | `editor.deleteWordRight` | Delete word right |

## Multi-Cursor

| Shortcut | Command | Description |
|----------|---------|-------------|
| Ctrl+D | `multicursor.selectNext` | Select next occurrence of current word or selection |
| Ctrl+K L | `multicursor.selectAll` | Select all occurrences at once |
| Ctrl+K U | `multicursor.undoCursor` | Undo last cursor addition |
| Alt+Click | *(mouse)* | Add cursor at click position |
| Escape | *(built-in)* | Collapse to single cursor (when multiple cursors exist) |

## Code Folding

| Shortcut | Command | Description |
|----------|---------|-------------|
| Ctrl+K [ | `fold.toggle` | Toggle fold at cursor |
| Ctrl+K 0 | `fold.collapseAll` | Collapse all folds |
| Ctrl+K 9 | `fold.expandAll` | Expand all folds |

## Search

| Shortcut | Command | Description |
|----------|---------|-------------|
| Ctrl+F | `search.find` | Find |
| Ctrl+R | `search.replace` | Find and replace |
| F3 | `search.findNext` | Find next |
| Shift+F3 | `search.findPrev` | Find previous |

In find/replace dialogs, use Alt+C to toggle case sensitivity and Alt+R (or Alt+X in replace) for regex.

## View

| Shortcut | Command | Description |
|----------|---------|-------------|
| Ctrl+B | `sidebar.toggle` | Toggle sidebar |
| Ctrl+K B | `panel.toggle` | Toggle bottom panel |
| Alt+Shift+M | `menubar.toggle` | Show/hide the menu bar |
| Ctrl+K E | `sidebar.explorer` | Show file explorer |
| Ctrl+K F | `sidebar.search` | Search across files |
| Ctrl+K R | `sidebar.searchReplace` | Search and replace in files |
| Ctrl+K C | `sidebar.changes` | Show changes |
| Ctrl+K Y | `view.keybindings` | Open keybindings |
| Ctrl+K , | `settings.openUI` | Open the settings editor |
| Ctrl+0 | `sidebar.focus` | Focus sidebar |
| Alt+Shift+Left | `sidebar.narrower` | Decrease sidebar width |
| Alt+Shift+Right | `sidebar.wider` | Increase sidebar width |
| Alt+Shift+Up | `panel.taller` | Increase panel height |
| Alt+Shift+Down | `panel.shorter` | Decrease panel height |

## Tabs

| Shortcut | Command | Description |
|----------|---------|-------------|
| Alt+. | `tab.next` | Next tab |
| Alt+, | `tab.prev` | Previous tab |
| Ctrl+W | `tab.close` | Close tab |

## LSP

| Shortcut | Command | Description |
|----------|---------|-------------|
| Ctrl+U | `editor.autocomplete` | Trigger autocomplete |
| Ctrl+K I | `editor.hover` | Hover info |
| F2 | `editor.rename` | Rename symbol |
| F12 | `editor.goToDefinition` | Go to definition |
| Shift+F12 | `editor.goToImplementation` | Go to implementation |
| Ctrl+L I | `editor.goToImplementation` | Go to implementation |
| Ctrl+L F | `editor.formatDocument` | Format document (LSP) |
| Ctrl+L E | `editor.formatExternal` | Format document (external formatter) |
| Ctrl+L S | `editor.formatSelection` | Format selection |
| Ctrl+L O | `editor.organizeImports` | Organize imports |
| Ctrl+L X | `editor.fixAll` | Fix all |
| Ctrl+L R | `editor.findReferences` | Find references |
| Ctrl+L T | `editor.goToTypeDefinition` | Go to type definition |

## Terminal

| Shortcut | Command | Description |
|----------|---------|-------------|
| Ctrl+T | `terminal.toggle` | Toggle terminal (half screen) |
| Alt+T | `terminal.fullscreen` | Toggle terminal fullscreen |
| Ctrl+K T | `terminal.new` | New terminal tab |

## Menu Bar

| Shortcut | Command | Description |
|----------|---------|-------------|
| F10 / Alt+F | `menu.file` | File menu |
| Alt+E | `menu.edit` | Edit menu |
| Alt+S | `menu.selection` | Selection menu |
| Alt+V | `menu.view` | View menu |
| Alt+O | `menu.options` | Options menu |
| Alt+H | `menu.help` | Help menu |
| Alt+Shift+M | `menubar.toggle` | Show/hide the menu bar |

The menu shortcuts above keep working while the menu bar is hidden — the dropdown opens on its own from the top edge.
