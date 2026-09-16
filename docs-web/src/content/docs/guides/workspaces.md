---
title: File Explorer & Workspaces
description: Working with files, folders, and multi-root workspaces.
---

## File Explorer

The file explorer lives in the sidebar (**Ctrl+B** to toggle, **Ctrl+K E** to focus).

- Directories are sorted before files, both alphabetically
- Expand/collapse with Enter or arrow keys
- Right-click context menu: **New File**, **New Folder**, **Rename**, **Delete**
- Sidebar actions button for **Refresh** and **New File**
- File-type icons, which need a [Nerd Font](https://www.nerdfonts.com) in your terminal. They default off, since without one they'd render as boxes: turn them on with `"explorer": { "icons": "nerd-font" }`, **Options → Font Icons**, or **Explorer: file icons** in the settings editor

When multiple folders are open, each root is shown as a collapsible group.

## Quick Navigation

- **Ctrl+O** to open a folder (`workspace.openFolder`)
- **Ctrl+K P** for quick file open with fuzzy search (`file.quickOpen`)

## Opening Files and Folders

```sh
ttt                             # opens the current directory
ttt .                           # also opens the current directory
ttt /path/to/dir                # opens that directory as the workspace
ttt /path/to/file.go            # opens just the file — no workspace folder
ttt /path/to/repo/subdir        # opens that folder; git features (changes,
                                # branch) use the enclosing git repo root
ttt dir1 dir2                   # opens multiple folders as a multi-root workspace
ttt --workspace project.ttt     # loads a saved workspace file

# Review a GitHub pull request
ttt https://github.com/owner/repo/pull/123

# Review a PR with the repo tree open
ttt . https://github.com/owner/repo/pull/123
```

## Multi-Folder Workspaces

Open multiple project directories in a single session. Each root appears as a collapsible group in the explorer, search, and changes panels.

### Workspace Files

Workspace files use the `.ttt` extension and store a list of folders as relative paths:

```json
{
  "folders": [
    { "path": "." },
    { "path": "../other-project" }
  ]
}
```

### Managing Workspaces

- **Save Workspace As...** from the File menu to create a workspace file
- **Add Folder to Workspace** and **Remove Folder from Workspace** via the command palette
- The git branch in the status bar switches automatically based on which workspace folder the active file belongs to

## Tabs

Tabs follow a pin-on-reclick model similar to VS Code:

- Opening a file from the explorer or search replaces the current unpinned tab
- Clicking an already-open tab (or opening the same file again) pins it
- **Ctrl+W** to close a tab, **Alt+.** / **Alt+,** to switch tabs
- Right-click a tab for **Close**, **Close Others**, **Close All**
- Tab bar actions button for **Close All**
