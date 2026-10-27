---
title: Settings
description: Complete settings reference for TTT.
sidebar:
  order: 1
---

Settings are stored in `~/.config/ttt/settings.json`. A complete example is available at [`config/settings.json`](https://github.com/eugenioenko/ttt/blob/main/config/settings.json) in the repository.

## Editing settings

There are two ways to change settings:

- **Settings editor** — **View → Settings**, **Ctrl+K ,**, or **Settings: Open Editor Settings** from the command palette (**Ctrl+P**). Opens a form in an editor tab, grouped into tabs by area, with headed sections inside each: **General** (plugins, debugging), **Editor** (indentation, display, on save, completion), **Appearance** (theme, window, code, markdown), **Sidebar** (explorer files and look, source control, search), **Terminal** and **Diff**. It opens on General. Edits are held until you press **Apply** (also available as **Settings: Apply Changes**), which writes `settings.json` and live-applies everything that does not require a restart. **Cancel** (also **Settings: Discard Changes**) closes the tab and drops them. Rows marked *(restart)* only take effect on next launch.
- **Raw JSON** — **Settings: Open settings.json** opens the file itself. Needed for the `lsp` settings and `formatters`, neither of which is exposed in the form.

Closing the settings tab with unapplied edits discards them.

Both write the same file, so you can move between them freely.

## Top-level

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `version` | int | `1` | Settings file format version |
| `theme` | string | `""` | Theme name (e.g. `"default-dark"`) |
| `debugMode` | bool | `false` | Enable debug logging |

## Appearance

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `appearance.icons` | string | `"none"` | Icons in the explorer, Changes panel, commit history and other UI glyphs such as chevrons and tab markers: `"nerd-font"` or `"none"`. The glyphs need a [Nerd Font](https://www.nerdfonts.com) in your terminal and show as boxes without one, so this defaults off; turn it on here, in Settings > Sidebar, or via **Options > Font Icons**. Colors come from the theme's `fileIcons` section |
| `appearance.overrides` | object | `{}` | Replace any UI glyph, keyed by icon name (see [Icon overrides](#icon-overrides)). An override wins over both the plain and Nerd Font glyph. |
| `appearance.scrollbarGlyphs` | string | `"blocks"` | Glyphs that draw git change marks on the editor scrollbar: `"blocks"` uses the half and eighth blocks every font has, which place a mark exactly near the top or bottom of a cell and stretch one in the middle to the nearer edge; `"legacy"` adds the Symbols for Legacy Computing thin bars, which place every mark to the exact eighth of a cell. Kitty, Ghostty, WezTerm and foot draw those themselves; elsewhere the font needs them or the marks show as boxes. |

## Icon overrides

`appearance.overrides` replaces any UI glyph by name. Resolution order: your override, then the Nerd Font glyph when `appearance.icons` is `"nerd-font"`, then the plain glyph.

```json
"appearance": {
  "icons": "nerd-font",
  "overrides": {
    "chevron-right": "▸",
    "chevron-down": "▾",
    "close": "✕"
  }
}
```

Every icon must be exactly one character that is one column wide, because trees, tabs and gutters lay out by fixed cell positions. An override that isn't falls back to the default, with a warning in the Output panel. Overrides apply live when settings change.

The old `appearance.chevrons.collapsed` and `appearance.chevrons.expanded` settings are migrated to `chevron-right` and `chevron-down` on load.

Plain defaults avoid East Asian *ambiguous*-width characters, which draw two columns wide in CJK-locale terminals or with `RUNEWIDTH_EASTASIAN` set. The exceptions are `dirty`, `lock`, `commit` and the symbol kinds, which have no narrow glyph that reads as well; override them if they misalign in your terminal.

| Name | Plain | Nerd Font | Used for |
|---|---|---|---|
| `chevron-right` | `▸` | `\ueab6` | Collapsed folder or group in trees, collapsed fold in the editor gutter |
| `chevron-down` | `▾` | `\ueab4` | Expanded folder or group, expanded fold while hovering the gutter |
| `chevron-up` | `▴` | `\ueab7` | Open select dropdown |
| `submenu` | `›` | `\ueab6` | Context menu item with a submenu |
| `arrow-left` | `◂` | `\ueb6f` | Tab strip overflow, left |
| `arrow-right` | `▸` | `\ueb70` | Tab strip overflow, right |
| `close` | `✕` | `✕` | Tab close button |
| `dirty` | `●` | `●` | Unsaved changes marker in tabs |
| `pinned` | `♦` | `\ueba0` | Pinned tab |
| `lock` | `⚿` | `\uea75` | Read-only tab |
| `more` | `⋮` | `⋮` | More actions menu buttons |
| `add` | `+` | `+` | Stage and new-item actions |
| `remove` | `−` | `−` | Unstage actions |
| `discard` | `✕` | `✕` | Discard changes action |
| `replace` | `⟳` | `\ueb3d` | Replace button in the replace bar |
| `branch` | `⎇` | `\uf418` | Branch in the Changes panel |
| `commit` | `●` | `\uf417` | Commit in commit history |
| `function` | `ƒ` | `\uea8c` | Symbol and completion kinds |
| `class` | `◆` | `\ueb5b` |  |
| `interface` | `◇` | `\ueb61` |  |
| `module` | `▤` | `\uea8b` |  |
| `field` | `▪` | `\ueb5f` |  |
| `constant` | `●` | `\ueb5d` |  |
| `variable` | `●` | `\uea88` |  |
| `string` | `§` | `\ueb8d` |  |
| `keyword` | `■` | `\ueb62` |  |
| `snippet` | `■` | `\ueb66` |  |
| `symbol` | `•` | `\ueb63` |  |

## Editor

All editor settings are nested under the `editor` key.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `editor.tabSize` | int | `4` | Number of spaces per indentation level |
| `editor.insertSpaces` | bool | `true` | Use spaces instead of tabs for indentation |
| `editor.wordWrap` | bool | `false` | Soft-wrap long lines; the global Options toggle changes this default |
| `editor.wordWrapColumn` | int | `0` | Maximum soft-wrap width in terminal display columns; `0` uses the available text width. Positive values cap it after gutters, scrollbars, and wrap margins. Ignored when wrapping is off; negative values normalize to `0` |
| `editor.languageOverrides` | object | `{}` | Per-language `wordWrap` and `wordWrapColumn` overrides keyed by language ID, such as `typescript`, `markdown`, or `go`. Each omitted field inherits its global value; explicit `false` and `0` override it. Does not require LSP |
| `editor.largeFiles.wordWrapMaxMB` | int | `32` | Files larger than this many megabytes open without word wrap, even when `editor.wordWrap` is on. `0` removes the limit |
| `editor.largeFiles.autoReloadMaxMB` | int | `64` | Files larger than this many megabytes show a Reload action instead of automatically reloading after disk changes. `0` removes the limit. One megabyte is 1,048,576 bytes; negative values also remove the limit |
| `editor.diffMode` | string | `"split"` | Default diff layout: `"split"` or `"unified"` |
| `editor.diffContext` | string | `"changes"` | Default diff context: `"changes"` or `"full"` |
| `editor.diffWordWrap` | bool | `false` | Wrap long lines in diff views |
| `editor.diffHighContrast` | bool | `false` | Strengthen changed-line visibility with semantic red/green foregrounds |
| `editor.diffEmphasizeCollapsedRows` | bool | `false` | Emphasize collapsed or omitted-line rows in diff views |
| `editor.diffSigns` | bool | `false` | Draw `+`/`−` in the diff gutter. Off by default: the gutter highlight already marks changed lines. Gap rows show their expand chevron regardless of this setting. The `minimal` gutter style has no room for signs or the chevron |
| `editor.diffSignsColor` | bool | `true` | Color diff gutter signs with the added/deleted colors; when off they use the line number color |
| `editor.diffGutterHighlight` | bool | `true` | Tint the gutter background of added and deleted rows; when off the gutter stays plain and only the text is tinted |
| `editor.diffColorGutterNumbers` | bool | `false` | Color the line numbers of added and deleted rows with the added/deleted gutter colors; when off the numbers use the normal line number color |
| `editor.diffScrollbarGitMarks` | bool | `false` | Mark added and deleted rows on the diff view scrollbar |
| `editor.autoIndent` | bool | `true` | Inherit the previous line's indent on Enter, plus one level after `{ ( [ :` (turn off for `noautoindent` behavior) |
| `editor.autoDedent` | bool | `true` | Dedent one level when typing a closing `} ) ]` on a blank line |
| `editor.lineNumbers` | bool | `true` | Show line numbers in the gutter |
| `editor.cursorStyle` | string | `""` | Cursor style: `"block"`, `"underline"`, or `"bar"` |
| `editor.formatOnSave` | bool | `false` | Auto-format the document via LSP on save |
| `editor.saveAllOnQuit` | bool | `false` | Add **Save All & Quit** to the **Unsaved changes** dialog. Saves modified named buffers first, then prompts **Save As** for each modified unnamed buffer. Cancelling or failing a save stops quitting; files already saved remain saved. Also in **Settings > Editor > On quit**. |
| `editor.saveAllOnQuitExceptUnnamed` | bool | `false` | Hide **Save All & Quit** whenever an unnamed buffer is open, even if it is unmodified. Only applies when `editor.saveAllOnQuit` is enabled. |
| `editor.insertFinalNewline` | bool | `true` | Ensure files end with a newline on load and save |
| `editor.trimTrailingWhitespace` | bool | `false` | Remove trailing whitespace from lines on save |
| `editor.focusOnOpen` | bool | `false` | Focus the editor when opening a file |
| `editor.syntaxHighlight` | bool | `true` | Enable syntax highlighting |
| `editor.gitGutter` | bool | `true` | Show git change indicators in the gutter, compared against the staged version of the file (the git index) |
| `editor.scrollbarGitMarks` | bool | `true` | Mark git changes on the editor's vertical scrollbar. Needs `editor.gitGutter` |
| `editor.menuBar` | bool | `true` | Show the menu bar row at the top of the window |
| `editor.gutterStyle` | string | `"compact"` | Gutter layout: `"minimal"`, `"compact"`, or `"extended"` |
| `editor.borderStyle` | string | `"default"` | Border style preset: `"default"`, `"rounded"`, `"sharp"`, `"double"`, `"bold"`, `"ascii"`, `"none"`. Use `"default"` or `"theme"` to defer to the active theme. |
| `editor.selectionStyle` | string | `"background"` | Selection highlight style: `"background"` (use theme's selection background) or `"invert"` (invert foreground and background colors) |
| `editor.bracketPairColorization` | bool | `false` | Colorize matching bracket pairs by nesting depth |
| `editor.grammars` | array | `[]` | External TextMate grammars (see [Grammars](#grammars)) |

### Language-specific wrapping

Each wrap field resolves independently: language override → global setting → built-in default. The defaults are `wordWrap: false` and `wordWrapColumn: 0`.

```json
{
  "editor": {
    "wordWrap": true,
    "wordWrapColumn": 0,
    "languageOverrides": {
      "typescript": { "wordWrapColumn": 80 },
      "markdown": { "wordWrapColumn": 65 }
    }
  }
}
```

With a global column of `80`, a Markdown override of `0` instead wraps at the pane width. Explicit `wordWrap: false` disables wrapping for that language. The global Word Wrap toggle leaves language overrides in place. Effective settings update on tab switches, language detection changes, and settings reload, without requiring LSP. Large-file safeguards still apply; diff wrapping and rendered `markdown.wrapWidth` remain separate. See [Soft Wrapping](/guides/editor/#soft-wrapping) for details.

## Explorer

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `explorer.showHidden` | bool | `true` | Show hidden files (dot-prefixed) in the file explorer |
| `explorer.showGitIgnored` | bool | `true` | Show gitignored files in the file explorer |
| `explorer.gitStatusColors` | bool | `true` | Color files and folders in the file explorer by their git status (modified, new, deleted, conflicted). Colors come from the theme's `warning`, `success`, `danger`, and `conflict` colors |
| `explorer.dimStagedGitColors` | bool | `false` | Render staged changes in a dimmed version of their status color, telling them apart from pending ones. Off by default: the Changes panel already separates staged from unstaged, and the extra shades make the explorer busier. Needs `explorer.gitStatusColors` |
| `explorer.autoReveal` | bool | `false` | Expand the file explorer to the active file and select it whenever the active file changes. **Explorer: Reveal Active File** does the same on demand |

## Quick Open

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `quickOpen.showHidden` | bool | `false` | Show dot-prefixed files and directories in Go to File |
| `quickOpen.showGitIgnored` | bool | `false` | Show Git-ignored files in Go to File |

Set these independently in **Settings > Sidebar > Quick Open**. Hidden files still respect Git ignore rules unless `showGitIgnored` is enabled; showing Git-ignored files does not include hidden paths unless `showHidden` is enabled. `.git` files and directories always stay excluded. These options do not affect Explorer or workspace text search.

## Sidebar

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `sidebar.panelOrder` | string[] | built-in order | Initial sidebar panel-header order. Unknown plugin panel IDs are retained until that plugin loads. |
| `sidebar.width` | int | `30` | Initial sidebar width in columns. |
| `sidebar.commitHistoryHeight` | int | proportional | Initial height of the commit history in the Changes panel, in rows. |

These are starting values. Once you drag a header, resize the sidebar, the commit history, or the panel, the new layout is saved to `state.json` next to `settings.json` and takes precedence. Delete `state.json` to return to these values.

## Panel

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `panel.position` | string | `"bottom"` | Where the panel (terminal, diagnostics, output) is initially docked: `"bottom"` or `"right"`. **Dock Panel Right/Bottom** saves the choice to `state.json`, which takes precedence. |

## Welcome

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `welcome.showOnHome` | bool | `false` | Start on the welcome page instead of opening `$HOME` when ttt runs there with no arguments, as desktop launchers do. `ttt --welcome` does the same from anywhere. Also in **Settings > General**. |
| `welcome.favorites` | string[] | `[]` | Folders listed on the welcome page, opened with one click. `~` is expanded; missing folders are skipped. **Add to Favorites** / **Remove from Favorites** in an Explorer root's right-click menu (or **Welcome: Add/Remove Folder to/from Favorites** in the palette) update it automatically. |
| `welcome.recentFolders` | bool | `true` | Save opened folders in `state.json` and show them on the welcome page. Turning this off immediately removes the saved history. Also in **Settings > General**. |

Below the favorites, the welcome page lists the last 10 folders opened under **Recent**, most recent first. ttt keeps the history in `state.json`, so a `settings.json` in a dotfiles repo is not rewritten every time you open a folder. Favorites and folders that no longer exist are left out.

## Git

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `git.fileView` | string | `"list"` | Show working-tree and expanded commit files as a compact `"tree"` or full-path `"list"` |

## Terminal

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `terminal.shell` | string | `""` | Shell command (empty uses system default) |
| `terminal.ctrlVPastes` | bool | `true` | Paste with Ctrl+V. Set to `false` to send `^V` to the terminal program. Ctrl+Shift+V always pastes when reported distinctly by the outer terminal |
| `terminal.scrollback` | int | `1000` | Number of scrollback lines to retain |
| `terminal.selectionStyle` | string | `"background"` | Selection highlight style: `"background"` (use theme's selection background) or `"invert"` (invert foreground and background colors) |

## LSP

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `lsp.enabled` | bool | `true` | Enable LSP support |
| `lsp.hover` | bool | `true` | Show hover information from the language server when the mouse rests over code. `editor.hover` (Ctrl+K I) works regardless |
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
| `plugins.selectionStyle` | string | `"invert"` | Selection highlight style inside plugin panels, tabs and dialogs: `"invert"` (invert foreground and background colors) or `"background"` (use theme's selection background). Inverted selections stay visible on a plugin's highlighted rows. |

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

## Grammars

ttt embeds TextMate grammars for over 120 languages. `editor.grammars` adds more, or replaces a built-in one, from TextMate grammar files in JSON format (`.tmLanguage.json`). Most VS Code language extensions ship one.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | yes | Grammar file, relative to the config directory (`~/.config/ttt/`) unless absolute. |
| `language` | string | no | Language name shown in the status bar. Defaults to the grammar's `name`. |
| `fileTypes` | string[] | no | File extensions or exact file names, such as `zig` or `Justfile`. Defaults to the grammar's `fileTypes`. |
| `plugin` | string | no | Set by ttt on entries a plugin installed. Uninstalling that plugin removes the entry and its file. |

```json
{
  "editor": {
    "grammars": [
      { "path": "grammars/zig.tmLanguage.json", "fileTypes": ["zig", "zon"] }
    ]
  }
}
```

Entries earlier in the list take precedence over later ones, and all of them take precedence over the embedded grammars, so an entry whose file types match a built-in language replaces it. Grammars can include each other across sources: an embedded Markdown grammar highlights a fenced block with an external grammar, and an external grammar can include an embedded one. A grammar file that is missing or invalid is skipped and reported in the Output panel.

Plugins can contribute grammars too. Installing one copies its grammar files into `~/.config/ttt/grammars/` and adds entries here (see [Grammar plugins](/guides/plugin-authoring/#grammar-plugins)).

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
  "appearance": {
    "icons": "none",
    "scrollbarGlyphs": "blocks"
  },
  "editor": {
    "tabSize": 4,
    "insertSpaces": true,
    "wordWrap": false,
    "wordWrapColumn": 0,
    "diffMode": "split",
    "diffContext": "changes",
    "diffWordWrap": false,
    "diffHighContrast": false,
    "diffEmphasizeCollapsedRows": false,
    "diffSigns": false,
    "diffSignsColor": true,
    "diffGutterHighlight": true,
    "diffColorGutterNumbers": false,
    "diffScrollbarGitMarks": false,
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
    "selectionStyle": "background",
    "bracketPairColorization": false
  },
  "quickOpen": {
    "showHidden": false,
    "showGitIgnored": false
  },
  "search": {
    "debounce": 350
  },
  "explorer": {
    "showHidden": true,
    "showGitIgnored": true,
    "gitStatusColors": true,
    "dimStagedGitColors": false,
    "autoReveal": false
  },
  "sidebar": {
    "panelOrder": ["explorer", "search", "changes", "outline"]
  },
  "git": {
    "fileView": "list"
  },
  "terminal": {
    "shell": "/bin/zsh",
    "scrollback": 1000,
    "selectionStyle": "background"
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
    "enabled": true,
    "selectionStyle": "invert"
  },
  "formatters": {
    "go": "gofmt",
    "lua": "stylua -",
    "js": "prettier --stdin-filepath {file}",
    "py": "black -"
  }
}
```
