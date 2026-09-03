# Critical / High bugs still open

Re-verification of `audit/2026-07-12-ux-bug-audit.md` against `main` @ `b93468d` (v1.4.0-4), done 2026-09-02.

Method: re-ran the audit's own expected-failure suite (`tests/functional/audit-*.test.js`,
`internal/widgets/audit_clearrect_bug_test.go`) plus fresh `--exec` / code inspection.
Only **critical/high** findings that are **still reproducible** and have **no GitHub issue**
are listed here. Mediums/lows and already-fixed entries are summarised at the bottom.

---

## BUG-005 — Line commands under multicursor corrupt the buffer
- **Severity:** high (silent, destructive buffer corruption on a headline feature)
- **Status:** ✅ FIXED on `fix/multicursor-line-commands-stale-cursors` — `MoveLineUp/Down` now carry every cursor with its line (`moveLinesMulti`, `editor_widget_lines.go`), and `Duplicate/Delete/Join/Sort/Reverse/Unique/ToggleLineComment` collapse multicursor to the primary first (`collapseMultiForLineOp`). `audit-multicursor-bugs.test.js` BUG-005 flipped `it.fails`→`it` (2 cases).
- ~~**Status:** REPRODUCED on main (`--exec` + `audit-multicursor-bugs.test.js` still `it.fails`)~~
- **Repro:** file `foo bar foo baz\nfoo qux\nbar foo end\n`
  ```
  bin/ttt --size 120x40 --exec 'wait 200; key ctrl+k l; exec "Duplicate Line"; type Y; screenshot /tmp/s.txt; quit' foo.txt
  ```
- **Expected:** a line command under N cursors either shifts `Multi.Cursors` consistently or collapses multicursor mode; later typing only touches text a cursor covers.
- **Actual:** buffer becomes `foo bar Y baz` / `Yar foo baz` / `foo Y` / `bar foo end` — the `Y` lands in words no cursor ever selected. `DuplicateLine`/`DeleteLine`/`MoveLineUp`/`MoveLineDown` in `internal/ui/editor_widget_lines.go` never read or update `e.Multi.Cursors`; secondary cursors keep stale pre-shift offsets.
- **Fix direction:** collapse multicursor to the primary before running any line command (a line command under N cursors is ambiguous).
- **Manual repro (interactive):** scratch file with a repeated word:
  ```
  foo one
  foo two
  foo three
  ```
  1. Cursor at the very start of the file (on the first `foo`).
  2. `Ctrl+K` then `L` (`multicursor.selectAll`) → all three `foo` selected, status bar "3 cursors". *(This step is not the bug — it is the normal way to get multiple cursors.)*
  3. `Alt+↓` (Move Line Down) — or `Ctrl+K K` Delete Line, or palette → Duplicate Line.
  4. Type `X`.
  → buffer garbled: `foo two` / `Xne` / `X three`. The `X` landed at stale cursor offsets. Pressing `Esc` before step 4 (dropping the extra cursors) avoids it.
  Note: after step 3 the cursor count visibly drops (e.g. 3 → 2). That is not correct behaviour — it is the bug leaking: `MoveLineDown` moves lines without updating `Multi.Cursors`, so two cursors end up stacked on the same wrong position (and dedupe), and all of them now point at text that isn't what was selected.

## BUG-007 — Paste / Cut / Copy under multicursor only touch the primary selection
- **Severity:** high
- **Status:** REPRODUCED on main (code-confirmed + `audit-multicursor-bugs.test.js` still `it.fails`)
- **Repro:** select two `foo` occurrences (`ctrl+d` ×2), `Copy`, move away, `Paste`; only the primary is affected.
- **Actual:** `EditorGroupWidget.Copy`/`Cut`/`Paste` (`internal/ui/editor_group.go:1748-1808`) and `EditorPaneWidget.pasteText` (`internal/ui/editor_widget.go:297`) read only `t.Sel` / `e.Selection`; `e.Multi` is never consulted. Status bar still reports "(N cursors)".
- **Expected:** paste replaces every cursor's selection (VS Code semantics), or is an explicit no-op under multicursor.

