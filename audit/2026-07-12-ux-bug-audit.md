# UX Bug Audit

Started 2026-07-11 on branch `audit/bug-hunt`. Goal: **discover and document** UX bugs — no fixes on this branch. Each confirmed finding gets a ledger entry below and, where feasible, a repro test marked as expected-failure (`t.Skip("BUG-NNN")` in Go, `it.fails(...)` in vitest) so the suite stays green until the bug is fixed.

Process: one hunting agent at a time, scoped to an area from the coverage matrix. Orchestrator re-verifies every repro before it enters this file. LSP is out of scope.

**Resuming this hunt?** See [`## Resume guide`](#resume-guide) at the bottom: the orchestration loop, harness state, standing dump gaps, and ready-to-paste agent prompts for every remaining area.

## Status: discovery phase COMPLETE (all 19 areas swept)

**59 confirmed findings (BUG-001..059)** — 20 high, 29 medium, 10 low (pre-curation counts; a **curation pass is in progress** — see per-entry `Curation:` lines for downgrades/rejections, recount at the end). 37 have expected-failure repro tests (vitest `it.fails` / Go `t.Skip`); 22 are ledger-only (feedback signal undefined until fixed, tiny-size-only, PTY/timing-dependent, or design questions). Every repro was re-verified by the orchestrator before entry. No fixes on this branch.

**Recurring root-cause clusters** (fix these, not 59 individual bugs):
- Line-range commands ignore the col-0 selection convention -> BUG-001..004
- Primary-cursor-only ops ignore `e.Multi` -> BUG-005..008
- Missing `BatchCommand` (non-atomic undo) -> BUG-012, 021, 022
- Undo restores text but not the user's view (cursor/viewport) -> BUG-020, 023
- Editor search/find state shared across tabs, not reset on switch -> BUG-013, 048
- File ops mutate disk without stat-guard or tab-model reconciliation -> BUG-028..031
- Fold state keyed by raw line number, no content anchor -> BUG-024, 026, 027
- Focus/key-routing gaps around overlays & the terminal -> BUG-057, 058, 059

**Data-loss / trapping highs to fix first:** BUG-028/029/030 (explorer wipes/clobbers/misdirects saves), BUG-052 (plugin freezes editor), BUG-057 (typing corrupts file after dialog).

## Coverage matrix

| Area | Status | Findings |
|---|---|---|
| Editing commands × selection | swept (4 findings) | BUG-001..004 |
| Multicursor interactions | swept (4 findings) | BUG-005..008 |
| Undo/redo semantics | swept (6 findings) | BUG-020..025 |
| Code folding × editing | swept (2 findings) | BUG-026, BUG-027 |
| Find/replace + search highlights | swept (6 findings) | BUG-010..015 |
| Tabs & split panes | swept (1 finding; split panes N/A — feature doesn't exist) | BUG-016 |
| Explorer (file tree) | swept (9 findings) | BUG-028..035 |
| Global search (sidebar, rg-based) | swept (4 findings) | BUG-047..050 |
| Mouse targets / click offsets | swept (2 findings) | BUG-018, BUG-019 |
| Resize & layout | swept (5 findings) | BUG-036..040 |
| Wide-char / edge content (CJK, emoji, tabs, long lines) | swept (1 finding) | BUG-009 |
| Keyboard navigation parity | swept (2 findings) | BUG-017, BUG-051 |
| Themes & rendering | swept (2 findings) | BUG-041, BUG-042 |
| Settings & options | swept (clean) | — |
| Workspace (multi-folder) | swept (4 findings) | BUG-043..046 |
| Plugin widgets | swept (5 findings) | BUG-052..056 |
| Integrated terminal panel | swept (3 findings) | BUG-057..059 |

Status values: `pending` → `in progress` → `swept (N findings)` / `swept (clean)`.

## Findings

### BUG-001: Move Line Up/Down includes trailing col-0 selection line and swaps the invisible trailing empty line into the buffer
- **Area:** Editing commands × selection
- **Severity:** medium  *(was high — see Curation)*
- **Curation (2026-07-12, CONFIRMED, downgraded high→medium):** genuine — code-confirmed. `MoveLineUp/Down` (`editor_widget_lines.go`) iterate `start.Line..end.Line` with NO col-0 adjustment, unlike `JoinLines`/`ToggleLineComment` which do `if end.Col == 0 && endLine > start.Line { endLine-- }` (same file, 3 copies). Plus the EOF guard `end.Line >= len(Buf.Lines)-1` counts the invisible trailing `""` of a `\n`-terminated file, so it's off by one and swaps that phantom line into the buffer (injects a blank line). Downgraded to medium: real corruption but visible + undoable, triggered by the common select-lines-then-move workflow. **First of the col-0 cluster [[BUG-002]]/[[BUG-003]]/[[BUG-004]] — shared fix: a `lineRange()` helper applying the col-0 convention that ALL line commands call; 001 also needs the EOF guard to use the visible line count.**
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** file `line0\nline1\nline2\nline3\nline4\n`; `bin/ttt --size 120x40 --exec "wait 200; key down; key down; key shift+down; key shift+down; exec \"Move Line Down\"; screenshot /tmp/s.txt; quit" file.txt`
- **Expected:** selection line2→line4-col0 covers lines 2–3 (col-0 convention per JoinLines/ToggleLineComment); block swaps past line4 → `line0,line1,line4,line2,line3`
- **Actual:** a blank line appears between line1 and line2 (visible buffer grows 5→6 rows): `MoveLineDown`/`MoveLineUp` (`internal/ui/editor_widget_lines.go:33-54`) apply no col-0 adjustment, and the EOF guard uses the internal line count, which includes the invisible trailing empty line of any `\n`-terminated file — so that phantom line gets swapped into the middle of the file. Buffer marked modified; undo does restore correctly.
- **Test:** `tests/functional/audit-selection-bugs.test.js` (`it.fails`)

### BUG-002: Indent (Tab) with selection ending at col 0 indents one line too many
- **Area:** Editing commands × selection
- **Severity:** low  *(was medium — see Curation)*
- **Curation (2026-07-12, CONFIRMED, downgraded medium→low):** genuine, same root as [[BUG-001]] — KeyTab handler (`editor_widget_keyboard.go:239-243`) iterates `start.Line..end.Line` with no col-0 exclusion; `ToggleLineComment` with the identical selection is the correct control. Mildest of the cluster: a one-line over-indent, visible + undoable, no corruption. Shared fix: the `lineRange()` col-0 helper (see [[BUG-001]]).
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** file `line0\nline1\nline2\nline3\nline4\n`; `bin/ttt --size 120x40 --exec "wait 200; key down; key shift+down; key tab; screenshot /tmp/s.txt; quit" file.txt`
- **Expected:** selection line1→line2-col0 covers only line1 (control: `Toggle Line Comment` with the identical selection correctly comments only line1) → only line1 indented
- **Actual:** line1 AND line2 both indented — the KeyTab handler (`internal/ui/editor_widget_keyboard.go:238-247`) iterates `start.Line..end.Line` with no col-0 exclusion
- **Test:** `tests/functional/audit-selection-bugs.test.js` (`it.fails`)

### BUG-003: Duplicate Line and Delete Line ignore an active multi-line selection
- **Area:** Editing commands × selection
- **Severity:** medium
- **Curation (2026-07-12, CONFIRMED, kept medium):** genuine — `DuplicateLine`/`DeleteLine` (`editor_widget_lines.go`) only read `e.Cursor.Line`, never `e.Selection` (a different flavor from the col-0 off-by-one: zero selection-awareness). With lines selected, Duplicate copies the cursor line and Delete removes the cursor line (not the selected block), leaving a stale selection — a DIFFERENT line than selected gets deleted, which is genuinely confusing (kept at medium). VS Code makes these selection-aware. Shared fix: route through the [[BUG-001]] `lineRange()` helper so they become selection-aware AND col-0-correct at once.
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** file `line0\nline1\nline2\nline3\nline4\n`; `bin/ttt --size 120x40 --exec "wait 200; key down; key shift+down; key shift+down; exec \"Duplicate Line\"; screenshot /tmp/s.txt; quit" file.txt` (same shape with `exec "Delete Line"`)
- **Expected:** per the project convention ("line-based commands operate on the selected lines"), with lines 1–2 selected: Duplicate Line duplicates the block; Delete Line deletes it
- **Actual:** `DuplicateLine()`/`DeleteLine()` (`internal/ui/editor_widget_lines.go:56-83`) never consult the selection — both act only on the cursor's line (line3, not even part of the selection per the col-0 convention). Delete Line additionally leaves a stale selection range pointing past the shifted buffer.
- **Test:** `tests/functional/audit-selection-bugs.test.js` (`it.fails`, 2 cases)

### BUG-004: Outdent (Backtab) with selection ending at col 0 outdents one line too many
- **Area:** Editing commands × selection
- **Severity:** low  *(was medium — see Curation)*
- **Curation (2026-07-12, CONFIRMED, downgraded medium→low):** genuine, symmetric twin of [[BUG-002]] — KeyBacktab handler (`editor_widget_keyboard.go:208-219`) iterates `start.Line..end.Line`, clamps `end.Line`, but has no col-0 exclusion. Mild one-line over-outdent, visible + undoable. Only e2e-testable (`--exec` shift+tab → KeyTab+ModShift, not KeyBacktab). Completes the col-0 cluster (001-004, all genuine); shared `lineRange()` fix (see [[BUG-001]]).
- **Status:** confirmed (agent code-inspection suspicion, orchestrator confirmed at runtime via e2e)
- **Repro:** not drivable via `--exec` (`key shift+tab` synthesizes `KeyTab+ModShift`, not `KeyBacktab` — harness gap, see below); e2e test injects `tcell.KeyBacktab` directly
- **Expected:** selection line0→line1-col0 covers only line0 → only line0 outdented
- **Actual:** line1 outdented as well — `KeyBacktab` handler (`internal/ui/editor_widget_keyboard.go:201-232`) iterates `start.Line..end.Line` with no col-0 exclusion, same defect as BUG-002
- **Test:** `tests/e2e/audit_selection_bugs_test.go` (`t.Skip`-marked; verified failing when unskipped)

### BUG-005: Line commands under multicursor leave `e.Multi` stale — next keystroke corrupts the buffer
- **Area:** Multicursor interactions
- **Severity:** high
- **Curation (2026-07-13, CONFIRMED, kept high):** code-confirmed — `grep Multi internal/ui/editor_widget_lines.go` returns nothing, so `DuplicateLine`/`DeleteLine`/`MoveLineUp/Down` never touch `e.Multi.Cursors`; secondary cursors keep stale offsets after the line shift and the next multicursor keystroke edits at those offsets → silent buffer corruption (runtime-verified during the hunt). Kept **high** (silent, destructive buffer corruption on a headline feature) with the honest caveat that the trigger is a compound sequence (multicursor active + a *line* command + keep typing). First of the multicursor cluster [[BUG-006]]/[[BUG-007]]/[[BUG-008]], shared root "primary-cursor-only ops ignore `e.Multi`." **Fix for line commands specifically: collapse multicursor to the primary before running them** (a line command under N cursors is ambiguous); 006/007/008 need the "apply to all cursors" treatment instead.
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** file `foo bar foo baz\nfoo qux\nbar foo end\n`; `bin/ttt --size 120x40 --exec 'wait 200; key ctrl+k l; exec "Duplicate Line"; type Y; screenshot /tmp/s.txt; quit' foo.txt`
- **Expected:** line commands while multicursor is active either shift `Multi.Cursors` consistently (like `multiExecEnter` does) or collapse multicursor mode; typing afterwards must not touch text no cursor covers
- **Actual:** buffer corrupted — `Yar foo baz` / `foo Y` / `bar foo end` (characters clobbered in words that were never selected). Same pattern with Delete Line and Move Line Down. Root cause: `DuplicateLine`/`DeleteLine`/`MoveLineUp`/`MoveLineDown` (`internal/ui/editor_widget_lines.go`) never read or update `e.Multi.Cursors`.
- **Test:** `tests/functional/audit-multicursor-bugs.test.js` (`it.fails`)

### BUG-006: Case transforms under multicursor only affect the primary cursor
- **Area:** Multicursor interactions
- **Severity:** medium
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** same file; `bin/ttt --size 120x40 --exec 'wait 200; key ctrl+k l; exec "Transform to Uppercase"; screenshot /tmp/s.txt; quit' foo.txt`
- **Expected:** all 4 selected "foo" occurrences become "FOO"
- **Actual:** only the first is uppercased; the other 3 are silently ignored while the status bar reports "(4 cursors)". Root cause: `transformSelection` (`internal/ui/editor_widget_text.go:163`) reads only the primary `e.Selection`.
- **Test:** `tests/functional/audit-multicursor-bugs.test.js` (`it.fails`)

### BUG-007: Paste (and cut/copy) under multicursor only applies to the primary selection
- **Area:** Multicursor interactions
- **Severity:** high
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** same file; copy "bar", `key ctrl+k l`, `key ctrl+v`
- **Expected:** paste replaces every cursor's selection (VS Code semantics), or is an explicit no-op under multicursor
- **Actual:** only the primary "foo" becomes "bar"; the other 3 untouched, status bar still "(4 cursors)". Root cause: `EditorGroupWidget.Paste`/`Copy`/`Cut` (`internal/ui/editor_group.go:1262-1322`) and `pasteText` (`internal/ui/editor_widget.go:290`) never consult `e.Multi`.
- **Test:** `tests/functional/audit-multicursor-bugs.test.js` (`it.fails`)

### BUG-008: Undo after a multicursor edit strands the cursor and leaves `e.Multi` stale — next keystroke corrupts
- **Area:** Multicursor interactions
- **Severity:** high
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** same file; `bin/ttt --size 120x40 --exec 'wait 200; key ctrl+k l; type X; key ctrl+z; type Z; screenshot /tmp/s.txt; quit' foo.txt`
- **Expected:** undo restores text and either restores consistent multicursor selections or collapses to single cursor at the primary's pre-edit position
- **Actual:** text restores, but the cursor jumps to the last secondary cursor's stale post-edit position, "(4 cursors)" persists, and typing "Z" corrupts: `foo barZ foo baz` / `fZoo qux` / `bar fZoZo end` (two Z's from one keystroke). Root cause: undo (`internal/core/undo`) has no concept of `e.Multi`, so stale post-edit offsets survive into the reverted buffer.
- **Test:** `tests/functional/audit-multicursor-bugs.test.js` (`it.fails`)

### BUG-009: Cursor movement and backspace split ZWJ grapheme clusters
- **Area:** Wide-char / edge content
- **Severity:** medium
- **Status:** confirmed (orchestrator spot-check; the area agent's sweep missed it and reported clean)
- **Repro:** file `a👨‍👩‍👧‍👦b\n`; `bin/ttt --size 80x15 --exec 'wait 200; key right; key right; debug /tmp/d.json; key backspace; screenshot /tmp/s.txt; quit' zwj.txt`
- **Expected:** the family emoji (7 runes: 👨 ZWJ 👩 ZWJ 👧 ZWJ 👦) is one grapheme cluster — arrow keys cross it in one press, backspace deletes it whole (VS Code behavior)
- **Actual:** cursor stops mid-cluster (rune col 2 after two rights); backspace deletes only the 👨 rune, leaving a dangling ZWJ in the buffer and the emoji rendering exploded as `a 👩 👧 👦b`. Movement/deletion is rune-based everywhere, with no grapheme-cluster segmentation. Combining accents (e + U+0301) are presumably the same family — not separately verified.
- **Note:** rune-based `Col` is a documented design constraint, so the fix is a design decision (grapheme segmentation layer), not a one-liner. Plain CJK, skin-tone-free emoji, tabs, long lines, and boundary cases all passed the sweep — this is specifically about multi-rune clusters.
- **Test:** `tests/functional/audit-grapheme-bugs.test.js` (`it.fails`)

### BUG-010: Search matches go stale after buffer edits — Find Next jumps to non-matching lines
- **Area:** Find/replace
- **Severity:** high
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** open find (`ctrl+f`, query `alpha`), click into the editor, insert a line above the matches, press F3
- **Expected:** matches shift with the edited text (or are recomputed); Find Next never lands on a non-matching line
- **Actual:** F3 jumps to the stale line index — cursor lands on "beta". `SearchMatches` is never recomputed after buffer edits while the bar is open.
- **Test:** `tests/functional/audit-findreplace-bugs.test.js` (`it.fails`)

### BUG-011: Replace All ignores the Case-Sensitive/Regex toggles the bar itself displays — ✅ FIXED
- **Area:** Find/replace
- **Severity:** high
- **Status:** ✅ **FIXED** — `ReplaceBarWidget.OnReplaceAll` now carries the bar's current `Options` (case/regex), threaded through the `commands_search.go` callback into `EditorGroupWidget.ReplaceAll(query, replacement, opts)`, which passes them to `FindInLines` instead of a fresh `SearchOptions{}`. (The sidebar global-search path already did this; only the in-editor replace bar was broken.) Verified: full functional suite green, repro test flipped `it.fails`→`it`.
- **Repro (now fixed):** `ctrl+r`, query `Foo`, `alt+c` (bar shows 1/1), replacement `X`, `alt+r` → only "Foo" replaced
- **Test:** `tests/functional/audit-findreplace-bugs.test.js` (now real `it`)

### BUG-012: Replace All is not atomic in undo — one Ctrl+Z leaves a never-seen garbled state
- **Area:** Find/replace × undo
- **Severity:** medium
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** replace-all `cat`→`dog` over 4 matches, escape, one `ctrl+z`
- **Expected:** single undo step reverts the whole replace-all (VS Code semantics)
- **Actual:** every replacement pushes 2 ungrouped commands (DeleteSelection + InsertString) — 8 undos to fully revert; one undo yields `" dog dog"`, a state that never existed on screen
- **Test:** `tests/functional/audit-findreplace-bugs.test.js` (`it.fails`)

### BUG-013: Search state survives tab switches — Find Next navigates a tab that has no matches
- **Area:** Find/replace × tabs
- **Severity:** high
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** find `alpha` in tab A (1 match), switch to tab B (no matches), press F3
- **Expected:** bar clears or re-searches on tab switch; F3 in a matchless buffer is a no-op
- **Actual:** tab A's matches persist ("1/1" still shown for tab B); F3 clamps the stale line-5 target into tab B's 2-line buffer and moves the cursor to line 1 — meaningless navigation (no crash; line is clamped)
- **Test:** `tests/functional/audit-findreplace-bugs.test.js` (`it.fails`)

### BUG-014: Replace bar unconditionally swallows all keys — global bindings (tab-switch, tab-close) dead while open
- **Area:** Find/replace × keybindings
- **Severity:** medium
- **Status:** confirmed (agent-reported, orchestrator re-verified with find-bar control run)
- **Repro:** `ctrl+r` open, press `alt+.` (tab.next) — nothing happens; same key with only `ctrl+f` open switches tabs fine
- **Expected:** keys the replace input doesn't handle fall through to global handling, matching the find bar's behavior
- **Actual:** `ReplaceBarWidget.handleKey` (`internal/ui/replacebar_widget.go`) ends with an unconditional `return EventConsumed`
- **Test:** `tests/functional/audit-findreplace-bugs.test.js` (`it.fails`)

### BUG-015: Find does not seed the query from the active selection
- **Area:** Find/replace
- **Severity:** low (VS Code parity)
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** select "world", `ctrl+f`
- **Expected:** query box pre-filled with "world" (VS Code behavior; selection survives either way)
- **Actual:** empty "Search" placeholder; selection does survive
- **Test:** `tests/functional/audit-findreplace-bugs.test.js` (`it.fails`)

### BUG-016: Tab-bar overflow chevron switches the active tab instead of scrolling the strip
- **Area:** Tabs
- **Severity:** medium
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** open 5 files at `--size 50x20` (strip overflows); `click 2 2` on the `◀` chevron → `active_tab` goes 4→3
- **Expected:** chevrons scroll the tab strip to reveal hidden tabs without changing the active file (the code's own comment in `internal/ui/tabbar_widget.go` says "Scroll only when there is something hidden in that direction")
- **Actual:** chevron click calls `g.PrevTab()`/`g.NextTab()` (`internal/ui/editor_group.go:127-128`) — it changes the open file by one per click
- **Test:** `tests/functional/audit-tabbar-bugs.test.js` (`it.fails`)

### BUG-017: Ctrl+Home / Ctrl+End (document start/end) do nothing
- **Area:** Keyboard navigation parity
- **Severity:** medium
- **Status:** confirmed (orchestrator, found while validating the viewport dump field)
- **Repro:** 100-line file; `bin/ttt --size 80x20 --exec 'wait 200; key ctrl+end; debug /tmp/d.json; quit'` → cursor stays at 0,0
- **Expected:** Ctrl+End jumps to end of document, Ctrl+Home to start (plus shift variants for selection) — universal editor behavior, VS Code parity
- **Actual:** the `KeyHome`/`KeyEnd` handlers (`internal/ui/editor_widget_keyboard.go:125-151`) never check ModCtrl, no document start/end command exists in the registry, and nothing is bound — the keypress is silently dropped (not even line home/end fires)
- **Test:** `tests/functional/audit-navigation-bugs.test.js` (`it.fails`)

### BUG-020: Undo of line commands never restores the cursor to the edit site
- **Area:** Undo/redo
- **Severity:** high
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** Delete Line at line 2, move away, `ctrl+z` → text restores but cursor stays put
- **Expected:** undo returns the cursor to where the edit happened (as it does for typed text, paste, join lines)
- **Actual:** `cursorAfterUndo`/`cursorAfterRedo` (`internal/core/undo/undo.go`) have no cases for `InsertLineCommand`/`DeleteLineCommand`/`SwapLineCommand`/`ReplaceLinesCommand`, so `Undo()` returns nil and `editor_group.go:816` skips the cursor update. Affects Delete/Duplicate/Move/Sort Lines.
- **Test:** `tests/functional/audit-undo-bugs.test.js` (`it.fails`)

### BUG-021: Multi-line indent/outdent is one undo step per line, not atomic
- **Area:** Undo/redo
- **Severity:** high
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** select 3 lines, Tab, one `ctrl+z` → only the last line un-indents
- **Expected:** one undo reverts the whole indent (as Toggle Line Comment does via `BatchCommand`)
- **Actual:** KeyTab/KeyBacktab loop issues one top-level `exec` per line with no batch (`internal/ui/editor_widget_keyboard.go:240-247`)
- **Test:** `tests/functional/audit-undo-bugs.test.js` (`it.fails`)

### BUG-022: Enter with auto-indent takes 2–4 undos to revert; first undo can be a no-op
- **Area:** Undo/redo
- **Severity:** high
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** Enter at end of an indented line, one `ctrl+z` → stray blank line remains (only the auto-indent whitespace was undone). Bracket-pair Enter (`{`|`}`) takes 4 undos, the first a visible no-op.
- **Expected:** one undo fully reverts one keypress
- **Actual:** `execEnter()` (`internal/ui/editor_widget_keyboard.go:259-301`) issues 2–4 separate top-level exec calls, unbatched
- **Test:** `tests/functional/audit-undo-bugs.test.js` (`it.fails`)

### BUG-023: Viewport does not scroll to an off-screen undo location
- **Area:** Undo/redo
- **Severity:** medium
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** edit line 90, jump to line 1, `ctrl+z` → `cursor.line` 89, `viewport.top_line` 0 — cursor invisible
- **Expected:** undo scrolls the restored cursor into view like every other cursor-moving path
- **Actual:** `EditorGroupWidget.Undo()`/`Redo()` (`internal/ui/editor_group.go:810-842`) never call `scrollViewport()`
- **Test:** `tests/functional/audit-undo-bugs.test.js` (`it.fails`)

### BUG-024: Undoing an edit on a folded header line silently unfolds the region
- **Area:** Undo/redo × folding
- **Severity:** medium
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** fold a function, type a char on the header line, `ctrl+z` → buffer byte-identical to pre-edit, fold expanded
- **Expected:** fold state survives an undo that restores the exact pre-edit text
- **Actual:** fold collapsed-state is dropped on the edit and not restored by undo
- **Test:** `tests/functional/audit-undo-bugs.test.js` (`it.fails`)

### BUG-025: Undo grouping only breaks on whitespace — punctuation runs undo as one blob
- **Area:** Undo/redo
- **Severity:** low (possibly intentional — design question)
- **Status:** confirmed behavior; whether it's a bug needs an owner decision
- **Repro:** type `a.b.c.d.e`, one `ctrl+z` → entire string removed
- **Expected (VS Code-ish):** punctuation acts as a group delimiter like whitespace
- **Actual:** `canGroup()` (`internal/core/undo/undo.go:73-96`) only breaks groups on space/tab
- **Test:** none — behavior is a design choice; test would prescribe an undecided policy

### Harness gap from the undo sweep
No `folds` field in the debug dump (collapsed ranges) — BUG-024 had to be confirmed via screenshot fold markers. Add fold state if the folding sweep needs it.

### BUG-051: Goal column not preserved through a shorter line (feature documented but unimplemented)
- **Area:** Keyboard navigation parity
- **Severity:** medium
- **Status:** confirmed (orchestrator spot-check of a clean sweep — agent missed it; runtime + code confirmed)
- **Repro:** col 12 on a 20-char line, `down` through a 5-char line, `down` to a 20-char line → cursor at col 5, not 12. Also down-through-short-then-up returns col 5. A single down to a long line (no intervening short) keeps col 12.
- **Expected:** goal/preferred column survives passing through shorter lines (standard editor behavior; CLAUDE.md explicitly documents "cursor — Visual column cursor with goal-column preservation for vertical movement")
- **Actual:** the `Cursor` struct (`internal/core/cursor/cursor.go`) has only `Line`/`Col` — NO goal field. KeyUp/KeyDown (`internal/ui/editor_widget_keyboard.go:52-55, 79-82`) do `e.Cursor.Col = lineLen` on a shorter line, permanently overwriting the column. The documented feature isn't implemented (or was removed).
- **Test:** `tests/functional/audit-goalcolumn-bug.test.js` (`it.fails`, marker-based)

### Keyboard-nav area notes
Verified correct (Haiku sweep + orchestrator spot-check): word motions (ctrl+left/right) treat camelCase and snake_case as single words and `.`/punctuation as boundaries (matches VS Code); SmartHome toggles first-non-space ↔ col 0; PgUp/PgDn viewport + clamp; matching-bracket jump; go-to-line bounds; shift+ selection variants. The clean sweep MISSED BUG-051 (goal column) — caught only by the orchestrator's skeptical clamp-then-restore probe; single-move column tests pass, which is why a surface sweep reads clean.

### BUG-057: Dismissing a dialog while the terminal is focused steals focus to the editor — typing corrupts the file
- **Area:** Integrated terminal × focus
- **Severity:** high
- **Status:** confirmed (agent-reported, orchestrator re-verified via --exec — file corrupted, dirty)
- **Repro:** `bin/ttt --exec 'wait 300; key ctrl+t; wait 500; key ctrl+p; wait 300; key escape; wait 200; type XY; ...' f.txt` (f.txt = "hello world") → buffer becomes "XYhello world", modified:true; the "XY" meant for the shell lands in the editor
- **Expected:** dismissing a dialog returns focus to whatever was focused before (the terminal)
- **Actual:** `App.DismissDialog()` (`internal/app/app.go:557`) unconditionally calls `FocusEditor()` regardless of prior focus
- **Test:** none — the functional batch harness doesn't reproduce the terminal-focus timing (verified: test stayed green = bug absent there); `--exec` repro reliable, integration/PTY harness recommended

### BUG-058: Force keys fire and mutate background panel state while a modal overlay is open
- **Area:** Integrated terminal × overlays
- **Severity:** medium
- **Status:** confirmed (agent-reported, orchestrator re-verified — palette open AND terminal panel visible simultaneously)
- **Repro:** `key ctrl+p` (palette open), `key ctrl+t` → overlay still open, `bottom_panel.visible:true` — the terminal panel flips on behind the still-open palette
- **Expected:** force keys ignored (or handled consistently) while a modal overlay has focus
- **Actual:** `Root.HandleEvent` checks `ForceKeys` before `handleOverlay` (`internal/ui/root.go:98-115`); combined with BUG-057, dismissing the palette then sends typing to the editor
- **Test:** none — ledger-only (overlay+panel state; verified via --exec)

### BUG-059: Ctrl+K never reaches the PTY when the terminal is focused (chord matcher runs first)
- **Area:** Integrated terminal × keybindings
- **Severity:** high
- **Status:** confirmed (agent-reported, orchestrator re-verified at runtime + code ordering)
- **Repro:** focus terminal, type `foobar`, `ctrl+a`, `ctrl+k` (should kill-line in bash), enter → terminal runs `foobar` ("command not found") instead of an empty line; `^K` never reaches the shell. Ctrl+A (non-chord) passes through fine.
- **Expected:** per CLAUDE.md, all keys route to the PTY when the terminal is focused, except force keys
- **Actual:** `Root.HandleEvent` runs `handleChord(kev)` (`internal/ui/root.go:138`) BEFORE the RawKeyConsumer check (`:143-146`); since ~20 commands use `ctrl+k` as a chord prefix, the first Ctrl+K is always consumed by the chord matcher and never forwarded. Breaks readline Ctrl+K (kill-line) and any other chord-prefix key in the shell.
- **Test:** none — needs real PTY shell processing; ledger-only, integration/PTY harness

### Integrated terminal area notes
Robust: open/close/fullscreen toggle, basic key routing (echo reaches shell), rapid toggling, PTY resize (`stty size` matches the rendered rect), dead-PTY handling (`exit` closes the tab gracefully, focus falls back, no crash), multiple terminal tabs with independent scrollback, high-volume output (`seq 1 10000` keeps up), Ctrl+C (SIGINT delivered), ANSI parsing (no literal escapes), `clear`, and click-based focus routing (the counter-case isolating BUG-057 to dialog-based focus transitions). **The 3 findings are all focus/key-ROUTING gaps, not terminal-emulation bugs.** **DUMP GAPS:** `describeFocus()` returns `"other"` for both terminal AND editor focus (it checks `EditorPaneWidget` but `Root.Focused` is `EditorGroupWidget`; no terminal cases) — add `*ui.TerminalPanelWidget`→"terminal", `*ui.EditorGroupWidget`→"editor"; `describeOverlay()` returns `"unknown"` for the command palette (`SelectDialogWidget`); no per-cell color in screen/screenshot so terminal direct-color rendering is unverifiable via --exec.

### BUG-052: Plugin `p:clear()` with large dimensions freezes the editor (no bound check)
- **Area:** Plugin widgets
- **Severity:** high (a plugin can hang the whole editor)
- **Status:** confirmed (agent-reported; orchestrator re-verified at runtime AND in code — 12s timeout hit)
- **Repro:** a plugin whose render calls `p:clear(0, 0, 100000, 100000)`; activate its panel (Ctrl+B → click `»` overflow → click the plugin) → UI frozen (killed at 12s timeout, agent measured ~19.5s single render)
- **Expected:** `p:clear` clamps w/h to the panel/surface bounds (or is rejected)
- **Actual:** `ClearRect` on both `virtualSurface` and `subVirtualSurface` (`internal/widgets/virtual_surface.go:60-66, 126-132`) loops `h*w` times with no clamp — 1e10 iterations block the render/event loop. Part of the always-available raw-cell API.
- **Test:** `internal/widgets/audit_clearrect_bug_test.go` (`t.Skip`, goroutine+timeout — fails fast when unskipped)

### BUG-053: Plugin scrollview/markdown always draws a spurious full-width horizontal scrollbar
- **Area:** Plugin widgets
- **Severity:** medium
- **Status:** confirmed (agent-reported, orchestrator re-verified at runtime)
- **Repro:** a plugin with `p:scrollview({render=function(c) c:label("hi") end})` → a full-width `▄` scrollbar row renders under the short label
- **Expected:** no horizontal scrollbar when content fits
- **Actual:** `ScrollViewWidget.Render` (`internal/widgets/scrollview.go:102`) calls `Child.ScrollSize()` before `SetRect`; `VStackWidget.ScrollSize()` (`internal/widgets/vstack.go:73-80`) falls back to `w=80` when rect is zero, then SetRect fixes W=80, so `contentW(80) > viewW` stays true forever — a feedback loop. Affects every `p:scrollview` and `p:markdown` (also scrollview-wrapped)
- **Test:** none — ledger-only (visual; would need plugin-panel render assertions)

### BUG-054: Plugin dropdown popup overlaps/corrupts the status bar near the screen bottom
- **Area:** Plugin widgets
- **Severity:** medium
- **Status:** confirmed (agent-reported, orchestrator accepts — same overlay-clamp family as BUG-039/040)
- **Repro:** a plugin dropdown opened near the bottom → popup's bottom border draws on the status-bar row, left border mangles the panel border
- **Expected:** popup flips up / clamps above the status bar
- **Actual:** `ContextMenuWidget.Render` (`internal/ui/contextmenu_widget.go:98-116`) clamps against full surface height `sh` (includes the status row); `if y+menuH > sh` never fires when the bottom lands exactly on `sh-1`
- **Test:** none — ledger-only (overlay rects not in dump)

### BUG-055: Disabled-after-errors plugin keeps rendering and erroring forever (Enabled never checked at render)
- **Area:** Plugin widgets
- **Severity:** medium
- **Status:** confirmed (agent-reported; orchestrator re-verified at runtime — 1499 log lines — AND in code)
- **Repro:** a plugin that `error()`s in render; activate it → output log floods with "render: boom" AND "disabled after N errors" for N well past the threshold of 10
- **Expected:** after `maxPluginErrors` (10) the plugin stops being invoked
- **Actual:** `plugin.go:192` sets `p.Enabled=false`, but `PluginPanelWidget.Render` (`internal/plugin/panel_widget.go:40`) calls `CallRenderWith` unconditionally — `Enabled` is only read for list display, never at the render call site
- **Test:** none — ledger-only (needs plugin-panel activation; runtime repro in this entry)

### BUG-056: Negative box-model margins silently drop a plugin widget (no clamp/validation)
- **Area:** Plugin widgets
- **Severity:** low
- **Status:** confirmed (agent-reported, orchestrator re-verified at runtime — label absent)
- **Repro:** `p:label({text="x", margin_top=-5, margin_left=-5})` → the label never appears (before/after abut)
- **Expected:** clamp negative margins to 0 (or signal invalid); the content shouldn't vanish
- **Actual:** `BoxOverheadH()` (`internal/widgets/surface.go:120-129`) sums margins unclamped → `Height()` = `1 + (-5)` = -4; `VStackWidget.Render` drops children with `ch <= 0`
- **Test:** none — ledger-only

### Plugin widgets area notes
Robust where it counts: Lua syntax errors, errors thrown inside callbacks (vs render), malformed descriptors (wrong types, missing fields, nil), and wrong table column counts all degrade gracefully without crashing or dropping sibling widgets. Table `on_select`/`on_command` indices are correctly 1-based (no off-by-one); unicode/emoji input, prefix, clear_on_submit, deep nesting (10 levels), and focus routing in/out of plugin panels all work. `ttt.storage` does NOT exist (only `ttt.settings`) — calling it fails to register the plugin, contained safely. **The findings cluster in raw-cell bounds (BUG-052 freeze), scroll/overlay layout (053/054), the error-disable safety net (055), and box-model validation (056).** **Harness note:** activating a plugin's custom sidebar panel via `--exec` = Ctrl+B (show sidebar) → click the `»` overflow tab → click the plugin in the popup menu. **DUMP GAPS:** plugin panels render as an opaque `PluginPanel` leaf in `widget_tree` (no child widget rects); `focus` is `"other"` for plugin focus; `overlay.type` `"unknown"` for plugin dropdowns.

### BUG-047: Global-search navigation ignores the match column — cursor always lands at col 0
- **Area:** Global search
- **Severity:** high
- **Status:** confirmed (agent-reported, orchestrator re-verified — cursor col 0, real match col 8)
- **Repro:** search `needle`, activate the "another needle line" result → cursor at line 3 col 0 (should be col 8)
- **Expected:** cursor lands at the match's exact column
- **Actual:** `NavigateToSearchMatch` (`internal/app/callbacks.go:~131`) receives `col` but never uses it — `GoToLine` unconditionally sets `Cursor.Col=0` (`internal/ui/editor_group.go:885`) and col is never restored
- **Test:** `tests/functional/audit-global-search-bugs.test.js` (`it.fails`, marker-based)

### BUG-048: Editor search state shared across tabs — Find Next after a direct tab switch applies another file's match coords
- **Area:** Global search × tabs
- **Severity:** high
- **Status:** confirmed (agent-reported, orchestrator re-verified — F3 placed cursor at col 8, a column valid only in the other file, on a line that doesn't exist in the active buffer)
- **Repro:** search + navigate to a match in file B, dirty it, navigate to a match in a new tab, `alt+,` back to the first tab, `f3` → cursor jumps to stale cross-file coordinates
- **Expected:** Find Next after a tab switch no-ops or finds a match IN the active file
- **Actual:** `Editor.SearchMatches`/`SearchActive` live on the single Editor widget and are recomputed/cleared only by `NavigateToSearchMatch` and the sidebar-panel-change callback (`internal/app/callbacks.go:451`) — NOT by `SwitchTab` (`internal/ui/editor_group.go:516`). Direct tab switches (tab bar, `alt+.`/`alt+,`) leave stale coords that `FindNext` applies with no bounds/identity check. Same architectural class as BUG-013.
- **Test:** none — fragile multi-tab/dirty-tab sequence; exact `--exec` repro in this entry, shares root with BUG-013

### BUG-049: Per-file result cap (rg --max-count=100) silently truncates with no UI indication
- **Area:** Global search
- **Severity:** medium
- **Status:** confirmed (agent-reported, orchestrator re-verified — `many.txt (100)`, "104 results in 3 files", real count 2000)
- **Repro:** a file with 2000 matching lines → panel shows 100, no "showing 100 of 2000+" anywhere
- **Expected:** all matches reachable, or a visible truncation notice
- **Actual:** `--max-count=100` hard cap in `internal/ui/search_widget.go:314` with no truncation signal in list/summary/scroll
- **Test:** none — the fix defines the truncation-indicator text; ledger-only

### BUG-050: Multi-root workspaces with same-named files show indistinguishable unlabeled result groups
- **Area:** Global search
- **Severity:** medium
- **Status:** confirmed (agent-reported, orchestrator re-verified — two identical `dup.txt (1)` headers)
- **Repro:** two roots each with `dup.txt` containing the term → two `dup.txt (1)` group headers, no folder qualifier
- **Expected:** folder-prefixed/qualified path when roots share a relative path
- **Actual:** `SearchWidget.Render` (`internal/ui/search_widget.go:639`) prints `g.RelPath` relative to whichever workdir matches first; navigation still opens the correct absolute file (display-only bug)
- **Test:** none — fix defines the label format; ledger-only

### Global search area notes
Basic search, result counts, case/regex behavior, empty/whitespace/CJK queries, and the debounce+generation race (no stale flicker observed) came back clean; search-and-replace-in-files spot-checked OK (adjacent, not fully swept). Findings cluster in navigation (col ignored, cross-tab stale state) and result-display (truncation, multi-root labels). **DUMP GAP:** `Editor.SearchMatches`/`SearchActive` not in the dump — highlight rendering verified indirectly via Find-Next cursor placement.

### BUG-043: `--workspace <file>` silently falls back to cwd on load failure (no feedback)
- **Area:** Workspace
- **Severity:** medium
- **Status:** confirmed (agent-reported, orchestrator re-verified — `output:null`, opens cwd)
- **Repro:** `bin/ttt --workspace /tmp/nonexistent.ttt` (or a syntax-broken `.ttt`) → opens cwd as a single-folder workspace, no error anywhere
- **Expected:** a status error like the interactive "Open Workspace" command shows (`Error: unexpected end of JSON input`)
- **Actual:** `resolveArgs()` (`internal/app/widgets.go:75-84`) ignores `LoadFile`'s error and falls through to cwd; the CLI and interactive paths handle the identical failure inconsistently
- **Test:** none — the fix defines the error-feedback signal; ledger-only (mirrors BUG-042)

### BUG-044: Git branch/gutter missing when the opened file is below the repo root (resolved)
- **Area:** Workspace × git
- **Severity:** medium
- **Status:** resolved for active files whose file identity or nearest existing parent can be stably resolved, including linked worktrees
- **Repro:** open a file in a repo SUBDIR by absolute path → no branch in the status bar. Control that nails it: opening ttt's own `internal/ui/root.go` by absolute path shows NO branch, while opening the repo at its root (cwd) shows `audit/bug-hunt` — same repo, different result.
- **Expected:** branch/gutter work for any file inside a git working tree (the Changes panel already does — it uses `git rev-parse --show-toplevel`)
- **Actual:** repository membership and branch now come from the repository observer's cached canonical Git identity instead of `workspace.isGitRepo()`; active-file discovery resolves existing file and directory symlinks before choosing the Git directory, uses the nearest stably resolvable existing parent for missing files, and declines broken links or cycles.
- **Test:** `tests/functional/audit-workspace-bugs.test.js` (nested-repository, linked-worktree, and external file-symlink fixtures)

### BUG-045: `.ttt` workspace files accept a folder entry pointing at a regular file (no IsDir validation)
- **Area:** Workspace
- **Severity:** low
- **Status:** confirmed (agent-reported, orchestrator re-verified — `▼ a.txt` shows as a bogus expandable folder)
- **Repro:** `.ttt` with `{"folders":[{"path":"a.txt"}]}` where a.txt is a file → explorer renders it as an expandable node that toggles with no children
- **Expected:** reject/skip with a warning, or treat as a file — consistent with Add/Open Folder and CLI args, which validate `IsDir()`
- **Actual:** `workspace.LoadFile`/`AddFolder` do no `IsDir()` check
- **Test:** none — low severity; ledger-only

### BUG-046: Removing a workspace folder leaves its open tabs orphaned (folder-scoped features silently disabled)
- **Area:** Workspace
- **Severity:** low (may be intentional — some editors keep files open after folder removal)
- **Status:** confirmed (agent-reported, orchestrator re-verified — tab persists after its folder is removed)
- **Repro:** open a file from dirB, `Remove Folder` dirB → explorer drops dirB's tree but the b.txt tab stays open, and `FolderForFile` now returns nil for it (disabling git/gutter/LSP-root for that tab)
- **Expected:** close the tab, or mark it as no longer workspace-scoped
- **Actual:** `refreshWorkspaceWidgets()` (`internal/app/app.go`) updates Explorer/Search/Changes but never touches EditorGroup's open tabs
- **Test:** none — low/ambiguous; ledger-only

### Workspace area notes
Verified correct: multi-folder open with per-root labels; `FolderForFile` longest-prefix disambiguation (`proj` vs `proj-extra`); `.ttt` save/load roundtrip (folder order, paths, relative-path resolution from a different cwd); duplicate-folder dedup; Add/Open Folder + no-args cwd fallback; a `.ttt` entry pointing at a deleted directory loads without crashing. **Adjacent follow-up:** the explorer lists `.git` as a normal expandable directory (file-listing filter gap — belongs to an explorer/ignore sweep).

### Settings & options area notes (swept clean)
Haiku mechanical sweep (26 cases) + orchestrator spot-check found no bugs. Verified: every Options-menu toggle (line numbers, word wrap, auto-dedent, bracket colorization, git gutter, LSP, syntax highlight — the last correctly shows "Restart to apply") takes effect and persists to settings.json; gutter/border-style pickers persist; malformed/empty/missing settings.json all fall back to defaults with no crash. **Keybindings robustness spot-checked by orchestrator** (highest risk per the past real-config-wipe incident): malformed keybindings.json → defaults still work; empty `{}` → defaults RETAINED (additive-override, not replace); valid custom rebind takes effect; binding to a nonexistent command ignored gracefully. Residual gaps (low risk, not chased): indentation-picker persistence, mid-session live-reload (covered by the existing `reload_settings` e2e test), and "Open Settings" opening settings.json in a tab.

### BUG-041: Theme picker cancel reverts colors but leaves the border charset stuck on the preview
- **Area:** Themes & rendering
- **Severity:** medium
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** fresh config (default rounded borders); `exec "Switch Theme"`, type `turbo` (preview turbo-vision's double-line borders), `key escape` → border glyphs stay `╔═` instead of reverting to `╭─`
- **Expected:** dismissing the picker after only previewing reverts everything — colors AND border glyphs — to the pre-picker theme
- **Actual:** `ShowThemePicker` `OnDismiss` (`internal/app/commands_palette.go`) restores the style map and palette but never resets `*a.Borders` (which the preview's `applyTheme` set via `BuildBorderSet`), so preview borders persist until another theme is applied or restart
- **Test:** `tests/functional/audit-theme-bugs.test.js` (`it.fails`)

### BUG-042: Malformed theme JSON fails completely silently (no crash, no feedback)
- **Area:** Themes & rendering
- **Severity:** medium
- **Status:** confirmed (agent-reported, orchestrator re-verified — byte-identical screen, `output:null`)
- **Repro:** put a syntax-broken `themes/broken.json` in the config dir; either set `"theme":"broken"` in settings and launch, or `exec "Switch Theme"` + select it → nothing applies, no status/error, no crash
- **Expected:** a visible status-bar error (consistent with other config error paths), or clear indication the theme didn't load
- **Actual:** startup `config.Load()` discards the `json.Unmarshal` error entirely; runtime `ShowThemePicker` `OnSelect`/`OnChange` call `config.LoadTheme` and on error just `return` with no status message (and OnSelect doesn't persist the setting either) — the failure is invisible
- **Test:** none — the fix defines the error-feedback signal (status text); ledger-only until then

### Themes area notes
Cycled all 18 built-in themes (`internal/config/themes/*.json`) — no crash, no stale characters, clean redraw each. Missing-StyleDef-field themes inherit `DefaultTheme()` defaults gracefully (not a gap). Long-file scroll returns to byte-identical state; gutter alignment across 999→1000 correct and shrinks back; selection extent across tabs/emoji correct; sidebar/bottom-panel border T-junctions clean. No whitespace-render toggle exists. **Adjacent-area follow-up (not ledgered):** "Git: Open Compact/Extended Diff" silently no-op unless a Changes-panel row was keyboard-selected first — belongs to a Changes-panel/git sweep (and the known synthetic-click blind spot); revisit there. **DUMP GAP:** no active-style-map/border-set field in the dump — pure color-only regressions remain unconfirmable (only character-level defects like BUG-041's border glyphs are visible).

### BUG-036: Status bar text invisible at width <= 50 (editor box border overwrites it)
- **Area:** Resize & layout
- **Severity:** medium (50 cols is a realistic split-pane width)
- **Status:** confirmed (agent-reported, orchestrator re-verified — present at 51/60/70/80, gone at <=50)
- **Repro:** `bin/ttt --size 50x20 --exec 'wait 200; screenshot /tmp/s.txt; quit' file.txt` → last row is the editor box border `╰──╯`, no status text; at 51 the status bar shows "Ln 1, Col 1 ...". StatusBar rect is valid (`y:19 w:50 h:1`) but nothing renders there — a debug/screenshot disagreement.
- **Expected:** status bar renders or truncates gracefully at <=50 cols
- **Test:** `tests/functional/audit-resize-bugs.test.js` (`it.fails`)

### BUG-037: Sidebar crushes the editor pane to 1 column at small terminal sizes
- **Area:** Resize & layout
- **Severity:** low (only at very small sizes: <=30 cols)
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** `--size 30x15`, toggle sidebar → `Sidebar {w:26}` next to `EditorGroup {w:1}` (no line numbers/text). Same at 20x10; clamps at w:1, never negative, no crash.
- **Expected:** sidebar yields width (auto-collapse or shrink) so the editor keeps a usable minimum
- **Test:** none — small-size degradation; rect-invariant repro in the debug dump

### BUG-038: Bottom panel overlaps and fully hides the editor pane at small sizes (overlapping rects)
- **Area:** Resize & layout
- **Severity:** medium (genuine overlapping-rect invariant violation)
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** `--size 30x15`, toggle bottom panel → `EditorGroup {y:1,h:12}` (rows 1-12) and `BottomPanel {y:2,h:11}` (rows 2-12) overlap; screenshot shows only the panel, zero editor content. Same at 20x10. Non-overlapping at 40x20/80x24 (though editor crushed to h:1 at 40x20).
- **Expected:** editor and bottom panel split height without overlapping rects
- **Test:** none — small-size; overlapping-rect repro in the debug dump (candidate for an e2e rect-overlap assertion)

### BUG-039: Discard button vanishes from the unsaved-changes dialog at narrow widths (<=26 cols)
- **Area:** Resize & layout
- **Severity:** medium (silently removes a user action)
- **Status:** confirmed (agent-reported, orchestrator re-verified — present at 40, gone at <=26)
- **Repro:** `--size 26x15`, edit + `ctrl+w` → dialog shows only "Cancel"/"Save"; "Discard" label shrinks ("Di" at 28) then disappears entirely at <=26, while "Save" is never truncated
- **Expected:** all three actions stay visible/reachable (grow/wrap/truncate gracefully)
- **Test:** `tests/functional/audit-resize-bugs.test.js` (`it.fails`)

### BUG-040: Menu dropdown clips off-screen with no scroll affordance at tiny sizes
- **Area:** Resize & layout
- **Severity:** low (only at ~20x10)
- **Status:** confirmed (agent-reported, orchestrator re-verified — "Quit" not visible at 20x10)
- **Repro:** `--size 20x10`, `click 2 0` (File menu) → dropdown clipped horizontally (no right border/shortcuts) and vertically (Save Workspace/Review PR/Quit absent), no closing border, no scroll indicator
- **Expected:** dropdown scrolls/shrinks/repositions to fit; hidden items discoverable
- **Test:** none — tiny-size; screenshot-only repro

### Resize area notes
Findings cluster at small terminal sizes; the standout is BUG-036 (status bar at 50 cols — a realistic split width). No crashes/panics at any size down to 10x5 (rects clamp at w:1/h:1, never negative). **DUMP GAP:** `overlay.type` is always `"unknown"` and overlays (palette/dialog/menu) are NOT in `widget_tree` — no rect data for overlays, so BUG-039/040 are screenshot-only. Adding overlay rects to the dump would make dialog/menu layout testable non-visually. Adjacent note: tab-bar decorative border truncates mid-glyph at ~20 cols even with no overlay (likely normal chrome degradation; a tab-bar-rendering sweep could double-check).

### BUG-028: Selection-dependent explorer commands are exposed in the command palette (no valid target there)
- **Area:** Explorer / command palette
- **Severity:** low  *(was high — see Curation)*
- **Curation (2026-07-12, DOWNGRADED + REFRAMED):** original framing ("no selection", "silent data loss") was inaccurate — the tree's default selection IS the root (shown selected), and `FileOpDelete` shows a confirm dialog naming the folder. The real, general issue: `Explorer: Delete/Rename` (and other `explorer.*` context commands) operate on the explorer's selected node, but they are listed in the command palette (`commands_palette.go:14` = `a.Reg.List()`, unfiltered) where there is no meaningful selection — so from the palette they fall back to `Tree.Selected()` = root. **There are NO global keybindings for any `explorer.*` command** (confirmed: `keybindings.go` has none), so the palette is the ONLY context-free entry point; the right-click menu already guards the root (offers Refresh / Copy Path / Remove from Workspace via `isRoot()`, `callbacks.go:545`). Hiding these from the palette fully removes the bad path.
- **Status:** confirmed behavior; reframed as a palette-exposure design gap, not data loss
- **Fix (shared with the explorer-command-exposure cluster):** add a `Hidden bool` / `ShowInPalette` field to `command.Command`; filter it at the palette call site only (keep `Reg.List()` intact for the keybindings view / `exec` / tests). Mark selection-dependent commands hidden: `explorer.delete/rename/newFile/newFolder/open/removeRoot/copyAbsolutePath/copyRelativePath`. Keep `explorer.refresh/help` visible. This alone closes BUG-028 (no keybinding path remains); the residual bugs in BUG-029/030 are separate and still need their own fixes.
- **Repro:** `exec "Explorer: Delete"` with root selected, confirm → `os.RemoveAll` root (palette-only path)
- **Test:** `tests/functional/audit-explorer-bugs.test.js` (`it.fails`, delete case) — kept; note `exec`/`FindByTitle` bypasses the palette filter so the test still drives the command deliberately (real users lose only the palette route)

### BUG-029: Renaming an open file leaves the tab tracking the old path (misdirected save)
- **Area:** Explorer
- **Severity:** medium  *(was high — see Curation)*
- **Curation (2026-07-12, CONFIRMED, downgraded high→medium):** genuine bug (not mis-framed like BUG-028) — `FileOpRename` (`fileops.go:54`) does `os.Rename` + explorer `reload()` and never updates open editor tabs (no `RenameTab`/`UpdateTabPath` mechanism exists). Triggers via the legitimate right-click Rename on an open file, so it's independent of the palette-exposure issue. Downgraded to medium: needs the rename-open-file-then-save sequence and the misdirected save is confusing-but-detectable (tab shows old name, tree shows new); the folder-rename-save-fails variant is nastier but rarer. **Shares one root cause and fix with [[BUG-031]]: reconcile the open-tab model when a file is renamed/deleted** — fix them together.
- **Status:** confirmed (agent-reported, orchestrator re-verified with disk + tab-path dump)
- **Repro:** open root.txt, `Explorer: Rename` → renamed.txt, edit, save → tab still `path: root.txt`, so save recreates root.txt with the edit while renamed.txt keeps stale content. Folder-rename variant makes save fail outright ("no such file or directory"), stranding the edit.
- **Expected:** rename updates the open tab's path (or warns/blocks save)
- **Actual:** tab path never updated after rename
- **Test:** `tests/functional/audit-explorer-bugs.test.js` (`it.fails`)

### BUG-030: New File / Rename silently clobbers an existing file (silent delete) — ✅ FIXED
- **Area:** Explorer
- **Severity:** medium  *(was high — see Curation)*
- **Curation (2026-07-12, CONFIRMED → FIXED, downgraded high→medium):** genuine bug — worse than BUG-028 in that there is NO confirmation naming the destruction. `FileOpNewFile` wrote with `os.WriteFile` (O_TRUNC) with no stat check, so a colliding "New File" name silently emptied the existing file; `FileOpRename` `os.Rename` silently replaced an existing target. Triggered via the legitimate in-context New File/Rename, so independent of BUG-028's palette exposure. Downgraded to medium: needs a name collision (a less-common action), though the outcome was silent irreversible content loss.
- **Status:** ✅ **FIXED on `review` branch** — `os.Stat` guard added to both `FileOpNewFile` and `FileOpRename` (`internal/app/fileops.go`): existing target → `a.StatusError("<name> already exists")` and return, no write. Rename-to-self (`newPath == path`) is a no-op; `os.SameFile` allows case-only renames on case-insensitive filesystems while blocking genuine collisions. Orchestrator re-verified: `make build` clean, both repro tests pass as real `it`.
- **Repro (now fixed):** `Explorer: New File` named `dup.txt` when dup.txt exists → status error, dup.txt untouched. Rename onto an existing name → status error, both files intact.
- **Test:** `tests/functional/audit-explorer-bugs.test.js` — flipped `it.fails`→`it`, 2 real passing cases (New File + Rename collision).

### BUG-031: No notification when an open file is deleted on disk (the warning path is dead code)
- **Area:** Explorer × file watching
- **Severity:** medium
- **Curation (2026-07-12, CONFIRMED, kept + reframed):** the defect is the **missing signal**, not the save behavior. Saving to recreate a deleted file is CORRECT and should stay (don't lose the user's buffer — matches VS Code); do NOT make save error on plain delete. The real bug: the user gets zero indication the file is gone — buffer stays `modified:false`, tab looks normal, and the intended "was deleted on disk" warning is unreachable dead code (confirmed: `Buffer.DiskChanged` at `io.go:76-79` returns false when `os.Stat` fails, so `HandleFileChanged` bails at the `DiskChanged` check before reaching the delete-warning branch). Fix = notification + mark the buffer diverged (dirty), leaving save-to-recreate intact. The separate parent-dir-gone case (save actually FAILS → stranded edit) belongs with [[BUG-029]] — warn + offer Save As there.
- **Status:** confirmed (agent-reported, orchestrator re-verified via timed external rm)
- **Repro:** open a file, `rm` it externally (or delete via Explorer), wait → tab shows old content, `modified:false`, no status message. Edit+save silently resurrects the file.
- **Expected:** warn ("<file> was deleted on disk") and/or close/mark the tab
- **Actual:** `HandleFileChanged` (`internal/app/watch.go`) returns early on `buf.DiskChanged(path)==false`, but `Buffer.DiskChanged` (`internal/core/buffer/io.go`) returns false when the file is missing — so the "was deleted on disk" branch is unreachable
- **Test:** none — batch functional harness can't rm mid-session; ledger-only (integration/PTY test is the right home)

### BUG-032: Opening a file from the Explorer does not focus the editor — ❌ REJECTED (intentional)
- **Area:** Explorer
- **Severity:** ~~medium~~ — n/a (not a bug)
- **Curation (2026-07-12, REJECTED — intended behavior):** this is a deliberate, configurable default, not a defect. `FocusOnOpen` (settings.go:78) governs whether opening a file from the explorer moves focus to the editor; it defaults false (keep focus in the explorer for a browse-without-losing-focus workflow). Factual correction to the original finding: there is NO single-vs-double-click distinction — the tree has no double-click handling (`tree.go:541-573`: one Button1 click → `ActivateSelected` when `!SelectOnClick`, which the explorer leaves false), so a single click AND Enter take the identical path (`OnOpenFile` → `FocusEditorIfEnabled`, a no-op when `FocusOnOpen:false`). So both open without focusing — one global setting, intentionally defaulted. Optional future enhancement (NOT a bug): VS-Code-style Enter/double-click-focuses vs single-click-preview would need a click-vs-Enter distinction the tree doesn't have.
- **Status:** REJECTED — behavior is intentional
- **Test:** REMOVED (the `it.fails` case asserted the opposite of the intended default)

### BUG-033: CJK filenames break tree column alignment (rune-count vs display-width)
- **Area:** Explorer / rendering
- **Severity:** low  *(cosmetic divider misalignment; was medium)*
- **Curation (2026-07-12, CONFIRMED, downgraded medium→low):** real but cosmetic. **Fix requires a NEW display-width helper — there is none in the codebase.** Checked: `editor.byte_to_col`/`col_to_byte` (the Lua-exposed converters) count UTF-8 lead bytes = RUNE count, not display width (a CJK char → 1), so they reproduce the bug rather than fix it; `bufColToVisualCol` only expands tabs (CJK → width 1); no `runewidth` usage anywhere. The editor is rune-width throughout, not cell-width — SAME root cause as [[BUG-009]] (ZWJ). Fix = wrap `uniseg.StringWidth` (already an indirect dep — `go mod tidy` promotes it) into a `DisplayWidth(string) int` helper and use it in `tree.go` (~298, 556) instead of `len([]rune())`; that helper is the reusable foundation for BUG-009 and could back a real Lua `str_width`. **Recommend a small foundational task: add `DisplayWidth`, then BUG-033 + BUG-009 both consume it.**
- **Status:** confirmed (agent-reported; orchestrator confirmed the code path — the internal screenshot grid masks it because it uses the same 1-cell-per-rune model, but a real terminal renders wide glyphs at 2 cols → 3-col overflow for a 3-glyph name)
- **Repro:** a workspace with `日本語.txt` and `root.txt`; the sidebar/editor divider shifts right by the CJK glyphs' extra display width on that row
- **Expected:** the divider sits at the same display column on every row
- **Actual:** `internal/widgets/tree.go` (~lines 298, 556) sizes rows via `len([]rune(...))`, undercounting each wide glyph by one cell
- **Test:** none — misalignment is invisible in the char-grid screenshot; would need a display-width-aware render assertion (noted for a rendering-specific harness)

### BUG-034: New File honors `/` in the name, creating nested subdirectories — ❌ REJECTED (intended)
- **Area:** Explorer
- **Severity:** ~~low~~ — n/a (not a bug)
- **Curation (2026-07-12, REJECTED — intended feature):** this is exactly VS Code's New File behavior — typing `sub/dir/deep.txt` creates the intermediate folders (`os.MkdirAll(filepath.Dir(newPath))`) and the file. It's an established convenience, no data loss, no confusing outcome. Not a defect.
- **Status:** REJECTED — intended feature (VS Code parity)
- **Test:** none (was ledger-only)

### BUG-035: Quick Open does not sync the Explorer keyboard-selection to the opened file — ❌ REJECTED (mostly works)
- **Area:** Explorer
- **Severity:** ~~low~~ — n/a (agent over-reported)
- **Curation (2026-07-12, REJECTED — reveal actually works):** the agent under-observed because screenshots carry no color info. The active file IS revealed/highlighted on Quick Open: `SetActiveFile` fires in the event-loop sync (`eventloop.go:60`) → `Tree.SetActiveID`, and the active node renders with `StyleSidebarSelected` (`tree.go:315`). The only residual is that the keyboard-nav `selected` INDEX isn't synced to the active file (so arrowing after focusing the tree doesn't continue from it, and the `selected` highlight overrides the active-file one when the tree is focused). Trivial nit, arguably by-design. **Optional enhancement (not a bug):** on reveal, also set `selected` to the active node for exact VS Code parity.
- **Status:** REJECTED — reveal works; selection-index sync is a trivial optional enhancement
- **Test:** none (was ledger-only)

### Explorer area notes (clean probes)
Nested-dir navigation, create-in-selected-dir, empty-dir handling, keyboard expand/collapse, arrow nav through a 50-file dir, tree re-sort after create, collapse-state preservation across ops, and CJK/space filenames opening correctly all worked. Data-loss findings clustered in the rename/delete/collision paths and the file-watch teardown.

### BUG-026: Fold collapsed-state reattaches to an unrelated block after line-count edits
- **Area:** Folding × editing
- **Severity:** high
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** fold the `if true {` block (line 4), insert a blank line at the top of the file → the fold marker now collapses `func outer()`'s entire body instead
- **Expected:** collapsed state follows the folded content (or at worst clears); a region the user never folded must never become collapsed
- **Actual:** `fold.State.SetRanges` (`internal/core/fold/fold.go:26-39`) recomputes ranges on every line-count change and preserves collapse purely by raw `StartLine` equality — after the shift, the outer function's new start line coincides with the old collapsed key and inherits the fold, silently hiding different code. Duplicate Line on a folded header makes the fold vanish by the same mechanism.
- **Test:** `tests/functional/audit-fold-bugs.test.js` (`it.fails`)

### BUG-027: Move Line on a folded header swaps the header with a HIDDEN line — silent code reordering
- **Area:** Folding × editing
- **Severity:** high
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** fold `if true {`, press `alt+down` → buffer becomes `func outer() { / \t\tfoo() / \tif true {` — `foo()` hoisted out of its block — while the fold marker still renders as if valid
- **Expected:** move the whole folded region as a unit (VS Code) or no-op while folded; never reorder invisible code
- **Actual:** `MoveLineDown`/`Up` issue a raw `SwapLineCommand` with no fold awareness; since line COUNT is unchanged, the `exec()` fold-recompute guard (`internal/ui/editor_widget.go:214-217`) never fires, so the stale marker keeps rendering over now-invalid structure
- **Test:** `tests/functional/audit-fold-bugs.test.js` (`it.fails`)

### Folding area notes (clean probes)
Delete Line on folded header (hidden lines revealed, no data loss), copy/paste of header, selection deletion across folds, Join Lines, arrow-skip over collapsed regions, go-to-line auto-expand, nested fold preservation, collapse/expand-all, save-with-folds — all correct. Syntax-highlight layering on collapsed headers not independently verified (no style info in dump).

### BUG-018: Clicking a second menu header closes the open menu instead of switching to it
- **Area:** Mouse / menu bar
- **Severity:** medium
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** `click 4 0` (File opens), `click 30 0` (View header) → `.overlay` becomes null, no menu open; a further click is needed to open View
- **Expected:** standard menu-bar UX — clicking another header while a menu is open switches directly to that menu
- **Actual:** the second click only dismisses the open menu
- **Test:** `tests/functional/audit-mouse-bugs.test.js` (`it.fails`)

### BUG-019: Rightmost column of explorer tree rows is click-dead
- **Area:** Mouse / sidebar explorer
- **Severity:** low
- **Status:** confirmed (agent-reported, orchestrator re-verified)
- **Repro:** explorer tree rect `{x:1,w:30}`; `click 30 6` on a file row does nothing; `click 29 6` opens the file
- **Expected:** the full row rect (x=1..30) is clickable
- **Actual:** the last column is dead — off-by-one in the tree's hit test
- **Test:** `tests/functional/audit-mouse-bugs.test.js` (`it.fails`)

### Mouse-area notes
- Core editor click mapping is clean: scrolled viewports (vertical + horizontal), tabs, CJK, clamping past line end/below last line, gutter, double-click word select (incl. CJK/punctuation), triple-click line select — no offset divergences found.
- The reported "stale wrap-map click after Toggle Word Wrap" is a **harness artifact, not a product bug**: synchronous `exec "..."` doesn't trigger a render pass, so an immediately following `click` resolves against the pre-toggle wrap map. Through the real key-dispatch path (palette + Enter) the same click resolves correctly. Same root as the stale-status-bar gap below.
- Additional dump gaps: `.overlay` reports `{"type":"unknown"}` for menus (can't distinguish overlay kinds); top-level `.focus` reports `"other"` for most real focus states (also seen in the first sweep) — per-widget `focused` flags in the widget tree are the workaround. `--exec` has no `drag` command, so drag-selection remains unprobed.

### Tabs area notes (clean probes)
Per-tab cursor/selection/scroll/multicursor/fold restoration, undo isolation, dirty-flag lifecycle, close-with-unsaved-changes dialog, duplicate-open reuse, new-file/save-as, overflow hit-testing on tab labels, wrap-around switching, and widget-tree leak checks all passed. **Editor split panes do not exist** in the codebase (`SplitPanelWidget`/`ContentSplitWidget` are the sidebar/bottom layout splits) — that sub-area is N/A until the feature exists.

### Harness gap from the find/replace sweep
`debug` JSON has no `search` section (query, options, match list, active index, bar focus) — match staleness had to be inferred via cursor movement. Screenshot carries no style info, so highlight-artifact checks are only indirect. Consider a `search` block in the dump.

### Harness gap (not a product bug): `--exec key shift+tab` cannot produce `KeyBacktab`
`comboToTcell("shift+tab")` yields `(KeyTab, ModShift)`; there is no `backtab` keyword in the key parser, so the `KeyBacktab` code path is unreachable from `--exec`/functional tests. Real terminals send Backtab as CSI Z. Consider adding a `backtab` keyword when convenient — until then, Backtab behavior is only testable via e2e event injection.

### Harness gaps from the multicursor sweep
- ~~`debug` JSON lacked buffer text and multicursor state~~ — **fixed on this branch** (`buffer.text` capped at 1000 lines, `multi_cursor[]` with selection anchors).
- `--exec click/hover` always post `ModNone` — Alt+Click (mouse add-cursor) cannot be exercised; that feature remains untested.
- `exec "Command Name"` runs synchronously and bypasses the event loop's `syncStatus()`, so a screenshot taken immediately after shows a stale status bar (cursor pos, cursor count, dirty flag) until the next real key/click event. Affects harness observations only, not real palette usage.
- "Add cursor above/below" does not exist as commands — cursor creation is `ctrl+d` / `ctrl+k l` / Alt+Click only. `editor.splitSelectionToLines` exists but was not deeply probed (budget).

<!-- Template:
### BUG-NNN: <one-line summary>
- **Area:**
- **Severity:** low / medium / high
- **Status:** confirmed / unconfirmed
- **Repro:** `bin/ttt --size 120x40 --exec "..."`
- **Expected:**
- **Actual:**
- **Test:** <path or "none — reason">
-->

---

## Resume guide

Everything a fresh session needs to continue this audit without re-deriving it. As of the last commit: **35 confirmed findings (BUG-001..035), ~31 repro tests**, branch `audit/bug-hunt`.

### The orchestration loop (what the orchestrator does per area)

1. Mark the area `in progress` in the coverage matrix; commit.
2. Spawn **exactly one** hunting agent (background) with the area prompt below. Sonnet for judgment-heavy areas, Haiku for mechanical sweeps. Agents are **read-only on the repo** and report structured findings per `audit/agent-brief.md`.
3. When it returns, **re-verify every repro yourself** by re-running the exact `bin/ttt --exec` command. Do not trust an agent finding until you reproduce it. A "clean" report from a mechanical sweep gets a **skeptical spot-check of its single hardest case** before the area is marked clean (this is how BUG-009 was caught after a clean report).
4. For each confirmed finding: assign the next BUG-NNN, write a repro **test that asserts the CORRECT behavior**, marked expected-failure — `it.fails(...)` (vitest) or `t.Skip("BUG-NNN")` (Go e2e). It passes now, goes red when the bug is fixed. If untestable via the batch harness, record it ledger-only and say why.
5. Add a ledger entry (use the template in the Findings section), update the matrix, **commit per finding** (message `audit: BUG-NNN <summary>`). Commit from the repo root — `git -C /home/enko/Documents/ttt ...` if cwd drifted.
6. **Triage harness artifacts vs real bugs.** Some agent findings are `--exec` limitations, not product bugs (e.g. synchronous `exec "..."` skips a render pass → stale-looking clicks/status bar). Confirm through the real key-dispatch path before ledgering; record confirmed non-bugs in area notes so they aren't "fixed" later.
7. If an agent stalls or comes back empty, stop it and re-brief with a different angle/model.

Test verification trick: to confirm an `it.fails` test really captures the bug, temporarily flip it to `it(...)` (or comment the `t.Skip`) and confirm it FAILS with the bug present; then restore the marker. Never leave the suite red.

### Harness state (already built on this branch — don't rebuild)

Debug dump (`--exec "debug PATH"`, `internal/app/debug_dump.go`) now includes:
- `buffer.text` (line contents, capped 1000 lines) + `text_truncated`
- `multi_cursor[]` (per-cursor line/col/primary/sel_from) — only when multicursor is active
- `viewport` (top_line/left_col/width/height)
- plus the originals: `cursor`, `selection`, `tabs[]`, `active_tab`, `focus`, `sidebar`, `bottom_panel`, `overlay`, `diagnostics`, `output`, `widget_tree` (per-node rect/focused/props).

### Standing dump/harness gaps (known — work around, or extend the dump if an area needs it)

- No `search` block (query/options/matches/active index/bar-focus) — find/replace staleness had to be inferred from cursor moves.
- No `folds` block (collapsed ranges) — fold state only visible via screenshot `▶`/`⋯` markers.
- `overlay.type` reports `"unknown"` for menus/dialogs (can't distinguish kinds from JSON).
- Top-level `focus` reports `"other"` for most real focus states — use per-node `focused` flags in `widget_tree` instead.
- `--exec` clicks always carry `ModNone` → no Alt/Ctrl/Shift-click (Alt+Click add-cursor untestable).
- No `drag` verb → drag-selection unprobed.
- `key shift+tab` synthesizes `KeyTab+ModShift`, not `KeyBacktab` (BUG-004 needed an e2e event-injection test). No `backtab` keyword in the key parser.
- Synchronous `exec "Command Name"` bypasses the event loop's `syncStatus()`/render → a `screenshot`/`debug` taken immediately after shows a stale status bar and can mis-resolve clicks. Drive via `key ctrl+p` + type + `enter` when this matters.
- Screenshots carry no style/color info → theme/highlight/active-file-highlight checks are indirect.
- Functional batch harness runs all commands at once → can't do mid-session external `rm`/`touch` (BUG-031 is ledger-only for this reason; use `--exec` with a backgrounded shell `rm`, or an integration/PTY test).

### Cross-cutting root-cause clusters (for the eventual fix pass)

- **Missing `BatchCommand` wrapping** → BUG-012, 021, 022 (multi-command ops not atomic under undo). `ToggleLineComment` already does it right — copy that pattern.
- **Primary-cursor-only operations ignore `e.Multi`** → BUG-005, 006, 007, 008.
- **Line-range commands lack the col-0 convention** used by JoinLines/ToggleLineComment → BUG-001, 002, 003, 004.
- **Undo restores text but not the user's view** (cursor/viewport) → BUG-020, 023.
- **File ops mutate disk without stat-guard or tab-model reconciliation** → BUG-028, 029, 030, 031.
- **Fold state keyed by raw line number, no content anchor** → BUG-024, 026, 027.

### Remaining areas — ready-to-paste agent prompts

Every prompt below assumes the agent first reads `audit/agent-brief.md`. Prepend to each: *"First, read /home/enko/Documents/ttt/audit/agent-brief.md and follow it exactly."* and append the standard *"Already-known bugs (skip, see audit/2026-07-12-ux-bug-audit.md): BUG-001..NNN — don't re-report. Work in /tmp (mktemp -d) with files you create. Read-only on the repo; report findings in the exact format from the brief."* Bump the NNN to the current max as you go.

**Resize & layout** — (was in progress at pause; re-run if that agent's results were lost). `--exec` can only set size at startup (`--size WxH`), not mid-session — compare the SAME state at several fixed sizes (20x10, 30x15, 40x20, 80x24, 120x40, 200x50). Scan `widget_tree` at each for rects with w<=0/h<=0, children overflowing parents, overlapping siblings, nodes pushed off-screen. Probe: base editor, sidebar-open split divider, bottom-panel split heights, overlays (palette/quick-open/confirm/help dialogs) fitting/centering at tiny AND huge sizes, menu dropdowns at 20x10, status/menu bar at 20 cols, 300-char line at 40 vs 200 cols, word-wrap column per size, tab bar at 20 cols (rendering/overlap, not the BUG-016 chevron). A panic at any size is high severity.

**Themes & rendering** — Sonnet. Probe every built-in theme (list them from `internal/config/themes/*.json`): does each load without error and produce a complete style map (no missing StyleDef → default/black fallback)? Switch themes live (theme picker command) — stale cells or wrong colors after switch? Check the "never hardcode colors" rule holds — any widget rendering a literal color instead of a `term.Style*`? Diff-view background layering (syntax on top of diff bg via BgStyle). Cursor/selection visibility in each theme. Direct-color terminal rendering path. Rendering engine: double-buffer diff correctness — scroll a long file up/down and edit, look for stale/ghost cells not cleared; wide-char cell clearing (a CJK char deleted — does the second cell clear?); border/divider drawing at panel edges. Screenshots lack style info, so color-specific bugs need the widget_tree props or careful visual diffs; note where you can't confirm.

**Settings & options** — Haiku ok. `settings.json` load/save roundtrip (change a setting via the Options menu, does it persist?); malformed/partial settings.json — graceful recovery or crash? Each toggle in the Options menu actually takes effect (word wrap, line numbers, whitespace render, insert-final-newline, etc. — enumerate from the menu). Debounce settings (`autocomplete.debounce`, `search.debounce`) honored. Live settings reload (`reload_settings` path exists — see e2e test). Custom `keybindings.json` — a rebind takes effect; a malformed one recovers. Defaults match `DefaultSettings()`. Use `TTT_CONFIG_DIR` religiously so the real config is never touched.

**Workspace (multi-folder)** — Sonnet. Open 2+ folders (positional args and a `.ttt` workspace file via `--workspace`). `FolderForFile` longest-prefix match (open a file that could match two roots). Save/load a `.ttt` workspace file — roundtrip fidelity. Add/remove folder commands — tree + state update? git `IsRepo` detection per folder. File ops (new/rename/delete) targeting the correct folder when several are open. Open a workspace whose folder path no longer exists — graceful? Fallback to cwd when no folders given. Explorer showing multiple roots — per-root operations don't bleed across.

**Global search (sidebar, rg-based)** — Sonnet. This shells out to `rg` with debounced input (`search.debounce`), a generation counter + mutex to prevent races. Probe: type a query fast (debounce + generation — stale results from a prior query appearing?); results across multiple workspace folders; clicking a result navigates to the exact file/line/col; the editor-search-highlight lifecycle (docs: "cleared when switching away, re-applied from existing results when switching back") — switch panels and back, are highlights correct/stale? Regex + case toggles. Empty query. Regex-special chars. A query with thousands of hits (rendering/scroll). Search while editing the file that has hits. Requires `rg` installed. NOTE: known blind spot — synthetic clicks don't activate Changes-panel rows; the search results panel may or may not share that limitation, verify with keyboard nav too.

**Plugin widgets** — Sonnet. Use `--plugin FILE` with Lua scripts you write. Exercise each widget type from the PanelProxy API (label/title/tree/list/button/input/vstack/hstack/scrollview/box/divider/dropdown/progress/table/markdown/keyvalue) — does each render and respond to its callbacks (on_click/on_select/on_change/on_submit/on_command)? Box model (margins/padding) applied correctly. A plugin that throws a Lua error mid-render — does it crash the editor or contain the error? Plugin lifecycle, timers (`plugin-timers` test exists), `ttt.storage`. Raw cell API (cell/text/clear) mixed with widgets. Malformed widget descriptors. Focus routing into plugin panels. Report any editor crash from plugin misbehavior as high severity.

**Integrated terminal panel** — do LAST, Sonnet. PTY-heavy, harder via `--exec` (output is async; repros flakier — lean on the integration/PTY harness `tui-use` if needed, and note where `--exec` can't reach). Probe: open/close/toggle the terminal (`ctrl+t` default; `ctrl+backtick` is unbound per CLAUDE.md); key routing when focused (RawKeyConsumer — all keys to PTY); force-keys bypass (`terminal.toggle` must work even when terminal focused); VT rendering of colored/256-color output (direct-color path); terminal fullscreen (`alt+t`); focus toggle between editor and terminal; resize behavior; async output waking the event loop. A hang or crash is high severity. Config isolation matters doubly here (spawns a real shell).

**Keyboard navigation parity (complete the partial sweep)** — Haiku ok. BUG-017 (ctrl+home/end missing) was found incidentally; do the full systematic pass vs VS Code: word motions (ctrl+left/right) at line boundaries and across punctuation/underscores; Home/SmartHome behavior on indented lines; End; PgUp/PgDn (cursor + viewport, goal column); matching-bracket jump; go-to-line edge cases (0, negative, past EOF); select-variants (shift+ every motion) leave a correct selection; delete-word variants. Enumerate the movement bindings from `DefaultKeybindings()` and check each does what VS Code does.
