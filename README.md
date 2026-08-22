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
- **Find and Replace** — inline find bar (Ctrl+F) with match navigation, replace bar (Ctrl+R) with replace-one and replace-all
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
ttt /path/to/file.go            # opens just the file — no workspace folder
ttt /path/to/repo/subdir        # opens that folder; git features (changes,
                                # branch) use the enclosing git repo root
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

### Git Integration

Changes panel in the sidebar (Ctrl+K C) with full staging workflow.

Working-tree files and files under expanded commits can be shown as a compact
directory **Tree** or a full-path **List** (the default). The choice persists in
`git.fileView`. Changes, commit details, and Explorer expose safe **Expand All**
and **Collapse All** actions in their relevant menus.

**Staging:**
- **Spacebar** — toggle stage/unstage on the selected file
- **`a`** — stage all unstaged files
- **`u`** — unstage all staged files
- **`+` button** on the "Changes" section header — stage all files in that section
- **`-` button** on the "Staged" section header — unstage all files in that section

**Committing:**
- Inline commit message input below the file list (Tab from the tree to focus it)
- Type a message and press Enter to commit all staged files
- Commit History starts with the recent viewport-sized window; activate **Load older commits…** to append bounded pages from the same HEAD snapshot

**Remote operations:**
- **Pull**, **Push**, **Sync** (pull then push) from the sidebar actions button
- Per-repo actions via the group header menu button in multi-root workspaces

**Diff view:**
- Select a changed file to open a split or unified diff with syntax highlighting layered on diff backgrounds
- Set the global view mode, context, wrapping, and high-contrast presentation under **Options**; the Changes panel menu provides the same contextual controls
- Changes-only views show quiet collapsed-context rows that can be expanded in place; full-file context remains available globally or per diff
- Untracked files open directly in the editor

**Multi-root:**
- Changes are grouped by repository, each with its own collapsible Staged/Changes sections
- Each group has a commit input and a menu button for pull/push/sync on that specific repo
- File status badges: **M** (modified), **A** (added), **D** (deleted), **R** (renamed), **U** (untracked)

### Command Palette

- **Ctrl+P** — opens the command palette with all available commands
- **Ctrl+K P** — opens quick file open (searches all files across workspace folders)
- Type `>` in quick-open mode to switch to command mode
- Delete the `>` in command mode to switch to file mode
- Type `?` to browse searchable help for panels, navigation, and key chords
- Menu shortcuts resolve dynamically from your keybindings

### Bottom Panel

The bottom panel (Ctrl+K B to toggle) contains the **Terminal**, **Problems**, and **References** tabs.

- **Diagnostics tab** — lists all LSP diagnostics (errors, warnings) grouped by file; click to jump to location
- **References tab** — shows results from Find All References; click to jump to location
- **Terminal tab** — integrated terminal emulator (see below)

### Integrated Terminal

Built-in terminal emulator. Press Ctrl+T to toggle the terminal panel, or Alt+T for fullscreen.

- **Ctrl+K T** to spawn a new terminal tab
- Multiple terminal tabs with a vertical inner tab bar on the left edge
- Full VT escape sequence support via `hinshun/vt10x` and PTY management via `creack/pty`
- True color (24-bit) and 256-color rendering
- When the terminal is focused, all keys go to the PTY except force keys (Ctrl+T, Alt+T, Ctrl+Q, Ctrl+P, Ctrl+K P, Ctrl+B, F6)
- Terminal shell and scrollback are configurable in `settings.json`
- Terminal ANSI colors are theme-configurable via the `terminal` field in `theme.json`
- Close all terminals from the panel actions menu

### LSP (Language Server Protocol)

TTT has built-in LSP support for language-aware editing features. Language servers are configured via plugins — install the LSP plugin for your language and the corresponding server binary.

#### LSP Plugins

Install LSP plugins from the Plugins panel or command palette. Each plugin configures the language server automatically. Available plugins:

