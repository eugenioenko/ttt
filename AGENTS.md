# AGENTS.md

This file provides guidance to AI coding agents (Claude Code, Codex, Cursor, and others) when working with code in this repository.

## Project Overview

ttt is a terminal text editor written in Go, using tcell for terminal rendering. The Go module is `github.com/eugenioenko/ttt`.

## Build & Test Commands

```sh
make build        # builds to bin/ttt
make run          # build + run
make test         # go test ./...
make fmt          # gofmt -w .
make lint         # golangci-lint run
go test ./internal/core/buffer/   # run tests for a single package

# Open a multi-folder workspace
bin/ttt --workspace project.ttt

# Open specific folders or files
bin/ttt ~/projectA ~/projectB file.go
```

## Architecture

[`ARCHITECTURE.md`](ARCHITECTURE.md) is the source of truth for package ownership and the architecture convergence plan. The codebase uses dependency zones rather than a strict linear layer chain: domain, services, presentation kernel, product presentation, application, plugin host, and platform.

Known boundary violations and explicit boundary decisions are documented there. Highlighting is presentation-owned at `internal/highlight`; grammar selection, caching, and TextMate scope to `term.Style` mapping stay together there. tcell events are intentionally used across `term`, `widgets`, `ui`, and narrow application/platform wiring. Do not create cosmetic wrappers merely to satisfy the old layer diagram.

### Packages

Packages are grouped by the dependency zones in [`ARCHITECTURE.md`](ARCHITECTURE.md), which is the source of truth for zone membership and dependency direction.

**Domain** (`internal/core/`): UI-agnostic editor engine. Domain code must not start processes, access terminal state, render widgets, or coordinate application lifecycle.

- **`core/buffer/`**: line-based text storage (`[]string`), rune-level insert/delete, file I/O (load/save).
- **`core/cursor/`**: visual column cursor with goal-column preservation for vertical movement.
- **`core/undo/`**: command-pattern undo/redo via the `EditCommand` interface; `BatchCommand` groups edits into one undo step.
- **`core/selection/`**: selection ranges and text extraction.
- **`core/multicursor/`**: multi-cursor state (add, dedupe, collapse).
- **`core/fold/`**: indentation-based fold ranges and fold state.
- **`core/diff/`**: line diffing, unified diff generation and parsing, and git gutter change kinds.
- **`core/clipboard/`**: clipboard with system, OSC 52, and process-local backends. It starts processes, which the Domain rules forbid; treat it as an existing exception, not a pattern for new domain code.

**Services**: external-process and external-state integration, exposing typed operations and results without owning widgets.

- **`internal/git/`**: git CLI wrapper (status, staging, commit, repo discovery).
- **`internal/github/`**: `gh` CLI wrapper for pull requests.
- **`internal/lsp/`**: language server client (see LSP Integration).
- **`internal/terminal/`**: integrated terminal emulator. Wraps `gitpod-io/xterm-go` for VT parsing and `aymanbagabas/go-pty` for PTY lifecycle.
- **`internal/watcher/`**: fsnotify-based reporting of on-disk changes to open files and watched directories.
- **`internal/workspace/`**: multi-folder workspaces. `Folder` and `Workspace` track project roots, with `IsRepo` git detection, `FolderForFile` lookup (longest-prefix match), and JSON `.ttt` workspace files. Falls back to `cwd` when no folders are given.

**Presentation kernel**: screen cells, styles, width measurement, rendering, layout, and reusable interaction primitives.