## BUG-008 — Undo after a multicursor edit strands the cursor and corrupts on next keystroke
- **Severity:** high (silent buffer corruption, data loss)
- **Status:** ✅ FIXED on `fix/multicursor-line-commands-stale-cursors` — `EditorGroupWidget.Undo`/`Redo` collapse multicursor to a single cursor before running (`editor_group.go`). Text restores and the next keystroke inserts exactly one character. `audit-multicursor-bugs.test.js` BUG-008 flipped `it.fails`→`it`. (Cursor lands at the last edit site rather than the primary's pre-edit position — a pre-existing undo-cursor limitation, see BUG-020, not corruption.)
- ~~**Status:** REPRODUCED on main (`--exec` confirmed live)~~
- **Repro:** file `foo bar foo baz\nfoo qux\nbar foo end\n`
  ```
  bin/ttt --size 120x40 --exec 'wait 200; key ctrl+k l; type X; key ctrl+z; type Z; screenshot /tmp/s.txt; quit' foo.txt
  ```
- **Actual:** text restores, but the cursor jumps to the last secondary cursor's stale post-edit position, "(4 cursors)" persists, and one `Z` keystroke produces `foo barZ foo baz` / `fZoo qux` / `bar fZoZo end` (two Z's from one press). Undo (`internal/core/undo`) has no concept of `e.Multi`.
- **Expected:** undo restores text and either restores consistent multicursor selections or collapses to a single cursor at the primary's pre-edit position.
- **Manual repro (interactive):** same `foo one / foo two / foo three` scratch file:
  1. Cursor at start of file, `Ctrl+K` then `L` → 3 cursors on `foo`.
  2. Type `Q` (replaces each `foo`).
  3. `Ctrl+Z` — text looks fully restored to `foo one / foo two / foo three`.
  4. Type `X`.
  → `foo one` / `fXoo two` / `fXoXo three` — one keypress, two `X`s on the last line; cursors sit at stale offsets.
- **Fix direction:** collapse multicursor to a single cursor on Undo/Redo while `e.Multi` is active (`EditorGroupWidget.Undo`/`Redo`, `internal/ui/editor_group.go:1240-1272`). Shares the "collapse before non-multi-aware ops" fix with BUG-005.

## BUG-010 — Editor search matches go stale after buffer edits
- **Severity:** high
- **Status:** REPRODUCED on main (`audit-findreplace-bugs.test.js` BUG-010 still `it.fails`)
- **Repro:** `ctrl+f` query `alpha`, click into the editor, insert a line above the matches, press `F3`.
- **Actual:** `F3` navigates by the stale line index and lands on a non-matching line (e.g. `beta`). `Editor.SearchMatches` is never recomputed after buffer edits while the bar is open.
- **Expected:** matches shift with edited text (or are recomputed); Find Next never lands on a non-matching line.

## BUG-013 / BUG-048 — Editor search state is shared across tabs, not reset on switch
- **Severity:** high
- **Status:** REPRODUCED on main (code-confirmed: `SwitchTab`/`syncTabs` in `internal/ui/editor_group.go:747,1820` never touch `SearchQuery`/`SearchMatches`/`SearchActive`; `audit-findreplace-bugs.test.js` BUG-013 still `it.fails`)
- **Repro (BUG-013):** find `alpha` in tab A (1 match), switch to tab B (no matches), press `F3` → tab A's "1/1" still shown, `F3` clamps the stale line-5 target into tab B's 2-line buffer and moves the cursor to a meaningless position.
- **Repro (BUG-048):** navigate to a match in file B, switch tabs directly (tab bar / `alt+,`), press `F3` → cursor jumps to cross-file coordinates (a column/line valid only in the other buffer), with no bounds/identity check.
- **Expected:** Find Next after a tab switch no-ops or re-searches the active file. Search state must be per-tab or cleared on switch.

