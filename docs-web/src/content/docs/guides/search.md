---
title: Search
description: Searching across files with ripgrep.
---

## Workspace Search

The sidebar search panel (**Ctrl+K F**) is powered by [ripgrep](https://github.com/BurntSushi/ripgrep). Results are grouped by file with match counts.

Invoking **Ctrl+K F** or **Ctrl+K R** prefills the search query from the active editor's single-line selection. Repeating the command with a new selection updates the query. Empty or multiline selections preserve the current query, as does switching sidebar panels. Prefilled queries use the configured search debounce.

- **Smart-case** matching by default
- **Include/Exclude glob filters**: click the toggle arrow to reveal filter inputs (e.g. `*.go`, `vendor/**`)
- Tab between search, include, and exclude inputs
- Searches across all workspace folders simultaneously
- Click a result to jump to the file and line
- Search debounce is configurable (default 350ms, setting: `search.debounce`)

### Search and Replace Across Files

Use **Ctrl+K R** to open the sidebar search and replace panel. This lets you find and replace text across all files in the workspace.

Cancelling the Replace All confirmation returns focus to the search panel and preserves the active input field.

## Find in File

- **Ctrl+F** opens the inline find bar
- **Ctrl+R** opens find and replace
- Both commands prefill the search field from a single-line selection; empty or multiline selections preserve an existing query
- **F3 / Shift+F3** to jump to next/previous match
- **Alt+C** toggles case sensitivity
- **Alt+R** / **Alt+X** toggles regex matching
- Replace-one and replace-all support