- **`internal/term/`**: `Screen` interface. `TcellScreen` is the real implementation; `MockScreen` supports unit-level `Screen` and renderer tests; `SimScreen` implements tcell's screen contract for composed E2E and chaos tests. Also defines `DirectColor` and `CellAttr` for direct RGB rendering (used by the integrated terminal to bypass the style map for 256-color output).
- **`internal/render/`**: diff-based renderer that compares prev/curr cell grids and emits minimal updates.
- **`internal/textwidth/`**: display-width measurement (`Rune`, `String`, `Runes`), the single source of truth for how many terminal columns text occupies. Wraps `clipperhouse/displaywidth` with the same options tcell v3 uses internally, including the `RUNEWIDTH_EASTASIAN` toggle, so layout always matches what tcell draws.
- **`internal/highlight/`**: presentation-owned syntax highlighting via [textmate-go](https://github.com/eugenioenko/textmate-go) (VS Code's TextMate engine, 124 embedded grammars). Owns grammar selection, the incremental document, caching, and mapping TextMate scopes to `term.Style` (the engine's token category, refined by the token's innermost scope). Full-buffer re-tokenizing is a known performance trap; the engine's `Document` avoids it.
- **`internal/view/`**: viewport (scrolling, cursor-to-screen mapping) and the segment-based status bar.
- **`internal/widgets/`**: reusable widget primitives backing both the Plugin Widget API and core panels (tree, table, list, input, dialog, dropdown, tabs, stacks, scrollview, markdown, and so on). `surface.go`/`virtual_surface.go` provide the drawing surface abstraction; `focus.go` handles focus traversal.
- **`internal/markdown/`**: goldmark-based markdown to styled lines.

**Product presentation**:

- **`internal/ui/`**: editor and panel widgets: `EditorGroupWidget`/`EditorPaneWidget` (tabs and editing), sidebar, bottom panel, search, diff view, menus, dialogs. Notable files: `root.go` (`Root`, overlays, key matching, force keys, and the `RawKeyConsumer` interface), `terminal_widget.go` (renders the terminal grid as direct-color cells, translates keys to VT sequences), `content_split.go` (focus routing between editor and bottom panel).

**Application**:

- **`internal/app/`**: application orchestration, the largest package. `App` (`app.go`) wires everything together. `commands*.go` implement command handlers by domain. `eventloop.go` and `keys.go` run the main event loop and key dispatch. Also: explorer and changes panel, git gutter and PR views, the output panel (`output.go`), plugin host UI, menus, formatter, LSP document symbols.
- **`internal/command/`**: command `Registry` (register, look up, execute by ID).

**Plugin host**:

- **`internal/plugin/`**: Lua plugin engine (gopher-lua). `manager.go`/`registry*.go` handle discovery, loading, and the community registry; `permissions.go`/`sandbox.go` enforce the permission model; `lua_*.go` bind the `ttt` Lua module by domain.

**Platform**:

- **`cmd/ttt/main.go`**: entry point. Parses flags (`--workspace`, `--exec`, `--size`, and so on), sets up the screen, logging, and panic handling, constructs `App`, and wires the plugin host APIs.

**Not yet assigned a zone in ARCHITECTURE.md**:

- **`internal/config/`**: settings, keybindings (`DefaultKeybindings()`), themes (`theme.go`), and `.editorconfig` support.
- **`internal/image/`**: image decoding and Kitty graphics protocol placement.
- **`internal/icons/`**: named UI glyphs in Nerd Font or plain form. **`internal/fileicons/`**: file name to Nerd Font glyph mapping (generated from nvim-web-devicons).

### Design Principles

1. **UX comes first for user-facing work.** Implement the intended interaction and presentation before optimizing implementation shortcuts. Architecture refactors start with characterization tests and preserve existing UX unless the PR explicitly declares a behavior change.
2. **Single source of truth for layout.** When Render computes layout values (positions, offsets), store them on the struct so event handlers reuse them directly instead of recalculating — divergent calculations cause click offset bugs.

### Key Design Constraints

- Cursor `Col` is a rune index, not a byte index — all line-length calculations use `[]rune()`. It is **not** a terminal column: a fullwidth rune advances `Col` by 1 and the screen by 2.
- **Never compute display width by hand.** No `len([]rune(s))`, `visCol++`, or `x++` as a stand-in for terminal columns — use `textwidth.String`/`textwidth.Rune`. Fullwidth East Asian runes occupy two columns, and tcell advances the terminal cursor by the rune's width when it draws: a layout that assumes one column per rune writes the next character into a cell the terminal never displays, so it vanishes (issue #434). Three distinct quantities must not be conflated:
  - **byte offset** ↔ **rune index** — `editor.byte_to_col`/`col_to_byte` in the Lua API (`internal/plugin/lua_editor.go`); no width involved.
  - **rune index** ↔ **terminal column** — `bufColToVisualCol`/`visualColToBufCol` (`internal/ui/editor_widget_utils.go`); width-aware.
  - A widget that draws left to right should take its width from what `DrawText` returns rather than measuring the same string a second time.
- A fullwidth rune must never be drawn in the last column of a clip region: the terminal paints it across two columns regardless of clipping, so it bleeds over the border or scrollbar to its right. `DrawText` substitutes a space in that case.
- The renderer uses double-buffering (prev/curr cell grids) to minimize terminal writes.
- `Screen` isolates terminal drawing and screen lifecycle. tcell events remain the shared presentation event model in `term`, `widgets`, `ui`, and narrow application/platform routing. Domain and service packages must not import tcell.
- **Never hardcode colors.** All colors must go through the theme system (`internal/config/theme.go` → `StyleDef` → `term.Style` constants → `BuildStyleMap()` in `internal/app/theme.go`). Add a new `StyleDef` field to `ThemeConfig`, a `term.Style` constant, and wire it in `BuildStyleMap()`. Widgets reference `term.Style*` constants, never color values. The one exception is the integrated terminal, which uses direct RGB color rendering via `DirectColor`/`CellAttr` to support 256-color output.
- **Terminal colors** are configured via the `terminal` field in `ThemeConfig` (`TerminalColors`), which holds 16 ANSI colors plus foreground/background defaults.
- The diff view layers syntax highlighting on top of diff background colors using `BgStyle` layering.
- **RawKeyConsumer interface**: when the integrated terminal is focused, all key events are routed directly to the PTY. Only force-keys (Ctrl+`) bypass this to allow toggling the terminal panel.
- Async PTY output wakes the event loop via `PostEvent`/`EventInterrupt`.
- **Output panel** (`internal/app/output.go`) is a core surface, not a plugin console. Producers are plugin `ttt.log`, language servers (`lsp:<server>`), and every status bar notification (`notice`). Append via `App.LogOutput` on the main thread, or `App.LogOutputAsync` from a background goroutine — it routes through `OutputLineResult` on the event loop, because widget state must not be mutated off the main thread. `LogOutput` also mirrors to `slog`, so `ttt.log` (debug builds) stays a superset of the panel. The panel is capped at `outputMaxLines` and trims in chunks; append with `TreeWidget.AppendItem`, never by rebuilding the slice for `SetItems`.
- **Global search** (`search_widget.go`) shells out to `rg` (ripgrep) with debounced input (`search.debounce` setting). Uses a generation counter and mutex to prevent concurrent searches from racing. Editor search highlights are tied to the search panel lifecycle — cleared when switching away, re-applied from existing results when switching back.

### Keybinding System & tcell Key Mapping

Keybindings are defined in `internal/config/keybindings.go` (`DefaultKeybindings()`) and converted to tcell key constants via `comboToTcell()` in `internal/app/keys.go`. The matching happens in `matchKey()` in `internal/ui/root.go`.

**Critical: tcell control key behavior.** For Ctrl+letter, tcell v3 (legacy mode, which ttt uses) delivers events with **both** the `KeyCtrlA..Z` constant **and** `ModCtrl` set. When registering control key bindings in `comboToTcell`, do NOT strip `ModCtrl` — the registered modifier must match what tcell delivers, otherwise `matchKey()` will fail silently. Ctrl+punctuation control chars (space, backtick, `/`, `\`, `]`, `^`, `_`) have no `KeyCtrl*` constant in v3 — they arrive as `KeyRune` + `ModCtrl` + a printable string (the exact string differs between legacy terminals and kitty-protocol terminals). `foldCtrlEvent()` in `internal/ui/root.go` folds both encodings to the canonical registered form: ctrl+space and ctrl+backtick → `KeyNUL`+`ModCtrl`, ctrl+/ → `KeyUS`+`ModCtrl`. New ctrl+punctuation bindings need a fold entry there.

**Ctrl+Backtick (`` ctrl+` ``):** On legacy terminals Ctrl+` sends NUL (0x00), same as Ctrl+Space — they are indistinguishable, and both fold to `KeyNUL`+`ModCtrl`. Kitty-protocol terminals (Ghostty, Kitty, WezTerm) do report them distinctly under tcell v3, but ttt currently folds both to the same canonical key, so they remain one binding.

**Force keys:** Bindings for commands in the `config.ForceKeyCommands` map (`internal/config/keybindings.go`, registered via `root.AddForceKey()` in `internal/app/commands.go`) are checked even when a `RawKeyConsumer` (like the integrated terminal) has focus. `terminal.toggle` must remain a force key.

**Keymap source of truth:** `DefaultKeybindings()` in `internal/config/keybindings.go` is canonical. `config/keybindings.json` is a generated mirror of it (for docs and as a user reference) — when changing defaults, update both, plus the README and docs-web keybinding tables.

### LSP Integration

Language server support lives in `internal/lsp/`: a JSON-RPC 2.0 client over stdio with Content-Length framing and no external dependencies, one client per language, lazy-started on first use. Servers are configured under `lsp.servers` in settings (`LSPSettings` in `internal/config/settings.go`); `internal/app/app_lsp.go` wires the client into the editor.

Async LSP results (completions, signature help, hover, and so on) wake the event loop with the same `PostEvent(EventInterrupt)` pattern as git blame. Document sync is full-document, not incremental.

### Plugin API

The plugin API reference (Widget API, raw cell API, box model, named styles, status bar items, and every `ttt.*` function) lives in `docs-web/src/content/docs/guides/plugin-authoring.md`, with `plugins.md` and `plugin-testing.md` alongside it. Read it before re-deriving the API from source, and update it in the same PR as any API change.

Implementation: Lua bindings in `internal/plugin/lua_panel.go`, descriptors in `widget_desc.go`, Go widget construction in `widget_builder.go`, named styles in `styles.go` (`StyleByName()`), widget types in `internal/widgets/`.

Things the docs do not cover:

- `ttt.*` callbacks (`notify`, `set_status_item`, `exec_command`, ...) only work after `WirePlugin`, which runs after `InitFromSource`. Call them from command handlers or event callbacks, not at plugin load time.
- Status bar segments (`view.StatusBar`, `StatusSegment`) are shared by core and plugins. Lower priority sits closer to the edge. Core segments use priorities below 1000 (grep `StatusSegment{` for the current values); plugin segments default to 1000 and are ID-scoped as `pluginName:id`.

### Testing

The project has four levels of testing:

**Unit tests** (`internal/*/`) — Standard Go tests for individual packages. Core algorithms are testable without presentation dependencies; syntax-highlighting characterization and performance tests live with `internal/highlight`. Run with `go test ./internal/core/buffer/` or `make test` for all.

**E2E tests** (`tests/e2e/`) — Go tests that wire up the full `App` with a `term.SimScreen` (an in-memory `tcell.Screen`). The `testHarness` (`harness_test.go`) creates a temp directory with sample files, builds the complete app (config, commands, keybindings, renderer), and provides helpers: `pressKey()`, `pressRune()`, `click()`, `exec()`, `screenText()`, `assertContains()`. These tests run single-threaded (no event loop goroutine) — the test drives events and redraws manually.

**Functional tests** (`tests/functional/`) — JavaScript tests using vitest that drive the real compiled `bin/ttt` binary via the `--exec` debug harness. The `tui.js` wrapper accumulates commands (type, press, exec, snapshot) and runs them in a single batch via `execFileSync`. No external dependencies beyond vitest. Run with `cd tests/functional && pnpm test`. The binary must be built first (`make build`).

Scripted key, mouse, and command actions are acknowledged after main-thread handling and redraw, so do not add sleeps between synchronous actions. For genuinely asynchronous work, wait for a unique post-transition screen state that proves the result was applied. Use raw elapsed waits only when timing or delayed lifecycle behavior is itself the invariant.

The batch pattern: `tui.start(file)` resets state, commands accumulate, `tui.snapshot()` returns an index, `tui.run()` executes all commands and returns `{ snapshots: string[] }`. Assertions happen after `run()`:
```js
tui.start(file);
tui.type("hello");
const s0 = tui.snapshot();
const { snapshots } = tui.run();
expect(snapshots[s0]).toContain("hello");
```

**Integration tests** (`tests/integration/`) — JavaScript tests using vitest + the locally pinned `tui-use` CLI to drive the binary via a real PTY. Used for tests that need live PTY interaction: LSP, external file changes, settings roundtrip, bracketed paste. Run with `cd tests/integration && pnpm install && pnpm test`.

### Test expectations for changes

Choose the smallest deterministic layer that proves the intended invariant, then add broader coverage only when it proves a distinct boundary:

1. **Unit tests** — pure algorithms, state models, parsers, and lifecycle helpers.
2. **E2E tests** — composed editor and App behavior on `term.SimScreen`.
3. **Functional tests** — real-binary behavior that depends on startup, command dispatch, file effects, or visible composition. Use `tui.exec("Command Name")`, `tui.pressChord("ctrl+k", "x")`, and `tui.snapshot()`.
4. **Integration tests** — only behavior that genuinely requires a live PTY or external process boundary, such as terminal byte encoding, terminal modes, or real language-server compatibility.

The functional suite is a compact real-binary contract, not a mandatory duplicate of lower-layer coverage. An invariant proved at a lower deterministic boundary does not also require a functional test; add a higher-boundary test only for behavior unique to that boundary. During implementation, run focused tests for the affected contract. CI remains the broad regression gate before merge.

### Debug harness (`--exec`, `--plugin`, `--size`, `--debug`, `--listen`)

**USE THIS FOR DEBUGGING AND TESTING.** The editor has a built-in scripted interaction system that is faster than TUI tests and gives you direct access to internal state — reach for it before investigating UI bugs manually.

**`--exec "commands"`** — Execute semicolon-separated commands after startup. Run the real binary, interact with it, capture state, and exit — all in one command:

```bash
bin/ttt --size 120x40 --exec "wait-for Explore; screenshot /tmp/screen.txt; debug /tmp/state.json; quit"
cat /tmp/screen.txt   # see what's rendered
cat /tmp/state.json   # see full widget tree, focus, selection, panels
```

Supported commands (the source of truth is `ExecScriptUsage()` in `internal/app/exec_script.go`; keep this list in sync with it):
- `click X Y` — simulate left mouse click (press + release) at coordinates
- `rclick X Y` — simulate right mouse click at coordinates
- `hover X Y` — simulate mouse hover (move) at coordinates
- `drag X1 Y1 X2 Y2` — simulate a mouse drag between two points (interpolated over 10 steps)
- `key COMBO` — simulate key press (e.g. `key ctrl+p`, `key enter`, `key ctrl+k x`)
- `type TEXT` — type a string of text
- `paste TEXT` — simulate a bracketed paste (terminal paste)
- `copy` — copy the current selection to the clipboard
- `exec "Command Name"` — run a command by title (same as command palette)
- `screenshot PATH` — save screen text to file
- `debug PATH` — save debug state JSON (screen, cursor, buffer, focus, panels, tabs, selection, output log, integrated-terminal raw PTY byte tails, full widget tree with rect/focus/props per node)
- `wait MS` — wait milliseconds
- `wait-for TEXT [timeout=MS]` — wait until text appears on the actual visible screen; defaults to a bounded timeout. Quote text to preserve surrounding whitespace or escapes.
- `panel ID` — show and focus a bottom panel by ID
- `quit` / `shutdown` — exit the editor

Scripted input and main-thread commands are acknowledged after the event loop handles and redraws them, so following actions observe completed visible state. Invalid actions, missing commands/panels, capture failures, and wait timeouts stop the script: CLI `--exec` reports the error on stderr and exits nonzero; `POST /exec` returns a non-2xx response with the same detail.

**`--listen`** — Start an HTTP command server on `127.0.0.1:4242` (loopback-only — never exposed off the local machine). `POST /exec` runs the same script format as `--exec`, synchronously, against an **already-running** editor — for capturing a repro at the exact moment it happens instead of scripting it in advance. The editor must run in a real terminal (TTY); it is started by a person, and the agent drives it with `POST /exec`:

```bash
bin/ttt --listen &
curl -X POST --data "type hi; wait-for hi; screenshot /tmp/screen.txt" http://127.0.0.1:4242/exec
curl -X POST --data "shutdown" http://127.0.0.1:4242/exec
```

Pass `?sep=` to use a different command separator, mirroring `--exec-split-on`.

**`--size WxH`** — Force screen dimensions for deterministic layout (e.g. `--size 120x40`). Essential for reproducible screenshots and coordinate-based click tests.

**`--plugin FILE`** — Load a Lua plugin file on startup with full permissions. For more complex test scenarios that need callbacks, state, or event handling. **Important:** Plugin files must `local ttt = require("ttt")` before using any `ttt.*` API — the module is preloaded, not global. Callbacks (e.g., `ttt.notify`, `ttt.set_status_item`) only work after `WirePlugin` runs, which happens after `InitFromSource`, so they must be called from command handlers or event callbacks, not at init time.

**`--debug`** — Enable debug mode regardless of config setting.

**`TTT_CONFIG_DIR` env var** — overrides the config directory entirely (settings, keybindings, themes, plugins, plugin registry). Always set this when running scripted `--exec` sessions that touch settings or plugins, so the developer's real `~/.config/ttt` is not read or mutated. The functional test harness (`tests/functional/tui.js`) sets it automatically.

Headless `--exec` sessions use a process-local clipboard, so concurrent automation cannot overwrite the desktop clipboard or each other's copied text. Interactive sessions, including `--listen`, continue to use the system clipboard.

**Lua API equivalents** — Plugins can also call `ttt.screenshot(path)`, `ttt.debug(path)`, `ttt.click(x, y)`, and `ttt.quit()` directly.

**Command palette** — `Debug: Screenshot`, `Debug: Dump State`, `Debug: Simulate Click`, `Debug: Run Current File as Plugin` are available for interactive debugging.

### Implementation patterns

- **Undo contract**: all buffer mutations must go through the undo system via an `EditCommand` (in `internal/core/undo/`). Never modify `Buf.Lines` directly — create or reuse a command struct so undo/redo works.
- **Command naming**: use `domain.verbNoun` — e.g. `editor.joinLines`, `fold.toggle`, `multicursor.selectAll`.
- **Selection operations**: check `Selection.Active` first. Use `Selection.Range(cursor.Line, cursor.Col)` for bounds. Convention: if no selection, operate on all lines (for line-based commands) or no-op (for text transforms).
- **Keybindings**: `ctrl+shift` combos are unreliable in terminals — avoid them. Use `ctrl+k <key>` chords for new commands. Check `DefaultKeybindings()` in `internal/config/keybindings.go` before assigning to avoid collisions. If no obvious binding exists, leave the command as command palette only — not every command needs a keybinding.
- **Overlay stacking**: commands that open overlays via keybindings must guard against being called twice with `if a.Root.HasOverlay() { return }`. `ShowDialog`/`ShowConfirmDialog` themselves have no guard so legitimate stacking (e.g. quit confirm) still works.
- **Command handlers**: define handlers as named methods on `App` (e.g. `app.ExplorerRename`) and reference them in `reg.Register(...)`. Do not use inline closures for non-trivial handlers.
- **Comments: only critical ones.** Add a comment only when missing it would cause a bug or misuse: a hidden constraint, a non-obvious invariant, or a workaround for a specific bug (the `textwidth`/fullwidth-rune notes above show the bar to clear). Never comment what the code does, restate an identifier, narrate the change, or add docstrings for coverage; well-named identifiers already do that.

### Post-implementation review

After a feature is implemented and tests pass, review all changes for cleanup: dead code, unnecessary complexity, naming inconsistencies, or missing edge cases. Fix anything related to the feature in the same PR. If you spot something unrelated that needs attention, create a GitHub issue for it instead of fixing it in the current PR.

### Opening a pull request

- **Features need an accepted issue first.** Link it in the PR body (`Closes #123`). Bug fixes, docs, and small cleanups can go straight to a PR.
- **One concern per PR.** Keep it under roughly 600 changed lines; split larger work into a sequence of PRs.
- **Title** uses conventional commits: `type(scope): description`.
- **Body** explains why the change is needed, which test layer covers it and what that test proves, and, for visible changes, includes a screenshot captured with `--exec "...; screenshot PATH"`.
- **Comments:** only critical ones (see Implementation patterns). Remove comments that describe what the code does before opening.
- **Before opening:** `make test` and `make lint` pass, and the change has been exercised in the real binary.
- **AI-assisted PRs are welcome**, but the human submitting it must have run the change and be able to explain every line.
