# Audit decisions

Behavior that looks like a bug but is intended, or was reviewed and won't be fixed. Read this before reporting audit or regression findings, and skip anything listed here. Add new decisions in the same PR that makes them.

## By design

| Behavior | Reason | Source |
|----------|--------|--------|
| Confirm dialogs list the negative action first and focus item 0. "Save changes?" shows Discard / Cancel / Save with Discard focused. | App-wide dialog convention. | #1496 (finding 4) |
| Format on save does nothing for files with no recognized language, even with a `formatters.<ext>` entry. | No language means no format on save. Run the formatter by hand instead. | #1496 (finding 24) |
| Configured chords and reserved shortcuts take priority over the PTY while the terminal is focused, unless `terminal.forwardShortcuts` is on. | Documented in the terminal guide. Editor-scoped commands are excluded (#1520). | #1491 |
| With multiple cursors, Shift+arrows, Shift+Home and Shift+End move the cursors without extending selections. | Not supported; not promised by the docs. | #1496 |
| Duplicate Line and Delete Line act on the primary cursor only when multiple cursors are active. | Multi-cursor collapses before line commands. | #1496 |
| End on a soft-wrapped line moves to the end of the logical line. | Logical-line navigation. | #1496 |
| Custom `tab` / `shift+tab` bindings don't fire while the editor is focused. | The editor owns indent and outdent. Bindings work from other focus targets. | #1496 |
| The explorer filter only searches loaded (expanded) folders. | Documented. | #1470 |
| Undoing an edit on a folded header line unfolds the region. | Unfolding shows the user that something inside the fold changed. | BUG-024 (2026-07 audit) |
| The tab bar's overflow chevrons (◀ ▶) switch to the previous/next tab instead of scrolling the strip. | Tabs are navigated one at a time; the strip doesn't scroll on its own. | BUG-016 (2026-07 audit) |

## Won't fix

| Behavior | Reason | Source |
|----------|--------|--------|
| Saving `settings.json` from inside ttt doesn't apply it; a later settings change from the UI writes the in-memory settings back. | Saving a file only saves the file. **Reload Settings** applies manual edits. | #1517 |
| **Terminal: Close Terminal** closes the selected panel terminal, not a focused editor-tab terminal. | Panel and editor-tab terminals have separate close controls. Editor terminals close through their tab. | #1526 |
| Opening a file while the terminal is maximized focuses the editor without restoring the layout. | Maximizing is an explicit layout choice; restoring it is up to the user. | #1528 |
| Turning on word wrap leaves `⋯` fold markers visible (folding is disabled under wrap). | Cosmetic. | #1496 (finding 25) |
| Multi-cursor Backspace at line start / Delete at line end collapses to one cursor. | Deferred, not prioritized. Can be reopened. | #1496 (finding 1) |
| The status bar is hidden below about 60 columns. | Narrow widths are acceptable without it. Won't fix until requested. | BUG-036 (2026-07 audit) |
| Cursor movement and Backspace step through joined emoji (👨‍👩‍👧‍👦, skin-tone sequences) one piece at a time instead of as a single character. | Edge case. Fixing it means moving by grapheme cluster across cursor, delete and width code. Won't fix until requested. The `it.fails` test in `tests/functional/audit-grapheme-bugs.test.js` keeps documenting it. | BUG-009 (2026-07 audit) |

## Not reported (expected or out of scope)

- Quitting with running terminal processes doesn't prompt, and no processes are left behind.
- Ctrl+Q with a dirty buffer is ignored while an overlay is open.
- Explorer **New File** accepts `../` paths.
- Tabs for same-named files in different folders aren't disambiguated.

## Running an audit

- Drive the real binary headless with `--exec` (see AGENTS.md, "Debug harness"). Use a fresh `TTT_CONFIG_DIR` per run and fixture projects outside the repo, so your config isn't touched and repo plugins don't load.
- Don't run commands that open a browser, external app or file manager, check for updates, restart, or install plugins.
- Only report a finding that reproduces twice from a clean config and contradicts the docs or reasonable editor behavior. Include a minimal `--exec` repro.
- Collect findings on one tracking issue, one comment each, then split the confirmed ones into separate issues.
