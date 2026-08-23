# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

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

Known boundary violations and explicit boundary decisions are documented there. In particular, `core/highlight` still imports `term`; this dependency is frozen until its cleanup lane removes or reclassifies it. tcell events are intentionally used across `term`, `widgets`, `ui`, and narrow application/platform wiring. Do not create cosmetic wrappers or move files merely to satisfy the old layer diagram.

### Packages

- **`internal/core/`** — UI-agnostic editor engine. New domain APIs must not introduce terminal or rendering dependencies; `core/highlight` is the tracked boundary violation scheduled for cleanup.
  - `buffer/` — Line-based text storage (`[]string`), rune-level insert/delete, file I/O (load/save)
  - `cursor/` — Visual column cursor with goal-column preservation for vertical movement
  - `undo/` — Command-pattern undo/redo via `EditCommand` interface (InsertRune, DeleteRange, InsertLine)
  - `highlight/` — Per-line syntax highlighting via `chroma/v2` lexers (`Highlighter` interface with `Span` output). Full-buffer re-lexing is a known perf trap (~95µs/line) — avoid it.

- **`internal/view/`** — Viewport (scrolling, cursor-to-screen mapping) and status bar rendering

- **`internal/render/`** — Diff-based renderer: compares prev/curr cell grids and emits minimal updates

- **`internal/terminal/`** — Integrated terminal emulator. Wraps `eugenioenko/vt10x` (a fork of `hinshun/vt10x`) for VT escape sequence parsing and `aymanbagabas/go-pty` for PTY lifecycle management. Provides the backing state for terminal tabs.

- **`internal/term/`** — Terminal abstraction via `Screen` interface. `TcellScreen` is the real implementation. `MockScreen` supports unit-level `Screen` and renderer tests; `SimScreen` implements tcell's screen contract for composed E2E and chaos tests. Also defines `DirectColor` and `CellAttr` types for direct RGB color rendering (used by the terminal emulator to bypass the style map for 256-color support).

- **`internal/ui/`** — Window manager and pane system. `Window` binds a `Rect`, `Viewport`, and `Buffer` together. `WindowManager` tracks focus across windows. Also contains `terminal_widget.go` (renders vt10x grid as direct-color cells, handles key-to-VT translation), `root.go` (ForceKeys and RawKeyConsumer interface for terminal key routing), and `content_split.go` (OnTopClick/OnBottomClick for focus routing between editor and bottom panel).

- **`internal/textwidth/`** — Display-width measurement (`Rune`, `String`, `Runes`). The single source of truth for how many terminal columns text occupies. Wraps `clipperhouse/displaywidth` with the same options tcell v3 uses internally, including the `RUNEWIDTH_EASTASIAN` toggle, so ttt's layout always matches what tcell draws.

- **`internal/workspace/`** — Multi-folder workspace management. `Folder` and `Workspace` types track one or more project roots, with `IsRepo` git-detection, `FolderForFile` lookup (longest-prefix match), and JSON-based workspace file loading/saving (`.ttt` files). The editor falls back to `cwd` when no folders are explicitly provided.

- **`internal/app/`** — Application orchestration layer; the largest package in the codebase. `App` (`app.go`) owns wiring between all other layers. `commands*.go` files implement command handlers by domain (editor, explorer, git, search, settings, view, palette, debug, plugin, options, help). `eventloop.go` and `keys.go` handle the main event loop and key dispatch. Other notable files: `explorer.go`/`changes_panel.go` (file tree and git changes panel), `gitgutter.go`/`repo_ops.go`/`pr.go` (git integration), `output.go` (output panel, see Key Design Constraints), `plugin_api.go`/`plugins_panel.go`/`plugin_detail.go` (plugin host UI), `menubar.go`/`menus.go`, `formatter.go`, `symbols_go.go`/`symbols_panel.go` (LSP document symbols).

- **`internal/plugin/`** — Lua plugin engine (gopher-lua based). `manager.go`/`registry.go`/`registry_remote.go` handle plugin discovery, loading, and the remote community registry. `permissions.go`/`sandbox.go` enforce the plugin permission model. `lua_*.go` files bind Go functionality into the `ttt` Lua module by domain (editor, fs, net, events, commandline, diagnostics, settings, system, json, callbacks). `panel_widget.go`/`widget_builder.go`/`widget_desc.go` implement the Plugin Widget API (see below); `styles.go` maps named styles to `term.Style*`. User-facing plugin authoring docs live in `docs-web/src/content/docs/guides/plugin-authoring.md` (also `plugins.md`, `plugin-testing.md`) — check there before re-deriving plugin API usage from source.