| Plugin | Language | Server |
|--------|----------|--------|
| `lsp-go` | Go | gopls |
| `lsp-typescript` | TypeScript / JavaScript | typescript-language-server |
| `lsp-python` | Python | pyright |
| `lsp-c` | C / C++ | clangd |
| `lsp-rust` | Rust | rust-analyzer |
| `lsp-lua` | Lua | lua-language-server |
| `lsp-zig` | Zig | zls |
| `lsp-vue` | Vue | vue-language-server |
| `lsp-svelte` | Svelte | svelteserver |
| `lsp-css` | CSS / SCSS / Less | vscode-css-language-server |
| `lsp-html` | HTML | vscode-html-language-server |
| `lsp-json` | JSON | vscode-json-language-server |
| `lsp-yaml` | YAML | yaml-language-server |
| `lsp-bash` | Bash | bash-language-server |
| `lsp-docker` | Docker | docker-langserver |
| `lsp-tailwindcss` | Tailwind CSS | tailwindcss-language-server |
| `lsp-kotlin` | Kotlin | kotlin-language-server |
| `lsp-java` | Java | jdtls |
| `lsp-ruby` | Ruby | ruby-lsp |
| `lsp-dart` | Dart | dart language-server |
| `lsp-elixir` | Elixir | elixir-ls |
| `lsp-php` | PHP | phpactor |
| `lsp-terraform` | Terraform | terraform-ls |
| `lsp-markdown` | Markdown | marksman |

