---
title: Settings
description: Complete settings reference for TTT.
sidebar:
  order: 1
---

Settings are stored in `~/.config/ttt/settings.json`. A complete example is available at [`config/settings.json`](https://github.com/eugenioenko/ttt/blob/main/config/settings.json) in the repository.

## Editing settings

There are two ways to change settings:

- **Settings editor** — **View → Settings**, **Ctrl+K ,**, or **Settings: Open Editor Settings** from the command palette (**Ctrl+P**). Opens a form in an editor tab, grouped into **Editor**, **Appearance**, **Completion** and **Advanced** (Git, explorer, terminal, search and plugin options live under Advanced). Edits are held until you press **Apply** (also available as **Settings: Apply Changes**), which writes `settings.json` and live-applies everything that does not require a restart. **Cancel** (also **Settings: Discard Changes**) closes the tab and drops them. Rows marked *(restart)* only take effect on next launch.
- **Raw JSON** — **Settings: Open settings.json** opens the file itself. Needed for the `lsp` settings and `formatters`, neither of which is exposed in the form.

Closing the settings tab with unapplied edits discards them.

Both write the same file, so you can move between them freely.

## Top-level

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `version` | int | `1` | Settings file format version |
| `theme` | string | `""` | Theme name (e.g. `"default-dark"`) |
| `debugMode` | bool | `false` | Enable debug logging |

## Editor

All editor settings are nested under the `editor` key.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `editor.tabSize` | int | `4` | Number of spaces per indentation level |
| `editor.insertSpaces` | bool | `true` | Use spaces instead of tabs for indentation |
| `editor.wordWrap` | bool | `false` | Wrap long lines at the editor width |
| `editor.diffMode` | string | `"split"` | Default diff layout: `"split"` or `"unified"` |
| `editor.diffContext` | string | `"changes"` | Default diff context: `"changes"` or `"full"` |
| `editor.diffWordWrap` | bool | `false` | Wrap long lines in diff views |
| `editor.diffHighContrast` | bool | `false` | Strengthen changed-line visibility with semantic red/green foregrounds |
| `editor.autoIndent` | bool | `true` | Inherit the previous line's indent on Enter, plus one level after `{ ( [ :` (turn off for `noautoindent` behavior) |
| `editor.autoDedent` | bool | `true` | Dedent one level when typing a closing `} ) ]` on a blank line |
| `editor.lineNumbers` | bool | `true` | Show line numbers in the gutter |
| `editor.cursorStyle` | string | `""` | Cursor style: `"block"`, `"underline"`, or `"bar"` |
| `editor.formatOnSave` | bool | `false` | Auto-format the document via LSP on save |
| `editor.insertFinalNewline` | bool | `true` | Ensure files end with a newline on load and save |
| `editor.trimTrailingWhitespace` | bool | `false` | Remove trailing whitespace from lines on save |
| `editor.focusOnOpen` | bool | `false` | Focus the editor when opening a file |
| `editor.syntaxHighlight` | bool | `true` | Enable syntax highlighting |
| `editor.gitGutter` | bool | `true` | Show git change indicators in the gutter |
| `editor.menuBar` | bool | `true` | Show the menu bar row at the top of the window |
| `editor.gutterStyle` | string | `"compact"` | Gutter layout: `"minimal"`, `"compact"`, or `"extended"` |
| `editor.borderStyle` | string | `"default"` | Border style preset: `"default"`, `"rounded"`, `"sharp"`, `"double"`, `"bold"`, `"ascii"`, `"none"`. Use `"default"` or `"theme"` to defer to the active theme. |
| `editor.bracketPairColorization` | bool | `false` | Colorize matching bracket pairs by nesting depth |

## Explorer

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `explorer.showHidden` | bool | `true` | Show hidden files (dot-prefixed) in the file explorer |
| `explorer.showGitIgnored` | bool | `true` | Show gitignored files in the file explorer |

## Sidebar

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `sidebar.panelOrder` | string[] | built-in order | Preferred sidebar panel-header order. Dragging a header or using **Move Panel Left/Right** updates it automatically. Unknown plugin panel IDs are retained until that plugin loads. |

## Git

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `git.fileView` | string | `"list"` | Show working-tree and expanded commit files as a compact `"tree"` or full-path `"list"` |

## Terminal

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `terminal.shell` | string | `""` | Shell command (empty uses system default) |
| `terminal.scrollback` | int | `1000` | Number of scrollback lines to retain |

## LSP

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `lsp.enabled` | bool | `true` | Enable LSP support |
| `lsp.hover` | bool | `true` | Show hover information from the language server |
| `lsp.hoverDelay` | int | `500` | Milliseconds to wait before showing hover information |
| `lsp.saveOnRename` | bool | `false` | Auto-save files affected by a rename operation |
| `lsp.codeActionsOnSave` | string[] | `[]` | Code actions to run before save (e.g. `"source.organizeImports"`) |
| `lsp.notifyAvailability` | bool | `true` | Show a notification when a language server binary is not installed |
| `lsp.servers` | object | `{}` | Map of server key to `{ "command": [...], "languages": {...} }`. Configured automatically by LSP plugins (e.g. `lsp-go`, `lsp-typescript`). The optional `languages` field maps file extensions to language IDs for servers handling multiple file types. |

## Search

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `search.debounce` | int | `350` | Milliseconds to debounce global search input |

## Plugins

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `plugins.enabled` | bool | `true` | Enable the plugin system. When `false`, no plugins are loaded and the Plugins sidebar tab is hidden. |

## Markdown

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `markdown.wrapWidth` | int | `80` | Column width at which prose wraps in rendered markdown (hover popups and plugin markdown widgets) |

## Autocomplete

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `autocomplete.enabled` | bool | `true` | Enable LSP-powered autocompletion |
| `autocomplete.autoSuggest` | bool | `true` | Show completions automatically as you type |
| `autocomplete.debounce` | int | `150` | Milliseconds to wait after typing before requesting completions |
| `autocomplete.signatureHelp` | bool | `true` | Show function signature help on `(` and `,` |

## Formatters

External code formatters configured per file extension. Each formatter receives the buffer content via stdin and must write the formatted output to stdout. Use `{file}` as a placeholder for the file path (needed by formatters like prettier for filetype detection).

When `editor.formatOnSave` is `true`, external formatters take priority over LSP formatting. If no external formatter is configured for the file type, LSP formatting is used as a fallback.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `formatters.<ext>` | string | — | Formatter command for files with this extension. `<ext>` is the extension without the dot (e.g. `go`, `js`, `py`). |

**Example:**

```json
{
  "formatters": {
    "go": "gofmt",
    "lua": "stylua -",
    "js": "prettier --stdin-filepath {file}",
    "py": "black -"
  }
}
```

**Keybinding:** `Ctrl+L E` runs the external formatter. `Ctrl+L F` runs the LSP formatter.

## Full Example

```json
{
  "version": 1,
  "theme": "default-dark",
  "debugMode": false,
  "editor": {
    "tabSize": 4,
    "insertSpaces": true,
    "wordWrap": false,
    "diffMode": "split",
    "diffContext": "changes",
    "diffWordWrap": false,
    "autoIndent": true,
    "autoDedent": true,
    "lineNumbers": true,
    "cursorStyle": "",
    "formatOnSave": false,
    "insertFinalNewline": true,
    "trimTrailingWhitespace": false,
    "focusOnOpen": false,
    "syntaxHighlight": true,
    "gitGutter": true,
    "menuBar": true,
    "gutterStyle": "compact",
    "borderStyle": "default",
    "bracketPairColorization": false
  },
  "search": {
    "debounce": 350
  },
  "explorer": {
    "showHidden": true,
    "showGitIgnored": true
  },
  "sidebar": {
    "panelOrder": ["explorer", "search", "changes", "outline"]
  },
  "git": {
    "fileView": "list"
  },
  "terminal": {
    "shell": "/bin/zsh",
    "scrollback": 1000
  },
  "lsp": {
    "enabled": true,
    "hover": true,
    "hoverDelay": 500,
    "saveOnRename": false,
    "notifyAvailability": true,
    "codeActionsOnSave": [
      "source.organizeImports",
      "source.fixAll"
    ]
  },
  "autocomplete": {
    "enabled": true,
    "autoSuggest": true,
    "debounce": 150,
    "signatureHelp": true
  },
  "plugins": {
    "enabled": true
  },
  "formatters": {
    "go": "gofmt",
    "lua": "stylua -",
    "js": "prettier --stdin-filepath {file}",
    "py": "black -"
  }
}
```
