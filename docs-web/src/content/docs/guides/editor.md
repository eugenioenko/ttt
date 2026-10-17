---
title: Editor Basics
description: Core editing features in TTT.
---

## Syntax Highlighting

TTT highlights code with [textmate-go](https://github.com/eugenioenko/textmate-go), a pure Go port of VS Code's TextMate engine built for ttt. It embeds TextMate grammars for 124 languages, detected from the file name, and colors constructs that span lines (block comments, template literals, docstrings, tags with attributes on several lines) correctly.

## Large Files and Log Mode

Files over 64 MiB open in **log mode**, a read-only viewer that reads visible lines from disk. Use **Open in Log Mode** in the command palette or a file's Explorer context menu for smaller files. Set `editor.largeFiles.logModeMinMB` to another size, `0` for every text file, or `false` for manual activation only. The threshold is checked when opening a new tab; existing editor tabs keep their mode.

**Ctrl+End** reads the tail immediately and follows appended lines. Scrolling up or pressing Page Up pauses following; Ctrl+End resumes. The **LOG** item before the cursor position shows following and indexing progress. Click it, or run **Open in Editor**, to confirm loading the file into an editable tab. Large loads warn about time and memory use.

**Ctrl+F** searches the whole file in the background, including lines beyond the current index. **F3 / Shift+F3** move between matches. The find bar shows search progress; queries stop at 10,000 matches with a notice to refine the query. Go to Line works on indexed lines and reports when a requested line is not indexed yet. Line numbers appear as indexing reaches the view.

Selection works with the mouse, Shift and the navigation keys, and Ctrl+A. Ctrl+C copies up to 4 MiB, with a notice when the selection is capped. Each displayed line is capped at 1 MiB and marked when cut off; search still scans its full text. Long lines scroll horizontally. Log mode has no editing, undo, word wrap, highlighting, folds, language-server work, or git gutter. Truncating or replacing the file resets the view with a notice.

Normal editor tabs disable word wrap above `editor.largeFiles.wordWrapMaxMB` (32 MiB by default). If a clean editor file grows beyond `editor.largeFiles.autoReloadMaxMB` (64 MiB), changes on disk show a persistent **↻ Reload** item offering **Reload** and **Open in Log Mode**. **Reload File from Disk** also works from the command palette and confirms discarding unsaved edits. Set either limit to `0` to remove it. One megabyte in these settings is 1,048,576 bytes.

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

## Line Numbers

Line numbers are shown in the gutter with current-line highlighting. The gutter style can be set to `minimal`, `compact` (default), or `extended` via the `editor.gutterStyle` setting. Toggle line numbers on or off with the `lineNumbers` setting.

## Git Blame

Inline blame info for the current line is shown in the status bar, including the author, relative time, and commit summary.

## Git Gutter

When enabled, diff indicators appear in the line number gutter showing added, modified, and removed lines compared to the last committed version. Toggle with the "Toggle Git Gutter" command in the command palette.

When a file is taller than the editor, the same changes are marked on the vertical scrollbar in the gutter colors. Marks are placed to an eighth of a cell, and even a single changed line stays visible in a long file. Set `appearance.scrollbarGlyphs` to `"legacy"` for thin line marks if your terminal or font has the Symbols for Legacy Computing glyphs. Click a mark to scroll to that change. Turn the marks off with `editor.scrollbarGitMarks` or **Options > Scrollbar Git Marks**. Diff views can mark their changed rows too: turn on `editor.diffScrollbarGitMarks` or **Options > Diff Views > Scrollbar Git Marks**.
