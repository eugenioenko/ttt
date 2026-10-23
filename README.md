# TTT Editor: Terminal Text Tool

The IDE that lives in your terminal. A TUI that feels like GUI. A real alternative to VS Code, Zed, and Sublime that happens to run in your terminal. Single Go binary, zero config.

![TTT Demo](docs-web/public/demo/demo.gif)

Full documentation lives in [`docs-web/src/content/docs/`](docs-web/src/content/docs/) (published at [tttedit.dev](https://tttedit.dev)).

## Installation

### Prerequisites

- [Git](https://git-scm.com/) — required for source control features
- [ripgrep](https://github.com/BurntSushi/ripgrep) (`rg`) — required for workspace search

### Quick Install MacOS (brew)
```sh
brew tap eugenioenko/ttt
brew trust eugenioenko/ttt
brew install ttt
```

### Quick Install Linux, macOS, FreeBSD, & Android (Termux)
```sh
curl -sSfL https://raw.githubusercontent.com/eugenioenko/ttt/main/install.sh | sh
```

The script downloads the latest release binary for your OS/architecture and installs it to `~/.local/bin` on Linux, `/usr/local/bin` on macOS and FreeBSD, or `$PREFIX/bin` in Android Termux (arm64). To install elsewhere:

```sh
curl -sSfL https://raw.githubusercontent.com/eugenioenko/ttt/main/install.sh | INSTALL_DIR="$HOME/bin" sh
```

Set `INSTALL_DIR` on `sh` so the installer receives it.

### [Arch Linux (AUR)](https://aur.archlinux.org/packages/ttt)

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

### Quick Install Windows (scoop)
```sh
scoop install ttt
```

### Go Install

Requires [Go](https://go.dev/) 1.18 or newer:

```sh
go install github.com/eugenioenko/ttt/cmd/ttt@latest
```

This installs the `ttt` binary to your `$GOPATH/bin` (or `$HOME/go/bin` by default). Make sure that directory is in your `PATH`.

### Download Binary

Pre-built binaries for Linux, macOS, FreeBSD, Android (arm64), and Windows are available on the [Releases](https://github.com/eugenioenko/ttt/releases) page. Windows downloads include `ttt-windows-amd64.exe` for Intel/AMD PCs and `ttt-windows-arm64.exe` for ARM PCs. Download the one for your platform, make it executable, and put it in your `PATH`.

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

### Herdr

TTT is available as a [Herdr](https://herdr.dev) plugin, so you can open it as a pane inside your Herdr terminal workspace.

```sh
herdr plugin install eugenioenko/ttt/herdr-plugin
```

Once installed, bind it to a key (e.g. `Ctrl+b e`) to launch TTT in the active worktree with a single keystroke. See the [Herdr plugin README](herdr-plugin/README.md) for setup and configuration details.

## Features

### Editor

- **Syntax highlighting** powered by [textmate-go](https://github.com/eugenioenko/textmate-go), our pure Go port of VS Code's TextMate engine: 124 languages, with multi-line constructs handled correctly
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
ttt https://github.com/owner/repo/pull/123 # opens pull request review
git diff | ttt                  # opens piped output in an unsaved tab (or: ttt -)
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
- **Explorer: Reveal Active File** expands the tree to the active file and selects it; set `explorer.autoReveal` to do it on every tab switch
- File-type icons, colored by the theme (glyph mappings from [nvim-web-devicons](https://github.com/nvim-tree/nvim-web-devicons)). They need a [Nerd Font](https://www.nerdfonts.com) in your terminal, so they default off; turn them on with `"appearance": { "icons": "nerd-font" }` or **Options > Font Icons**

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

Working-tree files and files under expanded commits can be shown as a compact directory **Tree** or a full-path **List** (the default). The choice persists in `git.fileView`. File-type icons sit between the status letter and the file name; like the Explorer's, they follow the shared `appearance.icons` setting, need a Nerd Font, and default off — **Options > Font Icons** toggles them. Changes, commit details, and Explorer expose safe **Expand All** and **Collapse All** actions in their relevant menus.

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
- Working-tree changes open in the file's own editor tab in diff mode, so you can edit and save there; the diff updates as you type. **Git: Toggle Inline Diff** turns diff mode on or off for any tracked file
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

### Command Palette

- **Ctrl+P** — opens the command palette with all available commands
- **Ctrl+K P** — opens quick file open (searches all files across workspace folders)
- Type `>` in quick-open mode to switch to command mode; delete the `>` to switch back
- Type `?` to browse searchable help for panels, navigation, and key chords
- Menu shortcuts resolve dynamically from your keybindings

### Integrated Terminal

Built-in terminal emulator. Press Ctrl+T to toggle the terminal panel, or Alt+T for fullscreen.

- **Ctrl+K T** to spawn a new terminal tab; multiple tabs with a vertical inner tab bar on the left edge
- Full VT escape sequence support via [`eugenioenko/xterm-go`](https://github.com/eugenioenko/xterm-go) (a maintained fork of [`gitpod-io/xterm-go`](https://github.com/gitpod-io/xterm-go)) and PTY management via [`aymanbagabas/go-pty`](https://github.com/aymanbagabas/go-pty)
- True color (24-bit) and 256-color rendering
- When the terminal is focused, all keys go to the PTY except force keys (Ctrl+T, Alt+T, Ctrl+Q, Ctrl+P, Ctrl+K P, Ctrl+B, F6)
- Terminal shell and scrollback are configurable in `settings.json`; ANSI colors are theme-configurable via the `terminal` field in `theme.json`

See [Integrated Terminal](docs-web/src/content/docs/guides/terminal.md).

### LSP (Language Server Protocol)

TTT has built-in LSP support for language-aware editing: autocomplete, signature help, go to definition/implementation/type definition, find references, rename symbol, hover, document/selection formatting, format-on-save, and diagnostics.

Language servers are configured via plugins — install the LSP plugin for your language and the corresponding server binary from the Plugins panel or command palette.

You can also add custom servers manually in `~/.config/ttt/settings.json`. To disable LSP entirely: `"lsp": { "enabled": false }`.

The language segment in the status bar shows server state for the current file: `◉` connected, `◌` starting, `⚠` failed. Click it to open the OUTPUT panel, where each server logs its startup command and stderr under `lsp:<server>`.

See the [LSP guide](docs-web/src/content/docs/guides/lsp.md) and [LSP feature reference](docs-web/src/content/docs/reference/lsp-features.md) for keybindings, configuration, and per-feature detail.


### Theming

TTT supports fully customizable themes via JSON files. Every color is themeable — syntax highlighting, diff backgrounds, sidebar, tabs, status bar, terminal ANSI colors, borders, and semantic colors (success, danger, warning).

Built-in themes ship with the binary, and [extra themes](docs-web/src/content/docs/guides/extra-themes.md) are available for download from the repo.

Switch themes via **View > Switch Theme** (or the command palette) for a live preview. To create a custom theme, copy a built-in file to `~/.config/ttt/themes/` and set `"theme"` in `settings.json`. To use your terminal's native colors, set foreground/background to empty strings in your theme file.

See [Themes](docs-web/src/content/docs/guides/themes.md).

### Plugins

TTT supports Lua plugins that add sidebar panels, bottom panel tabs, commands, and keybindings. Plugins run in a sandboxed Lua VM with a permission system — users approve each plugin's capabilities on first load.

**Editor emulation** — prefer modal or Emacs-style editing? Install [Vim Mode](https://github.com/eugenioenko/ttt-vim) or [Emacs Mode](https://github.com/eugenioenko/ttt-emacs) from the **Plugins** sidebar tab. Both require ttt 1.1.0 or newer.

**Installing** — open the **Plugins** sidebar tab to browse and install from the community registry, or use **Plugins: Install from URL** to install from any git repository. Browse the available utilities, formatters, and language servers at [ttt-plugins](https://github.com/eugenioenko/ttt-plugins).

**Managing** — click an installed plugin to enable/disable it; **↑** to update, **×** to uninstall, **Plugins: Reload** for live reload during development. To disable the plugin system entirely: `"plugins": { "enabled": false }`.

**Creating** — plugins are Lua scripts with a `plugin.ttt.json` manifest. See the [Plugin Authoring Guide](docs-web/src/content/docs/guides/plugin-authoring.md).

See [Plugins](docs-web/src/content/docs/guides/plugins.md).

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

To run the pinned LSP tests independently in GitHub Actions, open **Actions → LSP Tests → Run workflow**, select a branch, and choose `all`, `c`, `go`, or `typescript`. The same workflow runs in CI. With the GitHub CLI:

```sh
gh workflow run lsp-tests.yml --ref main -f language=go
```

The **LSP Latest Canary** workflow can also be triggered manually to test the broader set of servers at their latest versions.

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

See [ARCHITECTURE.md](ARCHITECTURE.md) for the package structure, dependency boundaries, and architecture convergence plan.

## Contributing

Bug reports, feature requests, and pull requests are all welcome — open an [issue](https://github.com/eugenioenko/ttt/issues) or a PR. See [CONTRIBUTING.md](CONTRIBUTING.md) before opening one.

## Contributors & Acknowledgments

TTT is better because of the people who took the time to try it, report bugs, request features, and contribute code. Thank you. This project grows because of you. 🙏

<p>
  <a href="https://github.com/eugenioenko"><img src="https://avatars.githubusercontent.com/u/17834212?v=4&amp;s=96" width="48" height="48" alt="@eugenioenko" title="@eugenioenko" /></a>
  <a href="https://github.com/arimxyer"><img src="https://avatars.githubusercontent.com/u/37885347?v=4&amp;s=96" width="48" height="48" alt="@arimxyer" title="@arimxyer" /></a>
  <a href="https://github.com/SimonOcampo1"><img src="https://avatars.githubusercontent.com/u/124914405?v=4&amp;s=96" width="48" height="48" alt="@SimonOcampo1" title="@SimonOcampo1" /></a>
  <a href="https://github.com/tenox7"><img src="https://avatars.githubusercontent.com/u/8151602?v=4&amp;s=96" width="48" height="48" alt="@tenox7" title="@tenox7" /></a>
  <a href="https://github.com/pirate-boop"><img src="https://avatars.githubusercontent.com/u/241358583?v=4&amp;s=96" width="48" height="48" alt="@pirate-boop" title="@pirate-boop" /></a>
  <a href="https://github.com/bet4it"><img src="https://avatars.githubusercontent.com/u/16643669?v=4&amp;s=96" width="48" height="48" alt="@bet4it" title="@bet4it" /></a>
  <a href="https://github.com/Detoy"><img src="https://avatars.githubusercontent.com/u/5773665?v=4&amp;s=96" width="48" height="48" alt="@Detoy" title="@Detoy" /></a>
  <a href="https://github.com/DBinK"><img src="https://avatars.githubusercontent.com/u/21201676?v=4&amp;s=96" width="48" height="48" alt="@DBinK" title="@DBinK" /></a>
  <a href="https://github.com/risixdzn"><img src="https://avatars.githubusercontent.com/u/102877738?v=4&amp;s=96" width="48" height="48" alt="@risixdzn" title="@risixdzn" /></a>
  <a href="https://github.com/LetMeSleep8h"><img src="https://avatars.githubusercontent.com/u/183817470?v=4&amp;s=96" width="48" height="48" alt="@LetMeSleep8h" title="@LetMeSleep8h" /></a>
  <a href="https://github.com/austinm911"><img src="https://avatars.githubusercontent.com/u/31991302?v=4&amp;s=96" width="48" height="48" alt="@austinm911" title="@austinm911" /></a>
  <a href="https://github.com/sjdonado"><img src="https://avatars.githubusercontent.com/u/27580836?v=4&amp;s=96" width="48" height="48" alt="@sjdonado" title="@sjdonado" /></a>
  <a href="https://github.com/mshafir"><img src="https://avatars.githubusercontent.com/u/1139669?v=4&amp;s=96" width="48" height="48" alt="@mshafir" title="@mshafir" /></a>
  <a href="https://github.com/w3lld1"><img src="https://avatars.githubusercontent.com/u/42353747?v=4&amp;s=96" width="48" height="48" alt="@w3lld1" title="@w3lld1" /></a>
  <a href="https://github.com/omar3903"><img src="https://avatars.githubusercontent.com/u/22183080?v=4&amp;s=96" width="48" height="48" alt="@omar3903" title="@omar3903" /></a>
  <a href="https://github.com/RhenCloud"><img src="https://avatars.githubusercontent.com/u/111980217?v=4&amp;s=96" width="48" height="48" alt="@RhenCloud" title="@RhenCloud" /></a>
  <a href="https://github.com/RonanCodes"><img src="https://avatars.githubusercontent.com/u/6930753?v=4&amp;s=96" width="48" height="48" alt="@RonanCodes" title="@RonanCodes" /></a>
  <a href="https://github.com/choephix"><img src="https://avatars.githubusercontent.com/u/962412?v=4&amp;s=96" width="48" height="48" alt="@choephix" title="@choephix" /></a>
  <a href="https://github.com/O1sumitkumar"><img src="https://avatars.githubusercontent.com/u/109774891?v=4&amp;s=96" width="48" height="48" alt="@O1sumitkumar" title="@O1sumitkumar" /></a>
  <a href="https://github.com/vikas-0"><img src="https://avatars.githubusercontent.com/u/15345994?v=4&amp;s=96" width="48" height="48" alt="@vikas-0" title="@vikas-0" /></a>
  <a href="https://github.com/YodHeVauHe"><img src="https://avatars.githubusercontent.com/u/43342618?v=4&amp;s=96" width="48" height="48" alt="@YodHeVauHe" title="@YodHeVauHe" /></a>
  <a href="https://github.com/anyingiit"><img src="https://avatars.githubusercontent.com/u/49945850?v=4&amp;s=96" width="48" height="48" alt="@anyingiit" title="@anyingiit" /></a>
  <a href="https://github.com/Dominiquini"><img src="https://avatars.githubusercontent.com/u/1180808?v=4&amp;s=96" width="48" height="48" alt="@Dominiquini" title="@Dominiquini" /></a>
  <a href="https://github.com/jetpax"><img src="https://avatars.githubusercontent.com/u/6566148?v=4&amp;s=96" width="48" height="48" alt="@jetpax" title="@jetpax" /></a>
  <a href="https://github.com/egorse"><img src="https://avatars.githubusercontent.com/u/1152410?v=4&amp;s=96" width="48" height="48" alt="@egorse" title="@egorse" /></a>
  <a href="https://github.com/forthrin"><img src="https://avatars.githubusercontent.com/u/1167421?v=4&amp;s=96" width="48" height="48" alt="@forthrin" title="@forthrin" /></a>
  <a href="https://github.com/lozzo"><img src="https://avatars.githubusercontent.com/u/8793008?v=4&amp;s=96" width="48" height="48" alt="@lozzo" title="@lozzo" /></a>
  <a href="https://github.com/JohnWJ-co"><img src="https://avatars.githubusercontent.com/u/57254767?v=4&amp;s=96" width="48" height="48" alt="@JohnWJ-co" title="@JohnWJ-co" /></a>
  <a href="https://github.com/hassanabdulqayyum"><img src="https://avatars.githubusercontent.com/u/13608799?v=4&amp;s=96" width="48" height="48" alt="@hassanabdulqayyum" title="@hassanabdulqayyum" /></a>
  <a href="https://github.com/atreus44"><img src="https://avatars.githubusercontent.com/u/222225326?v=4&amp;s=96" width="48" height="48" alt="@atreus44" title="@atreus44" /></a>
  <a href="https://github.com/z-hunter"><img src="https://avatars.githubusercontent.com/u/3431923?v=4&amp;s=96" width="48" height="48" alt="@z-hunter" title="@z-hunter" /></a>
  <a href="https://github.com/LeTuR"><img src="https://avatars.githubusercontent.com/u/38325495?v=4&amp;s=96" width="48" height="48" alt="@LeTuR" title="@LeTuR" /></a>
  <a href="https://github.com/aaronsb"><img src="https://avatars.githubusercontent.com/u/2738471?v=4&amp;s=96" width="48" height="48" alt="@aaronsb" title="@aaronsb" /></a>
  <a href="https://github.com/rishubil"><img src="https://avatars.githubusercontent.com/u/382014?v=4&amp;s=96" width="48" height="48" alt="@rishubil" title="@rishubil" /></a>
  <a href="https://github.com/leandrocm86"><img src="https://avatars.githubusercontent.com/u/30850489?v=4&amp;s=96" width="48" height="48" alt="@leandrocm86" title="@leandrocm86" /></a>
  <a href="https://github.com/anderlli0053"><img src="https://avatars.githubusercontent.com/u/22171451?v=4&amp;s=96" width="48" height="48" alt="@anderlli0053" title="@anderlli0053" /></a>
  <a href="https://github.com/lecheel"><img src="https://avatars.githubusercontent.com/u/148137?v=4&amp;s=96" width="48" height="48" alt="@lecheel" title="@lecheel" /></a>
  <a href="https://github.com/michalekmatej"><img src="https://avatars.githubusercontent.com/u/42139665?v=4&amp;s=96" width="48" height="48" alt="@michalekmatej" title="@michalekmatej" /></a>
  <a href="https://github.com/micuit-cuit"><img src="https://avatars.githubusercontent.com/u/89157073?v=4&amp;s=96" width="48" height="48" alt="@micuit-cuit" title="@micuit-cuit" /></a>
  <a href="https://github.com/antonantal"><img src="https://avatars.githubusercontent.com/u/10272353?v=4&amp;s=96" width="48" height="48" alt="@antonantal" title="@antonantal" /></a>
  <a href="https://github.com/HarshRajSinghania"><img src="https://avatars.githubusercontent.com/u/40535627?v=4&amp;s=96" width="48" height="48" alt="@HarshRajSinghania" title="@HarshRajSinghania" /></a>
  <a href="https://github.com/xircon"><img src="https://avatars.githubusercontent.com/u/1316691?v=4&amp;s=96" width="48" height="48" alt="@xircon" title="@xircon" /></a>
  <a href="https://github.com/hazymoon22"><img src="https://avatars.githubusercontent.com/u/53938840?v=4&amp;s=96" width="48" height="48" alt="@hazymoon22" title="@hazymoon22" /></a>
  <a href="https://github.com/aahoughton"><img src="https://avatars.githubusercontent.com/u/502696?v=4&amp;s=96" width="48" height="48" alt="@aahoughton" title="@aahoughton" /></a>
  <a href="https://github.com/AnushreeKasturi"><img src="https://avatars.githubusercontent.com/u/219416439?v=4&amp;s=96" width="48" height="48" alt="@AnushreeKasturi" title="@AnushreeKasturi" /></a>
  <a href="https://github.com/EuanTop"><img src="https://avatars.githubusercontent.com/u/194128472?v=4&amp;s=96" width="48" height="48" alt="@EuanTop" title="@EuanTop" /></a>
  <a href="https://github.com/rafjaf"><img src="https://avatars.githubusercontent.com/u/2151998?v=4&amp;s=96" width="48" height="48" alt="@rafjaf" title="@rafjaf" /></a>
  <a href="https://github.com/sercanatalik"><img src="https://avatars.githubusercontent.com/u/6832079?v=4&amp;s=96" width="48" height="48" alt="@sercanatalik" title="@sercanatalik" /></a>
  <a href="https://github.com/jayhemnani9910"><img src="https://avatars.githubusercontent.com/u/193022578?v=4&amp;s=96" width="48" height="48" alt="@jayhemnani9910" title="@jayhemnani9910" /></a>
  <a href="https://github.com/kooler"><img src="https://avatars.githubusercontent.com/u/714009?v=4&amp;s=96" width="48" height="48" alt="@kooler" title="@kooler" /></a>
  <a href="https://github.com/cestercian"><img src="https://avatars.githubusercontent.com/u/183791452?v=4&amp;s=96" width="48" height="48" alt="@cestercian" title="@cestercian" /></a>
  <a href="https://github.com/ibraah007"><img src="https://avatars.githubusercontent.com/u/229618553?v=4&amp;s=96" width="48" height="48" alt="@ibraah007" title="@ibraah007" /></a>
  <a href="https://github.com/gondsman"><img src="https://avatars.githubusercontent.com/u/45125414?v=4&amp;s=96" width="48" height="48" alt="@gondsman" title="@gondsman" /></a>
  <a href="https://github.com/Yami066"><img src="https://avatars.githubusercontent.com/u/124346195?v=4&amp;s=96" width="48" height="48" alt="@Yami066" title="@Yami066" /></a>
  <a href="https://github.com/KeuqS"><img src="https://avatars.githubusercontent.com/u/50778230?v=4&amp;s=96" width="48" height="48" alt="@KeuqS" title="@KeuqS" /></a>
  <a href="https://github.com/wzd8888"><img src="https://avatars.githubusercontent.com/u/324454403?v=4&amp;s=96" width="48" height="48" alt="@wzd8888" title="@wzd8888" /></a>
  <a href="https://github.com/MattOrmianek"><img src="https://avatars.githubusercontent.com/u/16574489?v=4&amp;s=96" width="48" height="48" alt="@MattOrmianek" title="@MattOrmianek" /></a>
  <a href="https://github.com/mahirhir"><img src="https://avatars.githubusercontent.com/u/246701683?v=4&amp;s=96" width="48" height="48" alt="@mahirhir" title="@mahirhir" /></a>
  <a href="https://github.com/vincentmathis"><img src="https://avatars.githubusercontent.com/u/33002126?v=4&amp;s=96" width="48" height="48" alt="@vincentmathis" title="@vincentmathis" /></a>
  <a href="https://github.com/danielmrdev"><img src="https://avatars.githubusercontent.com/u/17232506?v=4&amp;s=96" width="48" height="48" alt="@danielmrdev" title="@danielmrdev" /></a>
  <a href="https://github.com/ulrichheringer"><img src="https://avatars.githubusercontent.com/u/79576174?v=4&amp;s=96" width="48" height="48" alt="@ulrichheringer" title="@ulrichheringer" /></a>
  <a href="https://github.com/adamsiwiec1"><img src="https://avatars.githubusercontent.com/u/73085927?v=4&amp;s=96" width="48" height="48" alt="@adamsiwiec1" title="@adamsiwiec1" /></a>
  <a href="https://github.com/aazminu"><img src="https://avatars.githubusercontent.com/u/203570326?v=4&amp;s=96" width="48" height="48" alt="@aazminu" title="@aazminu" /></a>
  <a href="https://github.com/Hrushikesh-ramilla"><img src="https://avatars.githubusercontent.com/u/184500519?v=4&amp;s=96" width="48" height="48" alt="@Hrushikesh-ramilla" title="@Hrushikesh-ramilla" /></a>
  <a href="https://github.com/mangeshraut712"><img src="https://avatars.githubusercontent.com/u/95149253?v=4&amp;s=96" width="48" height="48" alt="@mangeshraut712" title="@mangeshraut712" /></a>
  <a href="https://github.com/orshih6"><img src="https://avatars.githubusercontent.com/u/34618353?v=4&amp;s=96" width="48" height="48" alt="@orshih6" title="@orshih6" /></a>
  <a href="https://github.com/bfs2021"><img src="https://avatars.githubusercontent.com/u/101915083?v=4&amp;s=96" width="48" height="48" alt="@bfs2021" title="@bfs2021" /></a>
  <a href="https://github.com/dannyward630"><img src="https://avatars.githubusercontent.com/u/71604164?v=4&amp;s=96" width="48" height="48" alt="@dannyward630" title="@dannyward630" /></a>
  <a href="https://github.com/alytaphoenix"><img src="https://avatars.githubusercontent.com/u/62960711?v=4&amp;s=96" width="48" height="48" alt="@alytaphoenix" title="@alytaphoenix" /></a>
  <a href="https://github.com/montionugera"><img src="https://avatars.githubusercontent.com/u/5601426?v=4&amp;s=96" width="48" height="48" alt="@montionugera" title="@montionugera" /></a>
  <a href="https://github.com/peterheo"><img src="https://avatars.githubusercontent.com/u/104219687?v=4&amp;s=96" width="48" height="48" alt="@peterheo" title="@peterheo" /></a>
  <a href="https://github.com/webdevsamran"><img src="https://avatars.githubusercontent.com/u/121370806?v=4&amp;s=96" width="48" height="48" alt="@webdevsamran" title="@webdevsamran" /></a>
  <a href="https://github.com/premiuslol"><img src="https://avatars.githubusercontent.com/u/80600801?v=4&amp;s=96" width="48" height="48" alt="@premiuslol" title="@premiuslol" /></a>
  <a href="https://github.com/tyza66"><img src="https://avatars.githubusercontent.com/u/83948628?v=4&amp;s=96" width="48" height="48" alt="@tyza66" title="@tyza66" /></a>
  <a href="https://github.com/wangyusheng1985"><img src="https://avatars.githubusercontent.com/u/11242728?v=4&amp;s=96" width="48" height="48" alt="@wangyusheng1985" title="@wangyusheng1985" /></a>
  <a href="https://github.com/briandowns"><img src="https://avatars.githubusercontent.com/u/5714074?v=4&amp;s=96" width="48" height="48" alt="@briandowns" title="@briandowns" /></a>
  <a href="https://github.com/cristianchiriac"><img src="https://avatars.githubusercontent.com/u/80774326?v=4&amp;s=96" width="48" height="48" alt="@cristianchiriac" title="@cristianchiriac" /></a>
  <a href="https://github.com/motodriver"><img src="https://avatars.githubusercontent.com/u/39351936?v=4&amp;s=96" width="48" height="48" alt="@motodriver" title="@motodriver" /></a>
  <a href="https://github.com/bluzername"><img src="https://avatars.githubusercontent.com/u/12148037?v=4&amp;s=96" width="48" height="48" alt="@bluzername" title="@bluzername" /></a>
  <a href="https://github.com/huskyhao"><img src="https://avatars.githubusercontent.com/u/92284833?v=4&amp;s=96" width="48" height="48" alt="@huskyhao" title="@huskyhao" /></a>
  <a href="https://github.com/jay23606"><img src="https://avatars.githubusercontent.com/u/4602056?v=4&amp;s=96" width="48" height="48" alt="@jay23606" title="@jay23606" /></a>
  <a href="https://github.com/LuckTerence"><img src="https://avatars.githubusercontent.com/u/156219145?v=4&amp;s=96" width="48" height="48" alt="@LuckTerence" title="@LuckTerence" /></a>
  <a href="https://github.com/mike-ward"><img src="https://avatars.githubusercontent.com/u/567927?v=4&amp;s=96" width="48" height="48" alt="@mike-ward" title="@mike-ward" /></a>
  <a href="https://github.com/paoValle"><img src="https://avatars.githubusercontent.com/u/11146190?v=4&amp;s=96" width="48" height="48" alt="@paoValle" title="@paoValle" /></a>
  <a href="https://github.com/lien-hung"><img src="https://avatars.githubusercontent.com/u/147837573?v=4&amp;s=96" width="48" height="48" alt="@lien-hung" title="@lien-hung" /></a>
  <a href="https://github.com/Yuren0312"><img src="https://avatars.githubusercontent.com/u/168316868?v=4&amp;s=96" width="48" height="48" alt="@Yuren0312" title="@Yuren0312" /></a>
  <a href="https://github.com/philpax"><img src="https://avatars.githubusercontent.com/u/707827?v=4&amp;s=96" width="48" height="48" alt="@philpax" title="@philpax" /></a>
  <a href="https://github.com/samarthyadavv"><img src="https://avatars.githubusercontent.com/u/62054465?v=4&amp;s=96" width="48" height="48" alt="@samarthyadavv" title="@samarthyadavv" /></a>
  <a href="https://github.com/sduensin"><img src="https://avatars.githubusercontent.com/u/98645?v=4&amp;s=96" width="48" height="48" alt="@sduensin" title="@sduensin" /></a>
  <a href="https://github.com/Catalyze4"><img src="https://avatars.githubusercontent.com/u/194232798?v=4&amp;s=96" width="48" height="48" alt="@Catalyze4" title="@Catalyze4" /></a>
</p>

Want to help? Bug reports, feature requests, and pull requests are all welcome — open an [issue](https://github.com/eugenioenko/ttt/issues) or a PR.

## License

MIT