## BUG-020 — Undo of line commands never restores the cursor to the edit site
- **Severity:** high
- **Status:** REPRODUCED on main (`audit-undo-bugs.test.js` BUG-020 still `it.fails`)
- **Repro:** Delete Line at line 2, move the cursor away, `ctrl+z` → text restores but the cursor stays where it wandered.
- **Actual:** `cursorAfterUndo`/`cursorAfterRedo` (`internal/core/undo/undo.go`) have no cases for `InsertLineCommand`/`DeleteLineCommand`/`SwapLineCommand`/`ReplaceLinesCommand`, so `Undo()` returns nil and `editor_group.go` skips the cursor update. Affects Delete/Duplicate/Move/Sort Lines.
- **Expected:** undo returns the cursor to where the edit happened, as it does for typed text, paste, and Join Lines.

## BUG-026 — Fold collapsed-state reattaches to an unrelated block after line-count edits
- **Severity:** high (silently hides different code than the user folded)
- **Status:** REPRODUCED on main (`audit-fold-bugs.test.js` BUG-026 still `it.fails`)
- **Repro:** fold an inner `if true {` block, insert a blank line at the top of the file → the fold marker now collapses the *enclosing function's* body instead.
- **Actual:** `fold.State.SetRanges` (`internal/core/fold/fold.go:26-39`) recomputes ranges on every line-count change and preserves collapse purely by raw `StartLine` equality; after the shift a different block inherits the old collapsed key.
- **Expected:** collapsed state follows the folded content (or at worst clears); a region the user never folded must never become collapsed.

## BUG-027 — Move Line on a folded header swaps in hidden content (silent code reordering)
- **Severity:** high (silent code corruption)
- **Status:** REPRODUCED on main (`--exec` confirmed live: buffer became `func outer() {` / `\t\tfoo()` / `\tif true {` / … — `foo()` hoisted out of its block, fold marker still rendered as valid)
- **Repro:** file with an inner `if true {` block containing `foo()`; fold it, cursor on the header, run `Move Line Down`.
  ```
  bin/ttt --size 100x30 --exec 'wait 250; key down; exec "Toggle Fold"; wait 100; exec "Move Line Down"; debug /tmp/d.json; quit' fold.go
  ```
- **Actual:** `MoveLineDown`/`Up` issue a raw `SwapLineCommand` with no fold awareness; line COUNT is unchanged so the fold-recompute guard never fires and the stale marker keeps rendering over now-invalid structure.
- **Expected:** move the whole folded region as a unit (VS Code) or no-op while folded; never reorder invisible code.

## BUG-047 — Global-search navigation ignores the match column (cursor lands at col 0)
- **Severity:** high
- **Status:** REPRODUCED on main (`audit-global-search-bugs.test.js` BUG-047 still `it.fails`)
- **Repro:** sidebar search a term that occurs mid-line, activate that result → cursor at line N col 0 instead of the match column.
- **Actual:** `NavigateToSearchMatch` (`internal/app/callbacks.go`) receives `col` but never uses it; `GoToLine` unconditionally sets `Cursor.Col = 0` (`internal/ui/editor_group.go`).
- **Expected:** cursor lands at the match's exact column.

## BUG-052 — Plugin `p:clear()` with large dimensions freezes the editor
- **Severity:** high (a plugin can hang the whole editor — no bound check)
- **Status:** REPRODUCED on main (code-confirmed: `ClearRect` on both `virtualSurface` and `subVirtualSurface`, `internal/widgets/virtual_surface.go:53-58,112-118`, still loop `h*w` with no clamp; `audit_clearrect_bug_test.go` still `t.Skip`)
- **Repro:** a plugin whose `render` calls `p:clear(0, 0, 100000, 100000)`; activate its panel → UI frozen (~1e10 iterations block the render/event loop).
- **Expected:** `p:clear` clamps w/h to the surface bounds (or rejects the call).

