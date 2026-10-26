---
title: Editor Basics
description: Core editing features in TTT.
---

## Syntax Highlighting

TTT highlights code with [textmate-go](https://github.com/eugenioenko/textmate-go), a pure Go port of VS Code's TextMate engine built for ttt. It embeds TextMate grammars for 124 languages, detected from the file name, and colors constructs that span lines (block comments, template literals, docstrings, tags with attributes on several lines) correctly.

## Soft Wrapping

Enable `editor.wordWrap` to display long lines on multiple rows without inserting newlines or marking the file modified. `editor.wordWrapColumn` defaults to `0`, which uses the pane's available text width after gutters, scrollbars, and existing margins. A positive value caps that width: a 65-column limit uses 65 columns in a wide pane and 50 in a pane with only 50 text columns. Columns measure terminal display width, including wide characters and tabs.

Use language IDs to set different limits for code and prose:

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

Each field resolves independently: language override, then global setting, then built-in default (`false` for wrapping and `0` for the column). An omitted field inherits; explicit `wordWrapColumn: 0` removes a global column cap, and `wordWrap: false` disables wrapping for that language. With effective wrapping off, the column has no effect. Unknown or unmatched file languages inherit the global settings. Detection uses the editor's grammars and does not require an installed or enabled language server, or syntax coloring.

The Options **Word Wrap** toggle changes the global default; explicit language overrides retain precedence. Switching tabs, changing the detected file language, or running **Reload Settings** applies the effective settings. Language overrides apply to all files of that language; filename and glob rules are not supported.

The `editor.largeFiles.wordWrapMaxMB` safeguard still applies, including when a language enables wrapping. Diff views retain their separate wrap policy. `markdown.wrapWidth` controls rendered Markdown, such as hover popups and plugin widgets; raw Markdown editing uses these editor settings.

## Bracket Matching

Matching brackets are highlighted automatically when the cursor is on a bracket character. Press **Ctrl+K M** to jump to the matching bracket.

Bracket pair colorization is also available, rendering each nesting level in a distinct color. It is off by default and can be enabled with the `editor.bracketPairColorization` setting.

## Auto Indent

New lines inherit the indentation of the previous line and gain an extra level after an open `{`, `(`, `[`, or trailing `:`. This is on by default; toggle it from the Options menu (**Auto Indent**) or with the `editor.autoIndent` setting. Turn it off for plain `noautoindent` behavior, where Enter opens a column-1 line.

Typing a closing `}`, `)`, or `]` on an otherwise-blank line dedents it one level so it lines up with its opening line. This dedent is on by default; toggle it from the Options menu (**Auto Dedent**) or with the `editor.autoDedent` setting.

## Find and Replace

- **Ctrl+F** opens the find bar with match navigation
- **Ctrl+R** opens find and replace with replace-one and replace-all
- **F3 / Shift+F3** to jump between matches

## Go to Line

Press **Ctrl+G** to open the Go to Line dialog.

## Selection and Clipboard

- **Ctrl+A** to select all
- **Ctrl+C / Ctrl+X / Ctrl+V** for copy, cut, paste with system clipboard support

## Undo and Redo

- **Ctrl+Z** to undo, **Ctrl+Y** to redo
- Uses a command-pattern undo stack for reliable history tracking

## External File Changes

Files without unsaved edits reload automatically when another program changes them on disk. Files over 64 MiB keep their current contents and show **↻ Reload** in the status bar instead. Click it, or run **Reload File from Disk** from the command palette, to load the latest contents. Reloading a file with unsaved edits asks for confirmation.

Set `editor.largeFiles.autoReloadMaxMB` to change the limit, or `0` to allow automatic reloads at any size. File watching remains active to detect changes.

## Multi-Cursor Editing

TTT supports editing with multiple cursors simultaneously. Place cursors at multiple locations and type, delete, or insert lines. All cursors act in parallel.

### Adding Cursors

- **Ctrl+D** selects the current word (or extends the current selection) and adds a cursor at the next occurrence
- **Ctrl+K L** selects all occurrences of the current word/selection at once
- **Alt+Click** adds a cursor at the clicked position

### Removing Cursors

- **Ctrl+K U** undoes the last cursor addition
- **Escape** collapses back to a single cursor (when multiple cursors are active)

### Behavior

- Typing, backspace, delete, and enter work at all cursor positions simultaneously
- Undo/redo groups multi-cursor edits into a single action
- The status bar shows the cursor count (e.g., "3 cursors") when multiple cursors are active

## Line Operations

- **Alt+Up / Alt+Down** moves the current line (or selected lines) up or down
- **Ctrl+K K** deletes the current line
- **Ctrl+Enter** inserts a new line below the cursor
- **Ctrl+K J** joins the current line with the next line

## Toggle Comment

Press **Ctrl+/** to toggle line comments on the current line or selection.

## Code Folding

TTT supports indent-based code folding:

- **Ctrl+K [** toggles the fold at the current line
- **Ctrl+K 0** collapses all foldable regions
- **Ctrl+K 9** expands all foldable regions

## Text Transforms

These commands are available from the command palette:

- **Transform to Uppercase** / **Transform to Lowercase** / **Transform to Titlecase**
- **Sort Lines Ascending** (**Ctrl+K O**) / **Sort Lines Descending**
- **Reverse Lines**
- **Unique Lines** (remove duplicates)
- **Trim Trailing Whitespace** (selected lines, or the whole file)
- **Split Selection into Lines**

## Word Operations

- **Ctrl+Home / Ctrl+End** moves the cursor to the start or end of the file; with **Shift** it selects
- **Ctrl+Left / Ctrl+Right** moves the cursor one word left or right
- **Ctrl+Shift+Left / Ctrl+Shift+Right** selects one word left or right
- **Alt+Backspace** deletes the word to the left
- **Alt+Delete** or **Ctrl+Delete** deletes the word to the right

## Indentation

TTT supports `.editorconfig` files and picks up indent size automatically per file. It also auto-detects indentation from file content. You can manually override indentation via the status bar indent picker.

## Mouse Support

- Click to position the cursor
- Click tabs to switch files
- Drag sidebar and panel dividers to resize
- Right-click for context menus

The sidebar resize handle and resize pointer cover only the divider column. Clicks and drags in the first content column beside it go directly to the content, including plugin tabs without a gutter.

Other resize allowances are intentionally retained until a separate change is requested:

- Left and right drawers accept a resize drag on their border or one column outside it.
- A bottom-docked panel also accepts a resize drag on the tab bar row immediately below its divider when the panel does not consume the press. Right-docked panels use only their divider column.

## Line Numbers

Line numbers are shown in the gutter with current-line highlighting. The gutter style can be set to `minimal`, `compact` (default), or `extended` via the `editor.gutterStyle` setting. Toggle line numbers on or off with the `lineNumbers` setting.

## Git Blame

Inline blame info for the current line is shown in the status bar, including the author, relative time, and commit summary.

## Git Gutter

When enabled, diff indicators appear in the line number gutter showing added, modified, and removed lines compared to the last committed version. Toggle with the "Toggle Git Gutter" command in the command palette.

When a file is taller than the editor, the same changes are marked on the vertical scrollbar in the gutter colors. Marks are placed to an eighth of a cell, and even a single changed line stays visible in a long file. Set `appearance.scrollbarGlyphs` to `"legacy"` for thin line marks if your terminal or font has the Symbols for Legacy Computing glyphs. Click a mark to scroll to that change. Turn the marks off with `editor.scrollbarGitMarks` or **Options > Scrollbar Git Marks**. Diff views can mark their changed rows too: turn on `editor.diffScrollbarGitMarks` or **Options > Diff Views > Scrollbar Git Marks**.