- **`internal/widgets/`** — Reusable UI widget primitives that back both the Plugin Widget API and core app panels: `tree.go`, `table.go`, `list_widget.go`, `input.go`, `dialog.go`, `dropdown.go`, `tabs.go`/`tabbed.go`, `hstack.go`/`vstack.go`, `scrollview.go`/`scrollbar.go`, `label.go`/`title.go`/`text.go`, `button.go`/`checkbox.go`, `progress.go`, `markdown.go`, `box.go`/`divider.go`. `surface.go`/`virtual_surface.go` provide the drawing surface abstraction; `focus.go` handles focus traversal; `builder.go` is shared construction plumbing.

- **`cmd/ttt/main.go`** — Entry point with event loop. Wires all components together, handles key dispatch, viewport scrolling, and redraw. Accepts a `--workspace <file>` flag to open a saved workspace, or folder/file paths as positional arguments.

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
- **Never hardcode colors.** All colors must go through the theme system (`internal/config/theme.go` → `StyleDef` → `term.Style` constants → `buildStyleMap`). Add a new `StyleDef` field to `ThemeConfig`, a `term.Style` constant, and wire it in `buildStyleMap()`. Widgets reference `term.Style*` constants, never color values. The one exception is the integrated terminal, which uses direct RGB color rendering via `DirectColor`/`CellAttr` to support 256-color output.
- **Terminal colors** are configured via the `terminal` field in `ThemeConfig` (`TerminalColors`), which holds 16 ANSI colors plus foreground/background defaults.
- The diff view layers syntax highlighting on top of diff background colors using `BgStyle` layering.
- **RawKeyConsumer interface**: when the integrated terminal is focused, all key events are routed directly to the PTY. Only force-keys (Ctrl+`) bypass this to allow toggling the terminal panel.
- Async PTY output wakes the event loop via `PostEvent`/`EventInterrupt`.
- **Output panel** (`internal/app/output.go`) is a core surface, not a plugin console. Producers are plugin `ttt.log`, language servers (`lsp:<server>`), and every status bar notification (`notice`). Append via `App.LogOutput` on the main thread, or `App.LogOutputAsync` from a background goroutine — it routes through `OutputLineResult` on the event loop, because widget state must not be mutated off the main thread. `LogOutput` also mirrors to `slog`, so `ttt.log` (debug builds) stays a superset of the panel. The panel is capped at `outputMaxLines` and trims in chunks; append with `TreeWidget.AppendItem`, never by rebuilding the slice for `SetItems`.
- **Global search** (`search_widget.go`) shells out to `rg` (ripgrep) with debounced input (`search.debounce` in settings.json, default 350ms). Uses a generation counter and mutex to prevent concurrent searches from racing. Editor search highlights are tied to the search panel lifecycle — cleared when switching away, re-applied from existing results when switching back.

### Keybinding System & tcell Key Mapping

Keybindings are defined in `internal/config/keybindings.go` (`DefaultKeybindings()`) and converted to tcell key constants via `comboToTcell()` in `internal/app/keys.go`. The matching happens in `matchKey()` in `internal/ui/root.go`.

**Critical: tcell control key behavior.** For Ctrl+letter, tcell v3 (legacy mode, which ttt uses) delivers events with **both** the `KeyCtrlA..Z` constant **and** `ModCtrl` set. When registering control key bindings in `comboToTcell`, do NOT strip `ModCtrl` — the registered modifier must match what tcell delivers, otherwise `matchKey()` will fail silently. Ctrl+punctuation control chars (space, backtick, `/`, `\`, `]`, `^`, `_`) have no `KeyCtrl*` constant in v3 — they arrive as `KeyRune` + `ModCtrl` + a printable string (the exact string differs between legacy terminals and kitty-protocol terminals). `foldCtrlEvent()` in `internal/ui/root.go` folds both encodings to the canonical registered form: ctrl+space and ctrl+backtick → `KeyNUL`+`ModCtrl`, ctrl+/ → `KeyUS`+`ModCtrl`. New ctrl+punctuation bindings need a fold entry there.

**Ctrl+Backtick (`` ctrl+` ``):** On legacy terminals Ctrl+` sends NUL (0x00), same as Ctrl+Space — they are indistinguishable, and both fold to `KeyNUL`+`ModCtrl`. Kitty-protocol terminals (Ghostty, Kitty, WezTerm) do report them distinctly under tcell v3, but ttt currently folds both to the same canonical key, so they remain one binding. `terminal.toggle` is bound to `ctrl+t` by default (with `alt+t` for `terminal.fullscreen`); `ctrl+backtick` is currently unbound.

**Force keys:** Bindings for commands in the `config.ForceKeyCommands` map (`internal/config/keybindings.go`, registered via `root.AddForceKey()` in `internal/app/commands.go`) are checked even when a `RawKeyConsumer` (like the integrated terminal) has focus. `terminal.toggle` must remain a force key.

**Keymap source of truth:** `DefaultKeybindings()` in `internal/config/keybindings.go` is canonical. `config/keybindings.json` is a generated mirror of it (for docs and as a user reference) — when changing defaults, update both, plus the README and docs-web keybinding tables.

### LSP Integration

Language server support lives in `internal/lsp/`. Servers are configured per-language in `~/.config/ttt/extensions.json`. The LSP client uses JSON-RPC 2.0 over stdio with Content-Length framing — no external dependencies.

- `jsonrpc.go` — codec (send/receive with Content-Length framing)
- `protocol.go` — minimal LSP type definitions (initialize, document sync, completions, signature help)
- `client.go` — LSP client with async read loop and request/response channel matching
- `manager.go` — one client per language, lazy-started on first use
- `extensions.go` — config loading from `extensions.json`
- `internal/app/lsp_convert.go` — bridge converting `lsp.CompletionItem` → `ui.CompletionItem`

Async completions and signature help use the same `PostEvent(EventInterrupt)` pattern as git blame. Document sync is full-document (not incremental). Auto-completion triggers on every text change with a configurable debounce timer (`autocomplete.debounce` in settings.json, default 150ms). Signature help triggers on `(` and `,` characters, dismissed on `)`.

### Plugin Widget API

Lua plugins render UI in sidebar panels, bottom-panel tabs, drawers, and editor tabs via a `PanelProxy` (`p`) passed to their `render` callback. Implementation lives in `internal/plugin/lua_panel.go` (Lua bindings), `internal/plugin/widget_desc.go` (descriptor struct), and `internal/plugin/widget_builder.go` (Go widget construction). Underlying widget types are in `internal/widgets/`.

**Widget methods** (called as `p:method(args)`):

| Method | Lua fields | Description |
|---|---|---|
| `p:label(text)` or `p:label({...})` | `text`, `style`, `badge`, `width`, borders | Static text line. `style` is a named style (see below). `border`/`border_top`/`border_bottom`/`border_left`/`border_right` draw borders. Supports box model. |
| `p:title(text)` or `p:title({...})` | `text`, `badge`, `menu`, `on_menu(command)`, `icon`, `padded` | Bold section heading with optional right-aligned badge and dropdown menu. `menu` is `{label, command, separator, checked}` tables; optional boolean `checked` reserves and controls a check indicator. `icon` overrides the dropdown button (default `⋮`). Supports box model. |
| `p:tree({...})` | `items`, `indent` (default 2), `on_select`, `on_expand`, `on_command`, `node_menu`, `key_commands`, `truncate_left` | Expandable tree view. Items are `{id, label, icon, badge, muted, expandable, expanded, children}` tables. `key_commands` maps single chars to commands via `on_command`. `truncate_left` truncates overflowing labels from the left (`…tail`) so the end stays visible. |
| `p:list({...})` | `items`, `on_select`, `on_command`, `node_menu`, `key_commands`, `truncate_left` | Flat list (backed by TreeWidget, no indentation). `truncate_left` keeps label tails visible on overflow. |
| `p:button({...})` | `label`, `on_click` | Clickable button. Label is immutable after creation (accelerator parsing). |
| `p:checkbox({...})` | `label`, `checked`, `style`, `on_change(checked)` | Boolean toggle. Renders `[x]`/`[ ]` with focus styling on brackets. Supports box model. |
| `p:input({...})` | `placeholder`, `prefix`, `clear_on_submit`, `on_change(text)`, `on_submit(text)` | Text input field. `clear_on_submit` (bool) clears text after submit. |
| `p:vstack({...})` | `render(child_panel)`, `gap` | Vertical stack container. The `render` function receives a child panel proxy to emit nested widgets. |
| `p:keyvalue({{key,value}, ...})` | array of `{key, value}` tables | Key-value list. The argument table IS the entries array (not an `entries` field); box model fields go on the same table. |
| `p:hstack({...})` | `render(child_panel)`, `gap`, `height` | Horizontal stack container. First child grows to fill available space, remaining children get fixed width. |
| `p:scrollview({...})` | `render(child_panel)` | Scrollable container. Wraps children with mouse wheel scrolling and scrollbar when content overflows. |
| `p:box({...})` | `render(child_panel)`, `border` (+ per-side), `height` | Container with optional border and fixed height. Children via `render` callback. |
| `p:divider()` | (none) | Horizontal divider line. Single-line separator, no configuration. |
| `p:dropdown({...})` | `label`, `entries`, `on_menu(command)` | Dropdown menu button. `entries` are `{label, command, separator, checked}` tables; optional boolean `checked` reserves and controls a check indicator. |
| `p:progress({...})` | `value` (0–1), `style`, `char` (default `▄`) | Horizontal progress bar. |
| `p:table({...})` | `columns` (`{label, width, align}`), `rows` (arrays of strings), `on_select(row_idx)`, `on_command(cmd, row_idx)`, `node_menu`, `key_commands` | Data table with headers and row selection. Row indices are 1-based. |
| `p:markdown(text)` or `p:markdown({...})` | `text` | Rendered markdown with selection/copy, auto-wrapped in a scrollview. Wraps at `markdown.wrapWidth` (default 80). |

All menu-entry tables (`actions`, `menu`, `entries`, and `node_menu`) accept `label`, `command`, `separator`, and optional boolean `checked`. Omitting `checked` keeps the menu indicator-free; `false` shows an unchecked slot and `true` shows a check.

**Raw cell API** (low-level drawing; can be mixed with widgets — raw cells draw directly on the surface, widgets stack from the top over it):

- `p:size()` — returns `width, height`
- `p:cell(x, y, char, style)` — set a single cell
- `p:text(x, y, text, style)` — draw a string
- `p:clear(x, y, w, h)` — clear a rectangle
- `p:redraw()` — request a redraw from the event loop

**Box model:** `margin_top`, `margin_bottom`, `margin_left`, `margin_right`, `padding_top`, `padding_bottom`, `padding_left`, `padding_right` — parsed via `parseBoxModel()` and applied via `applyBoxModel()`. Supported on all widgets except `divider`.

**Named styles** available for `style` fields: `default`, `muted`, `border`, `success`, `danger`, `warning`, `selected`, `item`, `line`, `input`, `bold`, `italic`, `code`, `syntax_comment`, `syntax_string`, `syntax_keyword`, `syntax_number`, `syntax_operator`, `syntax_function`, `syntax_type`, `syntax_builtin`, `syntax_variable`, `syntax_tag`, `syntax_attribute`. These map to `term.Style*` constants via `StyleByName()` in `styles.go`.

### Status Bar Segment API

The status bar uses a segment-based model (`view.StatusBar` with `StatusSegment`). Both core and plugins contribute segments. Each segment has an `ID`, `Side` (`"left"` or `"right"`), `Priority` (lower = closer to the edge), `Text`, optional `Style`, and optional `OnClick` handler.

Core segments use priorities 100–500 (branch=100, blame=200 on left; position=100, indent=200, encoding=300, eol=400, language=500 on right). Plugin segments default to priority 1000; lower values (e.g., 10) appear before core segments.

**Plugin Lua API:**
- `ttt.set_status_item(side, id, text, opts)` — add or update a status bar segment. `side` is `"left"` or `"right"`. `id` is scoped to the plugin (prefixed with `pluginName:`). `opts` is an optional table with `priority` (number, default 1000) and `on_click` (function).
- `ttt.remove_status_item(id)` — remove a segment by ID.

**Command execution:**
- `ttt.exec_command(id)` — execute any registered command by ID (e.g., `"editor.undo"`, `"file.save"`). Returns `true` if the command was found and executed, `false` otherwise. Requires `commands` permission.

These callbacks are only available after `WirePlugin` — call them from command handlers or event callbacks, not at plugin init time.

### Testing

The project has four levels of testing:

**Unit tests** (`internal/*/`) — Standard Go tests for individual packages. Most core algorithms are testable without a terminal dependency; `core/highlight` is the documented presentation-coupled violation scheduled for cleanup. Run with `go test ./internal/core/buffer/` or `make test` for all.

**E2E tests** (`tests/e2e/`) — Go tests that wire up the full `App` with a `term.SimScreen` (an in-memory `tcell.Screen`). The `testHarness` (`harness_test.go`) creates a temp directory with sample files, builds the complete app (config, commands, keybindings, renderer), and provides helpers: `pressKey()`, `pressRune()`, `click()`, `exec()`, `screenText()`, `assertContains()`. The watcher-aware `waitForFileChange()` helper blocks on `PollEvent` to receive real fsnotify events and dispatches them through the reconciliation path. These tests run single-threaded (no event loop goroutine) — the test drives events and redraws manually.

**Functional tests** (`tests/functional/`) — JavaScript tests using vitest that drive the real compiled `bin/ttt` binary via the `--exec` debug harness. The `tui.js` wrapper accumulates commands (type, press, exec, snapshot) and runs them in a single batch via `execFileSync`. No external dependencies beyond vitest. Run with `cd tests/functional && pnpm test`. The binary must be built first (`make build`).

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

Do not duplicate the same assertion at every layer. During implementation, run focused tests for the affected contract. CI remains the broad regression gate before merge.

### Debug harness (`--exec`, `--plugin`, `--size`, `--debug`, `--listen`)

**USE THIS FOR DEBUGGING AND TESTING.** The editor has a built-in scripted interaction system that is faster than TUI tests and gives you direct access to internal state — reach for it before investigating UI bugs manually.

**`--exec "commands"`** — Execute semicolon-separated commands after startup. Run the real binary, interact with it, capture state, and exit — all in one command:

```bash
bin/ttt --size 120x40 --exec "wait-for Explore; screenshot /tmp/screen.txt; debug /tmp/state.json; quit"
cat /tmp/screen.txt   # see what's rendered
cat /tmp/state.json   # see full widget tree, focus, selection, panels
```

Supported commands:
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
- `wait-for TEXT [timeout=MS]` — wait until text appears on the actual visible screen; defaults to a bounded 5000ms timeout. Quote text to preserve surrounding whitespace or escapes.
- `panel ID` — show and focus a bottom panel by ID
- `quit` / `shutdown` — exit the editor

Scripted input and main-thread commands are acknowledged after the event loop handles and redraws them, so following actions observe completed visible state. Invalid actions, missing commands/panels, capture failures, and wait timeouts stop the script: CLI `--exec` reports the error on stderr and exits nonzero; `POST /exec` returns a non-2xx response with the same detail.

**`--listen`** — Start an HTTP command server on `127.0.0.1:4242` (loopback-only — never exposed off the local machine). `POST /exec` runs the same script format as `--exec`, synchronously, against an **already-running** editor — for capturing a repro at the exact moment it happens instead of scripting it in advance:

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
- **Comments**: do not add comments to code unless they are critical — e.g. a non-obvious architectural constraint that would cause bugs or misuse if missed (see the `textwidth`/fullwidth-rune notes above for the bar to clear). Do not explain WHAT the code does; well-named identifiers already do that.

### Post-implementation review

After a feature is implemented and tests pass, review all changes for cleanup: dead code, unnecessary complexity, naming inconsistencies, or missing edge cases. Fix anything related to the feature in the same PR. If you spot something unrelated that needs attention, create a GitHub issue for it instead of fixing it in the current PR.

### Dependencies

Key external dependencies beyond the Go standard library:

- `github.com/gdamore/tcell/v3` — terminal rendering
- `github.com/aymanbagabas/go-pty` — PTY management for the integrated terminal
- `github.com/eugenioenko/vt10x` — VT escape sequence parsing for the integrated terminal (fork of `hinshun/vt10x`)
- `github.com/alecthomas/chroma/v2` — syntax highlighting lexers
- `github.com/yuin/gopher-lua` — Lua plugin engine
- `github.com/yuin/goldmark` — Markdown rendering
- `github.com/fsnotify/fsnotify` — file watching
- `github.com/clipperhouse/displaywidth` — terminal column width measurement