## BUG-057 (+ BUG-058) — Dismissing a dialog while the terminal is focused steals focus to the editor; typing then corrupts the file
- **Severity:** high (silent data loss / trapping) — **filed as [#578](https://github.com/eugenioenko/ttt/issues/578), labelled low priority** (niche trigger, visible + undoable; fix is a coupled 3-parter — see the issue)
- **Status:** REPRODUCED on main (`--exec` confirmed live: buffer `hello world` → `XYhello world`)
- **Repro:** file containing `hello world`
  ```
  bin/ttt --exec 'wait 300; key ctrl+t; wait 500; key ctrl+p; wait 300; key escape; wait 200; type XY; debug /tmp/d.json; quit' f.txt
  ```
- **Actual:** `App.DismissDialog()` (`internal/app/app.go:664-667`) unconditionally calls `FocusEditor()` regardless of what was focused before the dialog opened. Keystrokes meant for the shell land in the editor buffer.
- **Expected:** dismissing a dialog returns focus to whatever was focused before (the terminal).

## BUG-059 — Ctrl+K never reaches the PTY when the terminal is focused
- **Severity:** high
- **Status:** REPRODUCED on main (code-confirmed: `Root.HandleEvent` runs `handleChord(kev)` at `internal/ui/root.go:240` BEFORE the `rawKeysFocused()` check at `:244`)
- **Repro:** focus the integrated terminal, type `foobar`, `ctrl+a`, `ctrl+k` (readline kill-line), Enter → the shell runs `foobar` instead of an empty line; `^K` never arrives.
- **Actual:** ~20 commands use `ctrl+k` as a chord prefix, so the chord matcher always consumes the first Ctrl+K and never forwards it. Breaks readline Ctrl+K and any other chord-prefix key in shell apps.
- **Expected:** per AGENTS.md, all keys route to the PTY when the terminal is focused, except force keys — so the RawKeyConsumer check must run before chord matching.

---

## Not listed here (for reference)

**Fixed since the audit** (audit repro tests flipped to real `it()` and passing on main):
BUG-001, BUG-002, BUG-003 (col-0 selection cluster), BUG-011 (Replace All case toggle),
BUG-021, BUG-022 (atomic undo for indent / auto-indent Enter), BUG-030 (New File/Rename
clobber guard), BUG-044 (git branch below repo root).

**Still reproducible but medium/low severity** (left in the main ledger, not escalated):
BUG-004, BUG-006, BUG-009, BUG-012, BUG-014, BUG-015, BUG-016, BUG-017, BUG-018, BUG-019,
BUG-023, BUG-024, BUG-025, BUG-033, BUG-036, BUG-037, BUG-038, BUG-039, BUG-040, BUG-041,
BUG-042, BUG-043, BUG-045, BUG-046, BUG-049, BUG-050, BUG-051, BUG-053, BUG-054, BUG-055,
BUG-056, BUG-058.

**High findings already tracked elsewhere or resolved by the audit's own follow-up:**
BUG-028/029/031 (explorer file-op reconciliation — partly addressed on `fix/explorer-rename-tab-sync`),
BUG-034/BUG-035/BUG-032 (rejected as intended behaviour during curation).

**Now tracked:**
BUG-057 + BUG-058 → [#578](https://github.com/eugenioenko/ttt/issues/578) (overlay dismiss always
re-focuses the editor; force keys punch through modal overlays), labelled low priority.

**Related existing issues (not duplicates of the above):**
#310 (MoveLine/JoinLines non-atomic undo — low), #303 (HasOverlay guards — closed),
#371 (Tab/Shift+Tab multicursor — closed; does not cover BUG-005/007/008).
