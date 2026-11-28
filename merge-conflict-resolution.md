# Merge conflict resolution UX

Git text conflicts are the initial scope. jj support follows later.

## Goal

Add a visual merge editor that shows both versions of a conflicted file and an
editable result. Users select whole conflict blocks from either source and see
their choices reflected in Result. Reuse existing editor panes and gutter
facilities, keeping changes to the general editor small.

## Agreed direction

- Offer two visual layouts and the existing text conflict view.
- Select whole conflict blocks; use direct editing in Result for finer changes.
- Use clickable gutter icons to show and toggle inclusion of each source block.
- Keep both source panes read-only and Result editable.
- Make the feature discoverable from Changes and from the normal file editor.
- Expose the preferred conflict view alongside diff settings.

## Views

All three choices belong to one **Merge Conflict View** setting:

| Choice | Arrangement | Purpose |
| --- | --- | --- |
| Two Columns | Sources side by side above a full-width Result | Default; gives Result room for long lines |
| Three Columns | Source, Result, source across the screen | Useful on wide terminals; Result sits beside both inputs |
| Text | Existing editor with conflict markers and inline accept actions | Preserve the current workflow |

Source order follows VS Code: Incoming on the left, Current on the right. Show
available branch or revision labels so users can identify each source.

```text
Two Columns
┌───────────────────┬───────────────────┐
│ Incoming          │ Current           │
├───────────────────┴───────────────────┤
│ Result                                │
└───────────────────────────────────────┘

Three Columns
┌─────────────┬─────────────┬─────────────┐
│ Incoming    │ Result      │ Current     │
│             │             │             │
└─────────────┴─────────────┴─────────────┘
```

“Two Columns” still contains three panes. Descriptions in settings and the view
menu should explain where Result appears.

Another possible layout stacks Incoming and Current on the left with a
full-height Result on the right. It was raised during discussion but is not
part of the initial agreed scope. An optional common-base view is also a later
consideration, separate from the three main view choices.

## Discoverability and settings

- Opening a conflicted file from Changes uses the preferred conflict view.
- A conflicted file opened in the normal editor shows one persistent bar above
  the file: **Merge conflicts detected — Open Merge Editor**.
- Provide **Open Merge Editor** in the command palette and the file context
  menu when applicable.
- The visual merge editor has a visible **View** menu containing Two Columns,
  Three Columns, and Text, plus previous/next conflict controls and an unresolved
  conflict count.
- Switching views preserves choices, result edits, and undo history. Text and
  saved partial resolutions retain markers for unresolved conflicts and preserve
  completed edits. Richer selection state stays in the open merge session.
- Put **Merge Conflict View** under **Options > Diff Views** and in the Diff
  settings section. Keep it independent of ordinary split/unified diff mode.

The existing ordinary diff setting is `editor.diffMode`. Planned new key:
`editor.mergeConflictView`, with values `two-column`, `three-column`, and `text`.
Default: `two-column`.

The saved setting determines how conflicts open. Changing View inside a tab
affects only that tab, matching the distinction between saved diff defaults and
per-view overrides. If Text is the saved default, explicit **Open Merge Editor**
enters the Two Columns layout.

## Block selection and result editing

- Place an unchecked/checked gutter icon at the start of each source's conflict
  block. Highlight the block so the icon's scope is apparent.
- Clicking the icon toggles that source's contribution to the corresponding
  result region. Provide keyboard equivalents for selection and navigation.
- Allow choosing either side or both, and update Result immediately. Both means
  Incoming followed by Current, with **Swap Order** available for that block.
- Initialize Result with non-conflicting changes included. Conflicting regions
  remain visibly unresolved until handled.
- Preserve syntax highlighting, text selection, copy, and search in source panes.
- Make choices undoable together with their visible selection state.
- Keep navigation coordinated across all panes. Define scrolling/alignment
  behavior for unequal block lengths and wrapping.
- Treat manual edits in a conflicting region as a custom resolution, completed
  with an explicit **Mark Resolved** action. Confirm before a later source
  selection replaces those edits.
- Provide an explicit **Keep Neither** action. Track unresolved state separately
  from unchecked source icons.
- Normal Save saves progress. **Complete Merge** saves and stages the file once
  all conflicts are resolved. It does not commit or continue a rebase.

