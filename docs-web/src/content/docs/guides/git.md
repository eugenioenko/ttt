---
title: Git Integration
description: Source control features built into TTT.
---

The changes panel in the sidebar (**Ctrl+K C**) provides a full staging workflow.

TTT refreshes repository status after editor and source-control mutations. While the Changes panel is visible, it also polls the working tree and `HEAD`, so changes made by external tools appear without a manual refresh. Status-only polls reload commit history when `HEAD` changes; a manual refresh forces both.

## Staging & Unstaging

- **Spacebar** toggles stage/unstage on the selected file
- **`a`** stages all unstaged files
- **`u`** unstages all staged files
- **`+` button** on each unstaged file stages that file
- **`−` button** on each staged file unstages that file
- **`+` button** on the "Changes" section header stages all files
- **`−` button** on the "Staged" section header unstages all files

## Discarding Changes

- **`d`** discards changes to the selected unstaged file (with confirmation)
- **`D`** discards all unstaged changes in the current group (with confirmation)
- **`✕` button** on the "Changes" section header discards all unstaged changes
- Untracked files are deleted; modified files are restored to HEAD

## Committing

- Inline commit message input at the top of each group
- Type a message and press Enter to commit all staged files

## Remote Operations

- **Pull**, **Push**, **Sync** (pull then push) from the sidebar actions button
- Per-repo actions via the group header menu button in multi-root workspaces

## Diff View

Select a changed file in the changes panel to open a diff. Syntax highlighting is layered on top of diff background colors so you can read the code naturally while seeing what changed. Added and removed line numbers use `+` and `−` markers with semantic green and red styling.

The shared diff reader has two independent presentation choices:

- **Split** places old and new content side by side; **Unified** stacks removals before additions.
- **Changes Only** shows changed hunks and surrounding context; quiet disclosure rows expand omitted context in place. **Full File** shows the complete file with changes highlighted inline.

Set global defaults for layout, context, line wrapping, and high contrast from the **Diff Views** submenu under **Options**. The adjacent **Git Files** submenu chooses a persisted full-path List (the default) or compact Tree and exposes bulk expansion controls. The Changes panel menu exposes the same choices contextually. Commands applied directly to an open diff override that surface without changing the saved defaults.

Untracked files open directly in the editor instead of showing a diff.

## Multi-Root Support

When working with multiple folders:

- Changes are grouped by repository, each with its own collapsible Staged/Changes sections
- Each group has a commit input and a menu button for pull/push/sync on that specific repo
- File status badges: **M** (modified), **A** (added), **D** (deleted), **R** (renamed), **U** (untracked)

## Pull Request Review

You can review GitHub pull requests directly in TTT without cloning the branch or switching contexts. Pass a PR URL on the command line:

```sh
ttt https://github.com/owner/repo/pull/123
```

This opens the PR's changed files in the changes panel. Select any file to see its diff with full syntax highlighting.

To review a PR alongside your local repository, pass both:

```sh
ttt . https://github.com/owner/repo/pull/123
```

This gives you the local file tree in the explorer and the PR's changed files in the changes panel, so you can cross-reference the PR against the existing codebase.

## Git Gutter

The line number area displays diff indicators that show which lines have been added, modified, or deleted compared to the last commit. This gives you at-a-glance visibility into your uncommitted changes as you edit.

## Git Blame

The status bar shows inline blame info for the current line: author, relative time, and commit summary.
