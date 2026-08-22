---
title: Git Integration
description: Source control features built into TTT.
---

The changes panel in the sidebar (**Ctrl+K C**) provides a full staging workflow.

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

Select a changed file in the changes panel to open a side-by-side diff. Syntax highlighting is layered on top of diff background colors so you can read the code naturally while seeing what changed.

TTT offers two diff modes:

- **Partial diff view** shows only the changed hunks with surrounding context lines, letting you focus on what actually changed.
- **Full-file diff view** shows the complete file with changes highlighted inline.

Choose the default layout, context, and line wrapping from the Options menu. The Changes panel's three-dot menu keeps the same controls close to the files they affect. Collapsed context rows use a triangle in the line-number gutter; click one to reveal its omitted lines.

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