Whole-block selection follows the interaction described in the
[VS Code merge editor documentation](https://code.visualstudio.com/docs/sourcecontrol/merge-conflicts#use-the-3-way-merge-editor).
The agreed ttt workflow also supports direct result editing and an explicit
save-and-stage completion action.

## Implementation approach

### Compose existing editor panes

Introduce a dedicated `MergeEditorWidget` in `internal/ui` that composes three
`EditorPaneWidget`s. The merge widget owns layout, focus routing, conflict
navigation, selection presentation, and coordination between panes. Keep text
rendering and normal editing in the existing pane implementation.

Use the editable diff implementation as a reference for connecting a specialized
view to the file's buffer and undo history. Avoid creating competing editable
copies of the result. Reuse suitable diff highlighting and alignment primitives
without forcing three-way merge behavior into the two-way diff widget.

### Reuse gutter hooks

`EditorPaneWidget` already exposes `BookmarkOverlay` and `OnGutterClick`.
`internal/app/debugger.go` demonstrates displaying gutter icons through the
overlay. Use these hooks on the source panes to render selection icons and
handle toggles.

Keep merge selections in the merge model, separate from document bookmarks.
The public Lua bookmarks API operates on shared active-file bookmark state;
using it as merge storage could overwrite user/plugin bookmarks. Reusing the
underlying gutter hooks should not require changing the public bookmarks API.

Empty sides and deletions need an explicit clickable anchor because an absent
source block has no ordinary first line. Investigate existing filler/annotation
rows before adding any new editor hook.

### Merge state and source data

Keep conflict ranges, source choices, resolution state, and result mappings in a
testable model, separate from rendering. Route result mutations through undo
commands. Ensure manual edits, undo/redo, and view changes keep mappings valid.

The current `scm.ChangeDiff` conflict view supplies one provider-selected source
and the working file, rather than both sources plus a base. Add a typed optional
SCM capability for the required conflict inputs. Keep Git index-stage details
inside the Git adapter and service calls outside UI widgets.

Support Git text conflicts first. Defer jj support; the optional SCM capability
must leave room for it without pretending its conflict representation is
identical to Git's. Retain a usable text fallback for unsupported conflict types
or unavailable source data, including cases the initial visual editor cannot
represent faithfully.

### Integration constraints

- Preserve existing file save, dirty-state, close, and undo behavior.
- Store rendered geometry for hit testing rather than recalculating positions.
- Use the theme system for styling, `internal/icons` for glyphs, and
  `internal/i18n/en.json` for user-facing copy.
- Keep any new general editor hooks narrow and justified by a concrete need.
- Staging happens through the explicit **Complete Merge** action. Ordinary Save
  does not stage, and neither action commits or continues the repository operation.

## Implementation details to resolve

These are engineering details within the accepted behavior, rather than pending
product approvals:

- Represent unresolved result regions and serialize markers for partial saves
  without losing custom edits or violating undo history.
- Keep block identities and source choices consistent across manual edits,
  undo/redo, and Text/visual transitions. Reopening must preserve file content
  even when exact historical selection state cannot be reconstructed.
- Define capability checks and clear fallback behavior for standalone markers,
  missing sources, binary files, and unsupported delete/modify cases.
- Choose minimum pane widths, resize behavior, synchronized scrolling,
  alignment, divider behavior, and keyboard commands.
- Preserve unsaved work and reject stale source reads when repository or file
  state changes during a merge session.

## Implementation sequence

1. Characterize the existing inline conflict and editable-buffer behavior that
   must be preserved.
2. Implement typed source loading and the merge state model, with deterministic
   coverage for the chosen resolution semantics.
3. Compose the two visual layouts and gutter selection using existing panes.
4. Integrate result editing, undo, navigation, and lossless view transitions.
5. Add Changes/editor entry points, settings, translated copy, and documentation.
6. Verify the user flow in the real binary and capture before/after screenshots.

Use unit tests for merge state and source adapters, and composed UI/E2E tests
for gutter interaction, focus, layouts, view transitions, and undo. Add
real-binary tests only where they prove a distinct startup/save/dispatch
boundary. Run the i18n checks when adding copy and the repository's required
test and lint checks before opening a PR. Keep the source reader, merge state,
editor interaction, and app integration in focused commits.