You can also add custom servers manually in `~/.config/ttt/settings.json`. See the [LSP docs](https://tttedit.dev/guides/lsp/) for details.

To disable LSP entirely: `"lsp": { "enabled": false }` in settings.

#### Server Status

The language segment in the status bar shows the state of the server for the current file: `◉` connected, `◌` starting or not yet launched, `⚠` failed to start or exited, and no indicator when no server is configured. Click it to open the OUTPUT panel, where each server logs its startup command, stderr and any failures under `lsp:<server>`.

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
| Format Document (LSP) | Ctrl+L F | Format the entire document using the language server |
| Format Document (External) | Ctrl+L E | Format using an external formatter from settings |
| Format Selection | *(command palette)* | Format the selected range via LSP |
| Diagnostics | *(automatic)* | Error/warning squiggles inline, status bar summary, hover popup |
| Outline\* | *(command palette)* | Symbol tree of the current file in the sidebar; navigate and jump to definitions. Uses LSP document symbols with a built-in fallback for Go and Markdown |

\* Thanks to [@tenox7](https://github.com/tenox7) for contributing the Outline feature.

#### Auto-Completion

Completions trigger automatically as you type with a configurable debounce (default 150ms, set `autocomplete.debounce` in `settings.json`). The completion list filters live as you continue typing, and supports `completionItem/resolve` for additional details and auto-imports.

#### Diagnostics

The LSP server publishes diagnostics (errors, warnings, hints) which are displayed as:

- **Inline squiggles** — curly underlines on the affected range, colored by severity
- **Diagnostics panel** — a tab in the bottom panel listing all diagnostics grouped by file
- **Hover popup** — hover over a squiggle to see the diagnostic message
- **Status bar** — error/warning counts shown in the status bar

#### Formatting

- **Format Document (LSP)** (`Ctrl+L F`) — formats the entire file via the language server
- **Format Document (External)** (`Ctrl+L E`) — formats the file using an external formatter configured in `settings.json`
- **Format Selection** — formats only the selected range via LSP (available from the command palette)
- **Format on Save** — enable `editor.formatOnSave` in `settings.json` to auto-format when saving. External formatters take priority over LSP; if no external formatter is configured for the file type, LSP formatting is used as a fallback.

**External formatters** are configured per file extension in a top-level `formatters` map:

```json
{
  "formatters": {
    "go": "gofmt",
    "lua": "stylua -",
    "js": "prettier --stdin-filepath {file}",
    "py": "black -"
  }
}
```

The formatter receives the buffer via stdin and must write formatted output to stdout. Use `{file}` as a placeholder for the file path.

#### References & Rename

- **Find All References** — search for all usages of the symbol under the cursor; results appear in the bottom panel REFERENCES tab (available from the command palette)
- **Rename Symbol** (F2) — rename the symbol under the cursor across all files in the workspace. Applies multi-file workspace edits automatically. Enable `lsp.saveOnRename` in `settings.json` to auto-save affected files after renaming.

### Tabs

Tabs follow a pin-on-reclick model similar to VS Code:

- Opening a file from the explorer or search replaces the current **unpinned** tab
- Clicking on an already-open tab (or opening the same file again) **pins** it
- **Ctrl+W** to close a tab, **Alt+.** / **Alt+,** to switch tabs
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

Use **View > Switch Theme** from the menu bar (or run **Switch Theme** from the command palette) to open the theme picker with a live preview.

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

### Plugins

TTT supports Lua plugins that add sidebar panels, bottom panel tabs, commands, and keybindings. Plugins run in a sandboxed Lua VM with a permission system — users approve each plugin's capabilities on first load.

#### Vim Mode

Prefer modal editing? Install the [Vim Mode](https://github.com/eugenioenko/ttt-vim) plugin for a Vim compatibility layer: Normal, Insert and Visual modes, the operator/motion/text-object grammar, registers, marks, macros, dot-repeat, a `:` command line and `/` search. It is a single Lua plugin with no core changes of its own, and it enables or disables like any other plugin.

It does rely on plugin API additions that landed in ttt 1.1.0 (the key interceptor's precedence over Escape, needed to leave Insert mode, and the command-line API for `:` and `/`), so it requires ttt 1.1.0 or newer. Its manifest declares `api: 2`, so an older editor refuses to load it rather than running it half-broken.

Install it from the **Plugins** sidebar tab by searching for "vim", or run **Plugins: Install from URL** with `https://github.com/eugenioenko/ttt-vim`.

#### Installing Plugins

Open the **Plugins** sidebar tab to browse and install from the community registry, or use **Plugins: Install from URL** from the command palette to install from any git repository.

Community plugins are maintained at [ttt-plugins](https://github.com/eugenioenko/ttt-plugins):

| Plugin | Description |
|--------|-------------|
| cheat-sheet | Fetch cheat sheets from cheat.sh |
| color-picker | Color picker with hex/RGB swatches |
| docker-manager | Docker container management |
| go-test-runner | Run Go tests and view results |
| http-client | HTTP request client |
| json-viewer | Interactive JSON tree viewer |
| markdown-preview | Markdown preview panel |
| notepad | Persistent scratchpad |
| todo-scanner | Scan for TODO/FIXME/HACK/NOTE comments |

#### Managing Plugins

- Click an installed plugin to **enable/disable** it (persists across restarts)
- **↑** button to update, **×** to uninstall
- **Plugins: Reload** from the command palette for live reload during development

#### Disabling the Plugin System

To disable plugins entirely: `"plugins": { "enabled": false }` in settings.

#### Creating Plugins

Plugins are Lua scripts with a `plugin.ttt.json` manifest. See the [Plugin Authoring Guide](https://tttedit.dev/guides/plugin-authoring/) for the full API. To list your plugin in the built-in browser, submit a PR adding it to `community-plugins.json`.

### Menu Bar

File, Edit, Selection, View, and Help menus accessible via the menu bar or keyboard shortcuts. Menus display resolved keybindings next to each command. Navigate between menus with left/right arrow keys.

## Configuration

Config files are loaded from `<exe-dir>/config/` (bundled defaults) or `~/.config/ttt/` (user overrides):

| File | Purpose |
|------|---------|
| [`settings.json`](config/settings.json) | Editor settings (tabSize, wordWrap, theme, lsp, autocomplete, etc.) |
| [`keybindings.json`](config/keybindings.json) | Custom keybindings (VS Code key format) |
| `themes/*.json` | Custom color themes |

Most settings can be edited from a form instead of by hand: **View → Settings**, **Ctrl+K ,**, or **Settings: Open Editor Settings** from the command palette (**Ctrl+P**). The form is grouped into **Editor**, **Appearance**, **Completion** and **Advanced** (Git, explorer, terminal, search and plugin options live under Advanced). Changes are held until you press **Apply** (also **Settings: Apply Changes**), which writes `settings.json` and applies everything that does not need a restart; **Cancel** (also **Settings: Discard Changes**) closes the tab and drops them. Settings marked *(restart)* take effect on next launch. LSP settings and external formatters are not exposed in the form — those stay JSON-only.

To edit the raw files, use **Settings: Open settings.json** and **Settings: Open keybindings.json**.

#### Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `tabSize` | int | `4` | Number of spaces per indentation level |
| `insertSpaces` | bool | `true` | Use spaces instead of tabs for indentation |
| `wordWrap` | bool | `false` | Wrap long lines at the editor width |
| `diffMode` | string | `"split"` | Default diff layout: `"split"` or `"unified"` |
| `diffContext` | string | `"changes"` | Default diff context: `"changes"` or `"full"` |
| `diffWordWrap` | bool | `false` | Wrap long lines in diff views |
| `diffHighContrast` | bool | `false` | Use semantic red/green foregrounds as well as diff backgrounds |
| `autoIndent` | bool | `true` | Inherit the previous line's indent on Enter, plus one level after `{ ( [ :` (turn off for `noautoindent` behavior) |
| `autoDedent` | bool | `true` | Dedent one level when typing a closing `} ) ]` on a blank line |
| `lineNumbers` | bool | `true` | Show line numbers in the gutter |
| `sidebarVisible` | bool | `true` | Show the sidebar on startup |
| `sidebarWidth` | int | `30` | Width of the sidebar in columns |
| `cursorStyle` | string | `""` | Cursor style: `"block"`, `"underline"`, or `"bar"` |
| `theme` | string | `""` | Theme name (from `~/.config/ttt/themes/`) |
| `debugMode` | bool | `false` | Enable debug logging to `~/.config/ttt/debug.log` |
| `formatOnSave` | bool | `false` | Auto-format on save (external formatter first, then LSP) |
| `formatters.<ext>` | string | — | External formatter command for the given file extension (e.g. `formatters.go`, `formatters.js`) |
| `insertFinalNewline` | bool | `true` | Ensure files end with a newline on save |
| `search.debounce` | int | `350` | Milliseconds to debounce global search input |
| `explorer.showHidden` | bool | `true` | Show hidden files (dot-prefixed) in the file explorer |
| `explorer.showGitIgnored` | bool | `true` | Show gitignored files in the file explorer |
| `sidebar.panelOrder` | string[] | built-in order | Preferred sidebar panel IDs; updated when headers are dragged or moved with commands |
| `git.fileView` | string | `"list"` | Show changed and historical files as a compact `"tree"` or full-path `"list"` |
| `terminal.shell` | string | `""` | Shell command for the integrated terminal (empty = system default) |
| `terminal.scrollback` | int | `1000` | Number of scrollback lines to retain in the terminal |
| `lsp.saveOnRename` | bool | `false` | Auto-save all files affected by a rename operation |
| `lsp.servers` | object | `{}` | Map of language ID to `{ "command": [...], "languages": {...} }` for LSP servers. Configured automatically by LSP plugins. |
| `autocomplete.enabled` | bool | `true` | Enable LSP-powered autocompletion |
| `autocomplete.autoSuggest` | bool | `true` | Show completions automatically as you type |
| `autocomplete.debounce` | int | `150` | Milliseconds to wait after typing before requesting completions |
| `autocomplete.signatureHelp` | bool | `true` | Show function signature help on `(` and `,` |
| `plugins.enabled` | bool | `true` | Enable the plugin system (set `false` to disable all plugins) |

Example `~/.config/ttt/settings.json` (also available at [`config/settings.json`](config/settings.json)):

```json
{
  "tabSize": 4,
  "insertSpaces": true,
  "wordWrap": false,
  "diffMode": "split",
  "diffContext": "changes",
  "diffWordWrap": false,
  "autoIndent": true,
  "autoDedent": true,
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
  "sidebar": {
    "panelOrder": ["explorer", "search", "changes", "outline"]
  },
  "git": {
    "fileView": "list"
  },
  "terminal": {
    "shell": "",
    "scrollback": 1000
  },
  "lsp": {
    "saveOnRename": false
  },
  "autocomplete": {
    "enabled": true,
    "autoSuggest": true,
    "debounce": 150,
    "signatureHelp": true
  },
  "plugins": {
    "enabled": true
  },
  "formatters": {
    "go": "gofmt",
    "lua": "stylua -",
    "js": "prettier --stdin-filepath {file}"
  }
}
```

## Keybindings

All keybindings are customizable via [`keybindings.json`](config/keybindings.json). Supports chord sequences (e.g. `ctrl+k e`). These are the defaults:

| Shortcut | Action |
|----------|--------|
| | **General** |
| Ctrl+Q | Quit |
| Ctrl+P | Command palette |
| Ctrl+K P | Quick open file |
| Escape | Dismiss overlay / focus editor |
| F6 / Shift+F6 | Focus next / previous group |
| | **File** |
| Ctrl+N | New file |
| Ctrl+O | Open folder |
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
| Ctrl+/ | Toggle line comment |
| Ctrl+Enter | Insert line below |
| Alt+Up / Alt+Down | Move line up / down |
| Ctrl+K K | Delete line |
| Ctrl+K J | Join lines |
| Ctrl+K O | Sort lines |
| Ctrl+K M | Go to matching bracket |
| Ctrl+K [ | Toggle fold |
| Ctrl+K 0 / Ctrl+K 9 | Collapse / expand all folds |
| | **Multi-Cursor** |
| Ctrl+D | Select next occurrence |
| Ctrl+K L | Select all occurrences |
| Alt+Click | Add cursor at click position |
| Ctrl+K U | Undo last cursor addition |
| Escape | Collapse to single cursor |
| | **Search** |
| Ctrl+F | Find |
| Ctrl+R | Find and replace |
| F3 / Shift+F3 | Find next / previous |
| | **View** |
| Ctrl+B | Toggle sidebar |
| Ctrl+K B | Toggle bottom panel |
| Alt+Shift+M | Show/hide the menu bar |
| Ctrl+K E | Show file explorer |
| Ctrl+K F | Search across files |
| Ctrl+K R | Search and replace in files |
| Ctrl+K C | Show changes |
| Ctrl+K Y | Open keyboard shortcuts |
| Ctrl+0 | Focus sidebar |
| Alt+Shift+Left/Right | Resize sidebar |
| Alt+Shift+Up/Down | Resize bottom panel |
| | **Tabs** |
| Alt+. / Alt+, | Next / previous tab |
| Ctrl+W | Close tab |
| | **LSP** |
| Ctrl+U | Autocomplete |
| F12 | Go to definition |
| Shift+F12 | Go to implementation |
| F2 | Rename symbol |
| Ctrl+K I | Hover info |
| Ctrl+L F | Format document (LSP) |
| Ctrl+L E | Format document (external) |
| | **Terminal / Bottom Panel** |
| Ctrl+T | Toggle terminal |
| Alt+T | Terminal fullscreen |
| Ctrl+K T | New terminal tab |
| Ctrl+K B | Toggle bottom panel |
| | **Changes Panel** |
| Space | Toggle stage/unstage file |
| A | Stage all |
| U | Unstage all |
| R | Refresh |
| Enter | Open diff / activate |
| | **Menu Bar** |
| F10 / Alt+F | File menu |
| Alt+E / S / V / O / H | Edit / Selection / View / Options / Help |
| Alt+Shift+M | Show/hide the menu bar |

## Testing

TTT is tested at three levels to catch bugs across the entire stack — from core data structures to end-to-end user workflows.

### Unit Tests (Go)

The core editor engine (`internal/core/`) is fully unit-testable in isolation with zero terminal dependencies. Buffer operations, cursor math, undo/redo commands, and syntax highlighting all have dedicated Go tests.

```sh
make test                            # run all unit tests
go test ./internal/core/buffer/      # test a single package
```

### Functional Tests (vitest + --exec)

The primary test suite. These launch the real `ttt` binary via the built-in `--exec` debug harness, run scripted commands in batch, and assert on screenshots and file contents. No external dependencies beyond [vitest](https://vitest.dev/). 142 tests across 39 files covering:

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

### Integration Tests (vitest + tui-use)

Tests that require live PTY interaction — scenarios where something external happens while the editor is running. Built with [vitest](https://vitest.dev/) and [tui-use](https://github.com/onesuper/tui-use). 7 tests covering LSP completions, external file changes, settings roundtrip, and bracketed paste.

```sh
cd tests/integration
pnpm install
pnpm test
```

All test levels run in CI on every push and pull request.

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

## Contributors & Acknowledgments

TTT is better because of the people who took the time to try it, report bugs, request features, and contribute code. Thank you — this project grows because of you. 🙏

**Code contributions**

- [@tenox7](https://github.com/tenox7) — the **Outline** sidebar panel (LSP document symbols with a built-in Go/Markdown fallback) and **markdown syntax highlighting**.
- [@pirate-boop](https://github.com/pirate-boop) — **NixOS support** end to end: the initial `flake.nix` and ongoing `vendorHash` upkeep.

**Packaging**

- [@Dominiquini](https://github.com/Dominiquini) — maintaining the [Arch Linux (AUR)](https://aur.archlinux.org/packages/ttt) package.

**Bug reports & feature requests**

- [@jetpax](https://github.com/jetpax) — surfacing the macOS / iTerm2 issues (mouse support, clipboard copy, large-list scrolling, workspace paths).
- [@egorse](https://github.com/egorse) — the search-panel focus fix.
- [@pirate-boop](https://github.com/pirate-boop) — the Cyrillic cursor-offset fix and a number of UX/packaging ideas.

Want to help? Bug reports, feature requests, and pull requests are all welcome — open an [issue](https://github.com/eugenioenko/ttt/issues) or a PR.

## License

MIT
