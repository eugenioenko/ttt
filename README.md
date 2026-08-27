# TTT Editor: Terminal Text Tool

The IDE that lives in your terminal. Not a simplified terminal editor — a real alternative to VS Code, Zed, and Sublime that happens to run in your terminal. Single Go binary, zero config.

![TTT Demo](docs-web/public/demo/demo.gif)

Full documentation lives in [`docs-web/src/content/docs/`](docs-web/src/content/docs/) (published at [tttedit.dev](https://tttedit.dev)).

## Installation

### Prerequisites

- [Git](https://git-scm.com/) — required for source control features
- [ripgrep](https://github.com/BurntSushi/ripgrep) (`rg`) — required for workspace search

### Quick Install MacOS (brew)
```sh
brew tap eugenioenko/ttt
brew install ttt
```

### Quick Install Linux
```sh
curl -sSfL https://raw.githubusercontent.com/eugenioenko/ttt/main/install.sh | sh
```

The script downloads the latest release binary for your OS/architecture and installs it to `/usr/local/bin`. To install elsewhere:

```sh
INSTALL_DIR=~/.local/bin curl -sSfL https://raw.githubusercontent.com/eugenioenko/ttt/main/install.sh | sh
```

### [Arch Linux (AUR)](https://aur.archlinux.org/packages/ttt)

Thanks to [@Dominiquini](https://github.com/Dominiquini) for maintaining the AUR package.

```sh
yay -S ttt
```

### NixOS

> **Note:** The flake tracks `main`. A future tagged release will ship a pinned flake; until then, install from `main`.

Try it without installing:
```sh
nix run github:eugenioenko/ttt
```

Add to your `flake.nix` inputs:
```nix
{
  inputs.ttt.url = "github:eugenioenko/ttt";
}
```

Then add `inputs.ttt.packages.${system}.default` to your `environment.systemPackages` or home-manager packages.

Thanks to [@pirate-boop](https://github.com/pirate-boop) for keeping the flake's `vendorHash` in sync.

### Go Install

Requires [Go](https://go.dev/) 1.18 or newer:

```sh
go install github.com/eugenioenko/ttt/cmd/ttt@latest
```

This installs the `ttt` binary to your `$GOPATH/bin` (or `$HOME/go/bin` by default). Make sure that directory is in your `PATH`.

### Download Binary

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases](https://github.com/eugenioenko/ttt/releases) page. Download the one for your platform, make it executable, and put it in your `PATH`.

### From Source

> **Note:** Building from source compiles the latest development code, which may include work-in-progress features and could be less stable than official releases.

```sh
git clone https://github.com/eugenioenko/ttt.git
cd ttt
make build
```

This produces an optimized binary at `bin/ttt`. Add it to your `PATH` or copy it somewhere convenient:

```sh
cp bin/ttt ~/.local/bin/
```

## Features

### Editor

- **Syntax highlighting** via [chroma](https://github.com/alecthomas/chroma) — hundreds of languages with automatic detection
- **Bracket matching** with highlighted pairs
- **Find and Replace** — inline find bar (Ctrl+F) with match navigation, replace bar (Ctrl+R) with replace-one and replace-all
- **Go to Line** (Ctrl+G)
- **Selection, copy/cut/paste** (Ctrl+C/X/V) with system clipboard support
- **Undo/redo** (Ctrl+Z/Y) via a command-pattern undo stack
- **Code folding** — toggle folds (Ctrl+K [), fold/unfold all, fold-aware search and navigation
- **Line operations** — toggle line comment, move/duplicate/delete line, join lines, sort lines, split selection into lines
- **Multi-cursor editing** — Ctrl+D to select next occurrence, Ctrl+K L to select all occurrences, Alt+Click to add cursors; typing, backspace, delete, and enter work at all positions simultaneously
- **`.editorconfig` support** and **indent detection** — indentation is picked up automatically per file; manual override via the status bar indent picker
- **Mouse support** — click to position cursor, click tabs, drag sidebar/panel dividers, right-click context menus
- **Auto-completion** — LSP-powered completions with live filtering, debounce, and auto-import support
- **Signature help** — parameter hints shown automatically on `(` and `,`
- **Diagnostics** — inline curly underline squiggles, problems panel, hover popup, and status bar counts
- **Document formatting** — format document, format selection, and format-on-save via LSP or external formatters
- **Git blame** — inline blame info for the current line shown in the status bar (author, relative time, summary)
- **Line numbers** with current-line highlighting
- **Diff-based renderer** for efficient terminal updates (double-buffered cell grid)

### Multi-Folder Workspaces

Open multiple project directories in a single session. Each root appears as a collapsible group in the explorer, search, and changes panels.

```sh
ttt                             # opens the current directory
ttt .                           # also opens the current directory
ttt /path/to/dir                # opens that directory as the workspace
ttt /path/to/file.go            # opens just the file — no workspace folder
ttt /path/to/file.go:42:5       # opens the file at line 42, column 5
ttt /path/to/repo/subdir        # opens that folder; git features (changes,
                                # branch) use the enclosing git repo root
ttt dir1 dir2                   # opens multiple folders as a multi-root workspace
ttt --workspace project.ttt     # loads a saved workspace file
```

Workspace files use the `.ttt` extension and store a list of folders as relative paths:

```json
{
  "folders": [
    { "path": "." },
    { "path": "../other-project" }
  ]
}
```

- **Save Workspace As...** from the File menu to create a workspace file
- **Add Folder to Workspace** and **Remove Folder from Workspace** via the command palette
- The git branch in the status bar switches automatically based on which workspace folder the active file belongs to

See [File Explorer & Workspaces](docs-web/src/content/docs/guides/workspaces.md).

### File Explorer

Multi-root file tree in the sidebar (Ctrl+K E). When multiple folders are open, each root is shown as a collapsible group.

- Directories sorted before files, both alphabetically
- Expand/collapse with Enter or arrow keys
- Right-click context menu: **New File**, **New Folder**, **Rename**, **Delete**
- Sidebar actions button for **Refresh** and **New File**

### Search

Sidebar search panel (Ctrl+K F) powered by [ripgrep](https://github.com/BurntSushi/ripgrep). Results are grouped by file with match counts.

- **Smart-case** matching by default
- **Include/Exclude glob filters** — click the toggle arrow to reveal filter inputs (e.g. `*.go`, `vendor/**`)
- Tab between search, include, and exclude inputs
- Searches across all workspace folders simultaneously
- Click a result to jump to the file and line

See [Search](docs-web/src/content/docs/guides/search.md).

### Git Integration

Changes panel in the sidebar (Ctrl+K C) with full staging workflow.

Working-tree files and files under expanded commits can be shown as a compact directory **Tree** or a full-path **List** (the default). The choice persists in `git.fileView`. Changes, commit details, and Explorer expose safe **Expand All** and **Collapse All** actions in their relevant menus.

**Staging:**
- **Spacebar** — toggle stage/unstage on the selected file
- **`a`** — stage all unstaged files
- **`u`** — unstage all staged files
- **`+` / `-` buttons** on the section headers — stage/unstage all files in that section

**Committing:**
- Inline commit message input below the file list (Tab from the tree to focus it)
- Type a message and press Enter to commit all staged files
- Commit History starts with the 10 most recent commits; activate **Load older commits…** to append bounded pages from the same HEAD snapshot

**Remote operations:**
- **Pull**, **Push**, **Sync** (pull then push) from the sidebar actions button
- Per-repo actions via the group header menu button in multi-root workspaces

**Diff view:**
- Select a changed file to open a split or unified diff with syntax highlighting layered on diff backgrounds
- Set the global view mode, context, wrapping, and high-contrast presentation under **Options**; the Changes panel menu provides the same contextual controls
- Changes-only views show quiet collapsed-context rows that can be expanded in place; full-file context remains available globally or per diff
- Untracked files open directly in the editor

**Multi-root:**
- Changes are grouped by repository, each with its own collapsible Staged/Changes sections and commit input
- File status badges: **M** (modified), **A** (added), **D** (deleted), **R** (renamed), **U** (untracked)

**GitHub pull request review:**

```sh
ttt https://github.com/owner/repo/pull/123      # review a PR
ttt . https://github.com/owner/repo/pull/123    # review a PR with the repo tree open
```

Opens the PR as a review surface — commit-by-commit history detail, a live view of the current changes, and the same unified presentation controls as local diffs.

See [Git Integration](docs-web/src/content/docs/guides/git.md).

### Outline

Symbol tree of the current file in the sidebar. Navigate and jump to definitions. Uses LSP document symbols with a built-in fallback for Go and Markdown. Open it from the command palette.

Thanks to [@tenox7](https://github.com/tenox7) for contributing the Outline feature.

### Command Palette

- **Ctrl+P** — opens the command palette with all available commands
- **Ctrl+K P** — opens quick file open (searches all files across workspace folders)
- Type `>` in quick-open mode to switch to command mode; delete the `>` to switch back
- Type `?` to browse searchable help for panels, navigation, and key chords
- Menu shortcuts resolve dynamically from your keybindings

### Bottom Panel

The bottom panel (Ctrl+K B to toggle) contains the **Terminal**, **Problems**, and **References** tabs.

- **Diagnostics tab** — lists all LSP diagnostics (errors, warnings) grouped by file; click to jump to location
- **References tab** — shows results from Find All References; click to jump to location
- **Terminal tab** — integrated terminal emulator (see below)

### Integrated Terminal

Built-in terminal emulator. Press Ctrl+T to toggle the terminal panel, or Alt+T for fullscreen.

- **Ctrl+K T** to spawn a new terminal tab; multiple tabs with a vertical inner tab bar on the left edge
- Full VT escape sequence support via [`hinshun/vt10x`](https://github.com/hinshun/vt10x) and PTY management via [`creack/pty`](https://github.com/creack/pty)
- True color (24-bit) and 256-color rendering
- When the terminal is focused, all keys go to the PTY except force keys (Ctrl+T, Alt+T, Ctrl+Q, Ctrl+P, Ctrl+K P, Ctrl+B, F6)
- Terminal shell and scrollback are configurable in `settings.json`; ANSI colors are theme-configurable via the `terminal` field in `theme.json`

See [Integrated Terminal](docs-web/src/content/docs/guides/terminal.md).

### LSP (Language Server Protocol)

TTT has built-in LSP support for language-aware editing: autocomplete, signature help, go to definition/implementation/type definition, find references, rename symbol, hover, document/selection formatting, format-on-save, and diagnostics.

Language servers are configured via plugins — install the LSP plugin for your language and the corresponding server binary from the Plugins panel or command palette. Available plugins:

`lsp-go`, `lsp-typescript`, `lsp-python`, `lsp-c`, `lsp-rust`, `lsp-lua`, `lsp-zig`, `lsp-vue`, `lsp-svelte`, `lsp-css`, `lsp-html`, `lsp-json`, `lsp-yaml`, `lsp-bash`, `lsp-docker`, `lsp-tailwindcss`, `lsp-kotlin`, `lsp-java`, `lsp-ruby`, `lsp-dart`, `lsp-elixir`, `lsp-php`, `lsp-terraform`, `lsp-markdown`

You can also add custom servers manually in `~/.config/ttt/settings.json`. To disable LSP entirely: `"lsp": { "enabled": false }`.

The language segment in the status bar shows server state for the current file: `◉` connected, `◌` starting, `⚠` failed. Click it to open the OUTPUT panel, where each server logs its startup command and stderr under `lsp:<server>`.

See the [LSP guide](docs-web/src/content/docs/guides/lsp.md) and [LSP feature reference](docs-web/src/content/docs/reference/lsp-features.md) for keybindings, configuration, and per-feature detail.

### Tabs

Tabs follow a pin-on-reclick model similar to VS Code:

- Opening a file from the explorer or search replaces the current **unpinned** tab
- Clicking on an already-open tab (or opening the same file again) **pins** it
- **Ctrl+W** to close a tab, **Alt+.** / **Alt+,** to switch tabs
- Right-click a tab for **Close**, **Close Others**, **Close All**
- Drag tabs to reorder them

### Theming

TTT supports fully customizable themes via JSON files. Every color is themeable — syntax highlighting, diff backgrounds, sidebar, tabs, status bar, terminal ANSI colors, borders, and semantic colors (success, danger, warning).

10 themes ship in the binary ([`internal/config/themes/`](internal/config/themes/)): Aurora, Bubblegum, Default Dark, Default Light, Hotline, Monokai, One Dark, Solarized Dark, Solarized Light, Virtru Dark. Another 78 [extra themes](docs-web/src/content/docs/guides/extra-themes.md) are available for download from the repo.

Switch themes via **View > Switch Theme** (or the command palette) for a live preview. To create a custom theme, copy a built-in file to `~/.config/ttt/themes/` and set `"theme"` in `settings.json`. To use your terminal's native colors, set foreground/background to empty strings in your theme file.

See [Themes](docs-web/src/content/docs/guides/themes.md).

### Plugins

TTT supports Lua plugins that add sidebar panels, bottom panel tabs, commands, and keybindings. Plugins run in a sandboxed Lua VM with a permission system — users approve each plugin's capabilities on first load.

**Editor emulation** — prefer modal or Emacs-style editing? Install [Vim Mode](https://github.com/eugenioenko/ttt-vim) (Normal/Insert/Visual modes, the operator/motion/text-object grammar, registers, marks, macros, dot-repeat, `:` command line and `/` search) or [Emacs Mode](https://github.com/eugenioenko/ttt-emacs) (prefix keymaps, kill ring, mark and region, incremental search, universal argument, keyboard macros). Both require ttt 1.1.0 or newer and install from the **Plugins** sidebar tab.

**Installing** — open the **Plugins** sidebar tab to browse and install from the community registry, or use **Plugins: Install from URL** to install from any git repository. Community plugins are maintained at [ttt-plugins](https://github.com/eugenioenko/ttt-plugins): general utilities (cheat-sheet, color-picker, docker-manager, git-history, go-test-runner, http-client, json-viewer, markdown-preview, notepad, port-finder, spell, todo-scanner), one-click formatter setup (`formatter-gofmt`, `formatter-prettier`, `formatter-black`, `formatter-rustfmt`, `formatter-stylua`, `formatter-clang-format`, `formatter-shfmt`), and the LSP plugins listed above.

**Managing** — click an installed plugin to enable/disable it; **↑** to update, **×** to uninstall, **Plugins: Reload** for live reload during development. To disable the plugin system entirely: `"plugins": { "enabled": false }`.

**Creating** — plugins are Lua scripts with a `plugin.ttt.json` manifest. See the [Plugin Authoring Guide](docs-web/src/content/docs/guides/plugin-authoring.md).

See [Plugins](docs-web/src/content/docs/guides/plugins.md).

### Menu Bar

File, Edit, Selection, View, and Help menus accessible via the menu bar or keyboard shortcuts. Menus display resolved keybindings next to each command. Navigate between menus with left/right arrow keys.

## Configuration

Config files are loaded from `<exe-dir>/config/` (bundled defaults) or `~/.config/ttt/` (user overrides):

| File | Purpose |
|------|---------|
| [`settings.json`](config/settings.json) | Editor settings (tabSize, wordWrap, theme, lsp, autocomplete, etc.) |
| [`keybindings.json`](config/keybindings.json) | Custom keybindings (VS Code key format) |
| `themes/*.json` | Custom color themes |

Most settings can be edited from a form: **View → Settings**, **Ctrl+K ,**, or **Settings: Open Editor Settings** from the command palette. Changes are held until you press **Apply**; **Cancel** drops them. Settings marked *(restart)* take effect on next launch. LSP settings and external formatters are JSON-only — use **Settings: Open settings.json**.

See [Settings reference](docs-web/src/content/docs/reference/settings.md) for every key, its type, default, and description.

## Keybindings

All keybindings are customizable via [`keybindings.json`](config/keybindings.json). Chord sequences (e.g. `ctrl+k e`) are supported. Open your keybindings file with **Ctrl+K Y**, or press **Ctrl+P** and type `?` to browse shortcuts in-app.

The full default keybinding list is in [`docs-web/src/content/docs/reference/keybindings.md`](docs-web/src/content/docs/reference/keybindings.md).

## Testing

TTT uses deterministic tests at several boundaries, from core data structures to real-binary and live-PTY workflows.

### Unit Tests (Go)

Core algorithms such as buffer operations, cursor math, and undo/redo have focused Go tests. Core packages are presentation-independent; syntax highlighting lives in the presentation-owned `internal/highlight` package.

```sh
make test                            # run all unit tests
go test ./internal/core/buffer/      # test a single package
```

### E2E Tests (Go + SimScreen)

These wire the complete App to an in-memory `term.SimScreen` and exercise editor state, commands, event dispatch, and rendered output without starting a real terminal.

```sh
go test ./tests/e2e/
```

### Functional Tests (vitest + --exec)

These launch the real `ttt` binary via the built-in `--exec` debug harness, run scripted commands in batch, and assert on screenshots and file contents. No external dependencies beyond [vitest](https://vitest.dev/). Coverage includes:

- **File operations** — open, edit, save, Save As, new file, dirty indicator
- **Editing** — undo/redo, select all + overwrite, line delete/move/duplicate, word delete, sort, case transform
- **Unicode** — accented characters, CJK, emoji, stress tests
- **Find & Replace** — search, match navigation, single/all replace, save verification
- **Navigation** — go to line, tab switching, code folding, matching brackets
- **UI panels** — sidebar toggle, terminal toggle, command palette, word wrap
- **Tab management** — multi-tab state isolation, close, unsaved changes dialog
- **Multi-cursor** — add cursor, select occurrences, type with multiple cursors

```sh
cd tests/functional
pnpm install
pnpm test              # run all functional tests
```

The harness acknowledges scripted input and commands after main-thread handling and redraw. Tests should wait only for genuine asynchronous work, using a unique visible post-transition state; raw elapsed waits belong only in tests whose invariant is timing or delayed lifecycle behavior.

### Integration Tests (vitest + tui-use)

Tests that require live PTY interaction or a real external process boundary. Built with [vitest](https://vitest.dev/) and [tui-use](https://github.com/onesuper/tui-use), they cover real language servers, external file changes, settings roundtrips, and bracketed paste.

```sh
cd tests/integration
pnpm install
pnpm test
```

Use the smallest deterministic layer that proves an invariant. Functional tests are a compact real-binary contract, not a mandatory copy of lower-layer coverage. Add a broader test only when it establishes behavior unique to that boundary. CI runs the broad regression lanes on pull requests.

### Chaos Monkey (Fuzz Testing)

A randomized fuzz tester that hammers the editor with thousands of random events — keypresses, mouse clicks, resizes, clipboard operations, and command palette commands — to find panics and crashes that normal testing misses. It runs against an in-memory simulated screen (`term.SimScreen`) so no real terminal is needed.

All chaos targets run in Docker: the random command stream can write files, persist settings, and open browser tabs, so the harness refuses to run unsandboxed (set `CHAOS_ALLOW_HOST=1` to force a host run, e.g. under a debugger).

```sh
# Quick run (50 iterations x 500 events)
make chaos

# Run continuously (crash logs saved to chaos-output/)
make chaos-docker

# Reproduce a specific crash deterministically
CHAOS_REPLAY=chaos-output/crash-<seed>-<iter>.json make chaos-replay
```

Each crash is saved as a JSON report with the random seed and full event log, so any panic can be replayed and debugged deterministically.

## Debug & Testing CLI Flags

TTT includes a built-in scripted interaction system designed for AI agent interactivity and automated debugging. Think of `--exec` as a fast Playwright for the terminal — full click, keyboard, and command simulation with screenshot and state dump capture, all without the overhead of a terminal emulation layer.

| Flag | Description |
|------|-------------|
| `--exec "commands"` | Execute semicolon-separated commands after startup |
| `--listen` | Start an HTTP command server on `127.0.0.1:4242` (`POST /exec` accepts the same script format as `--exec`, against an already-running editor) |
| `--plugin FILE` | Load a Lua plugin file on startup with full permissions |
| `--size WxH` | Force screen dimensions (e.g. `120x40`) for deterministic layout |
| `--debug` | Enable debug mode regardless of config |

The `TTT_CONFIG_DIR` environment variable overrides the config directory (`~/.config/ttt`) entirely — settings, keybindings, themes, and plugins are read from and written to that directory instead. Use it to run scripted sessions isolated from your real configuration. Headless `--exec` sessions also use a process-local clipboard so concurrent automation cannot overwrite the desktop clipboard; interactive sessions, including `--listen`, keep the system clipboard.

### `--exec` Commands

The `--exec` flag accepts a semicolon-separated string of commands that run sequentially after the editor starts. AI agents (like Claude Code) can use this to interact with the editor, inspect UI state, and verify behavior programmatically — no manual interaction needed:

| Command | Description |
|---------|-------------|
| `click X Y` | Simulate a mouse click at screen coordinates |
| `key COMBO` | Simulate a key press (e.g. `key ctrl+p`, `key enter`) |
| `type TEXT` | Type a string of text character by character |
| `exec "Command Name"` | Run a command palette command by title |
| `screenshot PATH` | Save the current screen text to a file |
| `debug PATH` | Save the editor's debug state as JSON to a file |
| `wait MS` | Wait for the given number of milliseconds |
| `wait-for TEXT [timeout=MS]` | Wait until text appears on the visible screen (default timeout: 5000ms) |
| `quit` / `shutdown` | Exit the editor |

Quote `wait-for` text when it contains leading/trailing whitespace or escapes, for example `wait-for "Indexing complete" timeout=10000`. Scripted input and commands are acknowledged only after the main event loop handles and redraws them, so a following `wait-for`, `screenshot`, or `debug` observes their completed visible state. Invalid actions, missing commands/panels, capture failures, and wait timeouts stop the script: CLI `--exec` writes the error to stderr and exits nonzero; `POST /exec` returns a non-2xx response with the same detail.

Example — capture a screenshot and debug state, then quit:

```sh
ttt --size 120x40 --exec "wait-for Explore; screenshot /tmp/screen.txt; debug /tmp/state.json; quit"
```

Example — drive an already-running editor over `--listen` instead of scripting in advance:

```sh
ttt --listen &
curl -X POST --data "type hi; wait-for hi; screenshot /tmp/screen.txt" http://127.0.0.1:4242/exec
curl -X POST --data "shutdown" http://127.0.0.1:4242/exec
```

## Architecture

TTT uses dependency zones rather than a strict linear layer chain:

- **Domain** — editor algorithms and state in `internal/core/*`
- **Services** — Git, GitHub, LSP, terminal, watcher, and workspace integration
- **Presentation kernel** — screen cells, rendering, width, layout, and reusable widgets
- **Product presentation** — editor, diff, search, terminal, panel, and dialog surfaces in `internal/ui`
- **Application** — commands, async lifecycle, cancellation, and service coordination in `internal/app`
- **Plugin host** — Lua runtime, permissions, registry, and widget adapters in `internal/plugin`
- **Platform** — process startup and concrete composition in `cmd/ttt`

See [`ARCHITECTURE.md`](ARCHITECTURE.md) for the current dependency graph, known violations, explicit boundary decisions, package ownership rules, verification strategy, and the incremental convergence plan.

## Contributing

Bug reports, feature requests, and pull requests are all welcome — open an [issue](https://github.com/eugenioenko/ttt/issues) or a PR.

## Contributors & Acknowledgments

TTT is better because of the people who took the time to try it, report bugs, request features, and contribute code. Thank you — this project grows because of you. 🙏

**Code contributions**

- [@tenox7](https://github.com/tenox7) — the **Outline** sidebar panel (LSP document symbols with a built-in Go/Markdown fallback) and **markdown syntax highlighting**.
- [@pirate-boop](https://github.com/pirate-boop) — **NixOS support** end to end: the initial `flake.nix` and ongoing `vendorHash` upkeep.
- [@arimxyer](https://github.com/arimxyer) — re-envisioned and re-designed the **diff & code review experience** (commit history detail, live current changes, unified presentation controls, hierarchical file trees), plus tab drag reordering, orientation help, checked plugin menu entries, deterministic exec automation, and CLI open-at-line support.

**Packaging**

- [@Dominiquini](https://github.com/Dominiquini) — maintaining the [Arch Linux (AUR)](https://aur.archlinux.org/packages/ttt) package.

**Bug reports & feature requests**

- [@jetpax](https://github.com/jetpax) — surfacing the macOS / iTerm2 issues (mouse support, clipboard copy, large-list scrolling, workspace paths).
- [@egorse](https://github.com/egorse) — the search-panel focus fix.
- [@pirate-boop](https://github.com/pirate-boop) — the Cyrillic cursor-offset fix and a number of UX/packaging ideas.

Want to help? Bug reports, feature requests, and pull requests are all welcome — open an [issue](https://github.com/eugenioenko/ttt/issues) or a PR.

## License

MIT
