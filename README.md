# TTT Editor: Terminal Text Tool

The IDE that lives in your terminal. Not a simplified terminal editor — a real alternative to VS Code, Zed, and Sublime that happens to run in your terminal. Single Go binary, zero config.

![TTT Demo](docs-web/public/demo/demo.gif)

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

### [Arch Linux (AUR)](https://aur.archlinux.org/packages/ttt)

Thanks to [@Dominiquini](https://github.com/Dominiquini) for maintaining the AUR package.

```sh
yay -S ttt
```


### NixOS

> **Note:** Always install from a tagged release. The `main` branch is unstable and may contain work-in-progress features.

Try it without installing:
```sh
nix run github:eugenioenko/ttt/v0.3.5
```

Add to your `flake.nix` inputs:
```nix
{
  inputs.ttt.url = "github:eugenioenko/ttt/v0.3.5";
}
```

Then add `inputs.ttt.packages.${system}.default` to your `environment.systemPackages` or home-manager packages.

### Go Install

Requires [Go](https://go.dev/) 1.18 or newer:

```sh
go install github.com/eugenioenko/ttt/cmd/ttt@latest
```

This installs the `ttt` binary to your `$GOPATH/bin` (or `$HOME/go/bin` by default). Make sure that directory is in your `PATH`.

This downloads the latest release binary for your OS/architecture and installs it to `/usr/local/bin`. To install to a different directory:

```sh
INSTALL_DIR=~/.local/bin curl -sSfL https://raw.githubusercontent.com/eugenioenko/ttt/main/install.sh | sh
```

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

- **Syntax highlighting** via [chroma](https://github.com/alecthomas/chroma) — supports hundreds of languages with automatic detection
- **Bracket matching** with highlighted pairs
- **Find and Replace** — inline find bar (Ctrl+F) with match navigation, replace bar (Ctrl+H) with replace-one and replace-all
- **Go to Line** (Ctrl+G)
- **Selection, copy/cut/paste** (Ctrl+C/X/V) with system clipboard support
- **Undo/redo** (Ctrl+Z/Y) via a command-pattern undo stack
- **`.editorconfig` support** — indent size is picked up automatically per file
- **Indent detection** — auto-detects indentation from file content; manual override via the status bar indent picker
- **Multi-cursor editing** — Ctrl+D to select next occurrence, Ctrl+K L to select all occurrences, Alt+Click to add cursors; typing, backspace, delete, and enter work at all positions simultaneously
- **Mouse support** — click to position cursor, click tabs, drag sidebar/panel dividers, right-click context menus
- **Auto-completion** — LSP-powered completions with live filtering, debounce, and auto-import support
- **Signature help** — parameter hints shown automatically on `(` and `,`
- **Diagnostics** — inline curly underline squiggles, problems panel, hover popup, and status bar counts
- **Document formatting** — format document, format selection, and format-on-save via LSP (command palette)
- **Git blame** — inline blame info for the current line shown in the status bar (author, relative time, summary)
- **Line numbers** with current-line highlighting
- **Integrated terminal emulator** via [vt10x](https://github.com/hinshun/vt10x) — multiple tabs with vertical inner tab bar, full VT escape sequence support
- **Bottom panel** — tabbed panel with integrated terminal and problems list
- **Diff-based renderer** for efficient terminal updates (double-buffered cell grid)

### Multi-Folder Workspaces

Open multiple project directories in a single session. Each root appears as a collapsible group in the explorer, search, and changes panels.

```sh
ttt                             # opens the current directory
ttt .                           # also opens the current directory
ttt /path/to/dir                # opens that directory as the workspace
ttt /path/to/file.go            # opens the file; workspace is the git repo root
                                # (falls back to the file's parent dir if not in a repo)
ttt dir1 dir2                   # opens multiple folders as a multi-root workspace
ttt --workspace project.ttt     # loads a saved workspace file

# Review a GitHub pull request
ttt https://github.com/owner/repo/pull/123

# Review a PR with the repo tree open
ttt . https://github.com/owner/repo/pull/123
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

### File Explorer

Multi-root file tree in the sidebar (Ctrl+Shift+E). When multiple folders are open, each root is shown as a collapsible group.

- Directories sorted before files, both alphabetically
- Expand/collapse with Enter or arrow keys
- Right-click context menu: **New File**, **New Folder**, **Rename**, **Delete**
- Sidebar actions button for **Refresh** and **New File**

### Search

Sidebar search panel (Ctrl+Shift+F) powered by [ripgrep](https://github.com/BurntSushi/ripgrep). Results are grouped by file with match counts.

- **Smart-case** matching by default
- **Include/Exclude glob filters** — click the toggle arrow to reveal filter inputs (e.g. `*.go`, `vendor/**`)
- Tab between search, include, and exclude inputs
- Searches across all workspace folders simultaneously
- Click a result to jump to the file and line

### Git Integration

Changes panel in the sidebar (Ctrl+Shift+G) with full staging workflow.

**Staging:**
- **Spacebar** — toggle stage/unstage on the selected file
- **`a`** — stage all unstaged files
- **`u`** — unstage all staged files
- **`+` button** on the "Changes" section header — stage all files in that section
- **`-` button** on the "Staged" section header — unstage all files in that section

**Committing:**
- Inline commit message input at the top of each group
- Type a message and press Enter to commit all staged files

**Remote operations:**
- **Pull**, **Push**, **Sync** (pull then push) from the sidebar actions button
- Per-repo actions via the group header menu button in multi-root workspaces

**Diff view:**
- Select a changed file to open a diff with syntax highlighting layered on diff backgrounds
- Untracked files open directly in the editor

**Multi-root:**
- Changes are grouped by repository, each with its own collapsible Staged/Changes sections
- Each group has a commit input and a menu button for pull/push/sync on that specific repo
- File status badges: **M** (modified), **A** (added), **D** (deleted), **R** (renamed), **U** (untracked)

### Command Palette

- **Ctrl+Shift+P** — opens the command palette with all available commands
- **Ctrl+P** — opens quick file open (searches all files across workspace folders)
- Type `>` in quick-open mode to switch to command mode
- Delete the `>` in command mode to switch to file mode
- Menu shortcuts resolve dynamically from your keybindings

### Bottom Panel

The bottom panel (Ctrl+J to toggle) contains the **Terminal**, **Problems**, and **References** tabs.

- **Problems tab** — lists all LSP diagnostics (errors, warnings) grouped by file; click to jump to location
- **References tab** — shows results from Find All References; click to jump to location
- **Terminal tab** — integrated terminal emulator (see below)

### Integrated Terminal

Built-in terminal emulator. Press Ctrl+` to toggle the terminal panel.

- **Ctrl+K T** to spawn a new terminal tab
- Multiple terminal tabs with a vertical inner tab bar on the left edge
- Full VT escape sequence support via `hinshun/vt10x` and PTY management via `creack/pty`
- True color (24-bit) and 256-color rendering
- When the terminal is focused, all keys go to the PTY except Ctrl+` (to toggle the panel)
- Terminal shell and scrollback are configurable in `settings.json`
- Terminal ANSI colors are theme-configurable via the `terminal` field in `theme.json`
- Close all terminals from the panel actions menu

### LSP (Language Server Protocol)

TTT has built-in LSP support for language-aware editing features. Install a language server and TTT detects it automatically — zero configuration needed. If a server isn't installed, TTT shows a brief notification with a link to install instructions.

#### Built-in Language Support

| Language | Server | Install |
|----------|--------|---------|
| Go | gopls | `go install golang.org/x/tools/gopls@latest` |
| TypeScript / JavaScript | typescript-language-server | `npm i -g typescript typescript-language-server` |
| Python | pyright | `pip install pyright` |
| C / C++ | clangd | `sudo apt install clangd` |
| Rust | rust-analyzer | `rustup component add rust-analyzer` |
| Vue | vue-language-server | `npm i -g @vue/language-server` |
| Svelte | svelteserver | `npm i -g svelte-language-server` |
| CSS / SCSS / Less | vscode-css-language-server | `npm i -g vscode-langservers-extracted` |
| HTML | vscode-html-language-server | `npm i -g vscode-langservers-extracted` |
| JSON | vscode-json-language-server | `npm i -g vscode-langservers-extracted` |
| YAML | yaml-language-server | `npm i -g yaml-language-server` |
| Bash | bash-language-server | `npm i -g bash-language-server` |
| Lua | lua-language-server | [LuaLS releases](https://github.com/LuaLS/lua-language-server/releases) |
| Zig | zls | [ZLS releases](https://github.com/zigtools/zls) |
| Kotlin | kotlin-language-server | [Releases](https://github.com/fwcd/kotlin-language-server/releases) |
| Java | jdtls | [Eclipse JDT.LS](https://github.com/eclipse-jdtls/eclipse.jdt.ls) |
| Ruby | ruby-lsp | `gem install ruby-lsp` |
| Dart | dart language-server | Included with [Dart SDK](https://dart.dev/get-dart) |
| Elixir | elixir-ls | [Releases](https://github.com/elixir-lsp/elixir-ls/releases) |
| PHP | phpactor | `composer global require phpactor/phpactor` |
| Terraform | terraform-ls | [Releases](https://github.com/hashicorp/terraform-ls) |
| Markdown | marksman | [Releases](https://github.com/artempyanykh/marksman/releases) |
| Tailwind CSS | tailwindcss-language-server | `npm i -g @tailwindcss/language-server` |
| Docker | docker-langserver | `npm i -g dockerfile-language-server-nodejs` |

You can also add custom servers or override built-in ones in `~/.config/ttt/settings.json`. See the [LSP docs](https://tttedit.dev/guides/lsp/) for details.

To disable LSP entirely: `"lsp": { "enabled": false }` in settings.

#### Supported Features

| Feature | Keybinding | Description |
|---------|-----------|-------------|
| Autocomplete | Ctrl+U | Trigger completion at cursor position |
| Signature Help | *(automatic)* | Parameter hints shown on `(` and `,` |
| Go to Definition | F12 | Jump to the definition of the symbol under the cursor |
| Go to Implementation | Shift+F12 | Jump to the implementation of the symbol under the cursor |
| Go to Type Definition | *(command palette)* | Jump to the type definition of the symbol under the cursor |
| Find References | *(command palette)* | Find all references to the symbol under the cursor (results in bottom panel REFERENCES tab) |
| Rename Symbol | F2 | Rename the symbol under the cursor across all files in the workspace |
| Hover | Ctrl+K I | Show type information and documentation for the symbol under the cursor |
| Format Document | *(command palette)* | Format the entire document using the language server |
| Format Selection | *(command palette)* | Format the selected range |
| Diagnostics | *(automatic)* | Error/warning squiggles inline, status bar summary, hover popup |

#### Auto-Completion

Completions trigger automatically as you type with a configurable debounce (default 150ms, set `autocomplete.debounce` in `settings.json`). The completion list filters live as you continue typing, and supports `completionItem/resolve` for additional details and auto-imports.

#### Diagnostics

The LSP server publishes diagnostics (errors, warnings, hints) which are displayed as:

- **Inline squiggles** — curly underlines on the affected range, colored by severity
- **Problems panel** — a tab in the bottom panel listing all diagnostics grouped by file
- **Hover popup** — hover over a squiggle to see the diagnostic message
- **Status bar** — error/warning counts shown in the status bar

#### Formatting

- **Format Document** — formats the entire file via the language server (available from the command palette)
- **Format Selection** — formats only the selected range (available from the command palette)
- **Format on Save** — enable `formatOnSave` in `settings.json` to auto-format when saving

#### References & Rename

- **Find All References** — search for all usages of the symbol under the cursor; results appear in the bottom panel REFERENCES tab (available from the command palette)
- **Rename Symbol** (F2) — rename the symbol under the cursor across all files in the workspace. Applies multi-file workspace edits automatically. Enable `lsp.saveOnRename` in `settings.json` to auto-save affected files after renaming.

### Tabs

Tabs follow a pin-on-reclick model similar to VS Code:

- Opening a file from the explorer or search replaces the current **unpinned** tab
- Clicking on an already-open tab (or opening the same file again) **pins** it
- **Ctrl+W** to close a tab, **Ctrl+PgDn/PgUp** to switch tabs
- Right-click a tab for **Close**, **Close Others**, **Close All**
- Tab bar actions button for **Close All**

### Theming

TTT supports fully customizable themes via JSON files. You can change every color in the editor — from syntax highlighting and diff backgrounds to the sidebar, tabs, status bar, terminal ANSI colors, borders, and semantic colors (success, danger, warning).

#### Built-in Themes

10 themes ship in [`internal/config/themes/`](internal/config/themes/):

- Aurora
- Bubblegum
- Default Dark
- Default Light
- Hotline
- Monokai
- One Dark
- Solarized Dark
- Solarized Light
- Virtru Dark

<details>
<summary>Default Dark theme (click to expand)</summary>

```json
{
  "default": { "fg": "#fafafa", "bg": "#1f1f1f" },
  "success": { "fg": "#73c991" },
  "danger":  { "fg": "#f14c4c" },
  "warning": { "fg": "#e2c08d" },
  "border":  { "fg": "#555555" },
  "statusBar": {},
  "tabs": {
    "active":   { "fg": "#ffffff", "bold": true },
    "inactive": { "fg": "#999999" }
  },
  "sidebar": {
    "header":   { "fg": "#ffffff", "bold": true },
    "item":     {},
    "selected": { "fg": "#ffffff", "bg": "#37373d" }
  },
  "dialog": {
    "input":    {},
    "item":     {},
    "selected": { "fg": "#ffffff", "bg": "#37373d" },
    "muted":    { "fg": "#888888" }
  },
  "editor": {
    "lineNumber":   { "fg": "#999999" },
    "activeLine":   { "bg": "#282828" },
    "selection":    { "bg": "#3a3d41" },
    "searchMatch":  {},
    "searchActive": {}
  },
  "menu": {
    "item":   {},
    "active": { "fg": "#ffffff", "bg": "#505050", "bold": true }
  },
  "diff": {
    "added":    { "bg": "#1e2e1e" },
    "deleted":  { "bg": "#2e1e1e" },
    "modified": { "bg": "#2e2e1e" }
  },
  "scrollbar": { "fg": "#999999", "bg": "#555555" },
  "syntax": {
    "comment":     { "fg": "#6a9955" },
    "string":      { "fg": "#ce9178" },
    "keyword":     { "fg": "#569cd6" },
    "number":      { "fg": "#b5cea8" },
    "operator":    { "fg": "#d4d4d4" },
    "function":    { "fg": "#dcdcaa" },
    "type":        { "fg": "#4ec9b0" },
    "builtin":     { "fg": "#4ec9b0" },
    "variable":    { "fg": "#9cdcfe" },
    "punctuation": { "fg": "#d4d4d4" },
    "tag":         { "fg": "#569cd6" },
    "attribute":   { "fg": "#9cdcfe" }
  },
  "borders": {
    "horizontal": "─", "vertical": "│",
    "topLeft": "┌", "topRight": "┐",
    "bottomLeft": "└", "bottomRight": "┘",
    "topTee": "┬", "bottomTee": "┴",
    "leftTee": "├", "rightTee": "┤"
  }
}
```

</details>

#### Switching Themes

Press **Ctrl+K Ctrl+T** (or use **View > Switch Theme** from the menu bar) to open the theme picker with a live preview.

#### Customizing

To create a custom theme, copy one of the built-in theme files to your themes directory and edit it:

```sh
mkdir -p ~/.config/ttt/themes
cp internal/config/themes/monokai.json ~/.config/ttt/themes/my-theme.json
```

Set it in `settings.json`:

```json
{
  "theme": "my-theme"
}
```

Restart TTT (or switch themes) to pick up changes.

To use your terminal's native colors instead of the theme's, set foreground/background to empty strings in your theme file.

### Menu Bar

File, Edit, Selection, View, and Help menus accessible via the menu bar or keyboard shortcuts. Menus display resolved keybindings next to each command. Navigate between menus with left/right arrow keys.

## Configuration

Config files are loaded from `<exe-dir>/config/` (bundled defaults) or `~/.config/ttt/` (user overrides):

| File | Purpose |
|------|---------|
| [`settings.json`](config/settings.json) | Editor settings (tabSize, wordWrap, theme, lsp, autocomplete, etc.) |
| [`keybindings.json`](config/keybindings.json) | Custom keybindings (VS Code key format) |
| `themes/*.json` | Custom color themes |

You can also open these directly from the command palette (**Ctrl+P**): **Preferences: Open Settings** and **Preferences: Open Keyboard Shortcuts**.

#### Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `tabSize` | int | `4` | Number of spaces per indentation level |
| `insertSpaces` | bool | `true` | Use spaces instead of tabs for indentation |
| `wordWrap` | bool | `false` | Wrap long lines at the editor width |
| `lineNumbers` | bool | `true` | Show line numbers in the gutter |
| `sidebarVisible` | bool | `true` | Show the sidebar on startup |
| `sidebarWidth` | int | `30` | Width of the sidebar in columns |
| `cursorStyle` | string | `""` | Cursor style: `"block"`, `"underline"`, or `"bar"` |
| `theme` | string | `""` | Theme name (from `~/.config/ttt/themes/`) |
| `debugMode` | bool | `false` | Enable debug logging to `~/.config/ttt/debug.log` |
| `formatOnSave` | bool | `false` | Auto-format the document via LSP on save |
| `insertFinalNewline` | bool | `true` | Ensure files end with a newline on save |
| `search.debounce` | int | `350` | Milliseconds to debounce global search input |
| `explorer.showHidden` | bool | `true` | Show hidden files (dot-prefixed) in the file explorer |
| `explorer.showGitIgnored` | bool | `true` | Show gitignored files in the file explorer |
| `terminal.shell` | string | `""` | Shell command for the integrated terminal (empty = system default) |
| `terminal.scrollback` | int | `1000` | Number of scrollback lines to retain in the terminal |
| `lsp.saveOnRename` | bool | `false` | Auto-save all files affected by a rename operation |
| `lsp.servers` | object | `{}` | Map of language ID to `{ "command": [...], "languages": {...} }` for LSP servers |
| `autocomplete.enabled` | bool | `true` | Enable LSP-powered autocompletion |
| `autocomplete.autoSuggest` | bool | `true` | Show completions automatically as you type |
| `autocomplete.debounce` | int | `150` | Milliseconds to wait after typing before requesting completions |
| `autocomplete.signatureHelp` | bool | `true` | Show function signature help on `(` and `,` |

Example `~/.config/ttt/settings.json` (also available at [`config/settings.json`](config/settings.json)):

```json
{
  "tabSize": 4,
  "insertSpaces": true,
  "wordWrap": false,
  "lineNumbers": true,
  "sidebarVisible": true,
  "sidebarWidth": 30,
  "theme": "default-dark",
  "formatOnSave": false,
  "insertFinalNewline": true,
  "search": {
    "debounce": 350
  },
  "explorer": {
    "showHidden": true,
    "showGitIgnored": true
  },
  "terminal": {
    "shell": "",
    "scrollback": 1000
  },
  "lsp": {
    "saveOnRename": false,
    "servers": {
      "go": { "command": ["gopls"] },
      "typescript": {
        "command": ["typescript-language-server", "--stdio"],
        "languages": {
          ".ts": "typescript",
          ".tsx": "typescriptreact",
          ".js": "javascript",
          ".jsx": "javascriptreact"
        }
      }
    }
  },
  "autocomplete": {
    "enabled": true,
    "autoSuggest": true,
    "debounce": 150,
    "signatureHelp": true
  }
}
```

## Keybindings

All keybindings are customizable via [`keybindings.json`](config/keybindings.json). Supports chord sequences (e.g. `ctrl+k ctrl+t`). These are the defaults:

| Shortcut | Action |
|----------|--------|
| | **General** |
| Ctrl+Q | Quit |
| Ctrl+P | Command palette |
| Alt+P | Quick open file |
| Escape | Focus editor |
| | **File** |
| Ctrl+N | New file |
| Ctrl+S | Save |
| Ctrl+K S | Save as |
| | **Editor** |
| Ctrl+Z | Undo |
| Ctrl+Y | Redo |
| Ctrl+A | Select all |
| Ctrl+C | Copy |
| Ctrl+X | Cut |
| Ctrl+V | Paste |
| Ctrl+G | Go to line |
| | **Multi-Cursor** |
| Ctrl+D | Select next occurrence |
| Ctrl+K L | Select all occurrences |
| Alt+Click | Add cursor at click position |
| Ctrl+K U | Undo last cursor addition |
| Escape | Collapse to single cursor |
| | **Search** |
| Ctrl+F | Find |
| Ctrl+H | Find and replace |
| F3 / Shift+F3 | Find next / previous |
| | **View** |
| Ctrl+B | Toggle sidebar |
| Ctrl+J | Toggle bottom panel |
| Ctrl+K E | Show file explorer |
| Ctrl+K F | Search across files |
| Ctrl+K C | Show changes |
| Ctrl+0 | Focus sidebar |
| Alt+Shift+Left/Right | Resize sidebar |
| Alt+Shift+Up/Down | Resize bottom panel |
| Ctrl+K Ctrl+T | Switch theme |
| | **Tabs** |
| Ctrl+PgDn / PgUp | Next / previous tab |
| Ctrl+W | Close tab |
| | **LSP** |
| Ctrl+U | Autocomplete |
| F12 | Go to definition |
| Shift+F12 | Go to implementation |
| F2 | Rename symbol |
| Ctrl+K I | Hover info |
| | **Terminal / Bottom Panel** |
| Ctrl+` | Toggle terminal |
| Ctrl+J | Toggle bottom panel |
| Ctrl+K T | New terminal tab |
| | **Changes Panel** |
| Space | Toggle stage/unstage file |
| A | Stage all |
| U | Unstage all |
| R | Refresh |
| Enter | Open diff / activate |
| | **Menu Bar** |
| F10 / Alt+F | File menu |
| Alt+E / S / V / H | Edit / Selection / View / Help |

## Testing

TTT is tested at two levels to catch bugs across the entire stack — from core data structures to end-to-end user workflows.

### Unit Tests (Go)

The core editor engine (`internal/core/`) is fully unit-testable in isolation with zero terminal dependencies. Buffer operations, cursor math, undo/redo commands, and syntax highlighting all have dedicated Go tests.

```sh
make test                            # run all unit tests
go test ./internal/core/buffer/      # test a single package
```

### Functional Tests (vitest + tui-use)

Black-box tests that launch the real `ttt` binary in a real terminal, type keys, and assert on screen output and file contents. These catch integration issues that unit tests can't — keybinding wiring, rendering, file I/O round-trips, and cross-component interactions.

Built with [vitest](https://vitest.dev/) and [tui-use](https://github.com/onesuper/tui-use) (terminal automation for TUI apps). 44 tests across 16 files covering:

- **File operations** — open, edit, save, Save As, new file, dirty indicator
- **Editing** — undo/redo, select all + overwrite, line delete/move/duplicate, word delete
- **Unicode** — accented characters, CJK, emoji
- **Find & Replace** — search, match navigation, replace, save verification
- **Navigation** — go to line, tab switching
- **UI panels** — sidebar toggle, panel switching, terminal toggle, command palette
- **Tab management** — multi-tab, close, unsaved changes dialog

```sh
cd tests/functional
pnpm install
pnpm test              # run all functional tests
pnpm test:watch        # watch mode
pnpm test:stress       # run 10 times to catch flaky tests
```

Functional tests run in CI on every push and pull request.

### Chaos Monkey (Fuzz Testing)

A randomized fuzz tester that hammers the editor with thousands of random events — keypresses, mouse clicks, resizes, clipboard operations, and command palette commands — to find panics and crashes that normal testing misses. It runs against a `tcell.SimulationScreen` so no real terminal is needed.

Runs in Docker to prevent random commands from opening browser tabs or interfering with your session.

```sh
# Quick local run (50 iterations x 500 events)
make chaos

# Build Docker image
make chaos-docker-build

# Run continuously in Docker (crash logs saved to chaos-output/)
make chaos-docker

# Reproduce a specific crash deterministically
CHAOS_REPLAY=chaos-output/crash-<seed>-<iter>.json go test ./tests/chaos/ -run TestChaosReplay
```

Each crash is saved as a JSON report with the random seed and full event log, so any panic can be replayed and debugged deterministically.

## Architecture

The codebase follows a strict layered architecture: **core -> view -> render -> term -> ui**.

- **`internal/core/`** — UI-agnostic editor engine (buffer, cursor, undo, syntax highlighting, multi-buffer management)
- **`internal/view/`** — Viewport scrolling and status bar
- **`internal/render/`** — Diff-based double-buffered renderer for minimal terminal writes
- **`internal/terminal/`** — Integrated terminal emulator (vt10x + pty)
- **`internal/term/`** — Terminal abstraction via `Screen` interface (only package that imports tcell)
- **`internal/ui/`** — Window manager, pane system, and all widgets
- **`internal/lsp/`** — Language Server Protocol client (JSON-RPC 2.0 over stdio, per-language server management)
- **`internal/workspace/`** — Multi-folder workspace management with git detection
- **`internal/git/`** — Git operations (status, stage, unstage, commit, pull, push, diff, blame)
- **`internal/config/`** — Configuration loading (settings, themes, keybindings, editorconfig)

## License

MIT
