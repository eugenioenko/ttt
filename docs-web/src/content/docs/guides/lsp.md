---
title: LSP
description: Language Server Protocol support in TTT.
---

TTT has built-in LSP support for language-aware editing features. Language servers run as external processes and communicate over JSON-RPC 2.0 via stdio.

## Built-in Language Support

TTT ships with 24 built-in language server configurations. You just need to install the server binary and TTT will detect and use it automatically. If a server isn't installed, TTT shows a brief notification when you open a file of that language (disable this with `lsp.notifyAvailability: false`).

To disable LSP entirely, set `lsp.enabled` to `false` in your settings:

```json
{
  "lsp": {
    "enabled": false
  }
}
```

### Go {#go}

```sh
go install golang.org/x/tools/gopls@latest
```

### TypeScript / JavaScript {#typescript}

```sh
npm install -g typescript typescript-language-server
```

Handles `.ts`, `.tsx`, `.js`, `.jsx`, `.mjs`, `.mts`, `.cjs`, and `.cts` files.

### Python {#python}

```sh
pip install pyright
# or
npm install -g pyright
```

### C / C++ {#c}

```sh
# Ubuntu/Debian
sudo apt install clangd

# macOS
brew install llvm

# Arch
sudo pacman -S clang
```

Handles `.c`, `.h`, `.cpp`, `.hpp`, `.cc`, and `.cxx` files.

### Vue {#vue}

```sh
npm install -g @vue/language-server
```

Handles `.vue` files.

### Rust {#rust}

```sh
rustup component add rust-analyzer
```

### Lua {#lua}

Install from [LuaLS releases](https://github.com/LuaLS/lua-language-server/releases) or via your package manager:

```sh
# macOS
brew install lua-language-server

# Arch
sudo pacman -S lua-language-server
```

### Svelte {#svelte}

```sh
npm install -g svelte-language-server
```

### CSS / SCSS / Less {#css}

```sh
npm install -g vscode-langservers-extracted
```

Handles `.css`, `.scss`, and `.less` files.

### HTML {#html}

```sh
npm install -g vscode-langservers-extracted
```

### JSON {#json}

```sh
npm install -g vscode-langservers-extracted
```

Handles `.json` and `.jsonc` files.

### YAML {#yaml}

```sh
npm install -g yaml-language-server
```

### Bash {#bash}

```sh
npm install -g bash-language-server
```

Handles `.sh` and `.bash` files.

### Docker {#docker}

```sh
npm install -g dockerfile-language-server-nodejs
```

### Tailwind CSS {#tailwindcss}

```sh
npm install -g @tailwindcss/language-server
```

### Kotlin {#kotlin}

See [kotlin-language-server releases](https://github.com/fwcd/kotlin-language-server/releases).

### Java {#java}

See [Eclipse JDT Language Server](https://github.com/eclipse-jdtls/eclipse.jdt.ls).

### Ruby {#ruby}

```sh
gem install ruby-lsp
```

### Dart {#dart}

Included with the [Dart SDK](https://dart.dev/get-dart):

```sh
dart language-server --protocol=lsp
```

### Elixir {#elixir}

See [elixir-ls releases](https://github.com/elixir-lsp/elixir-ls/releases).

### PHP {#php}

```sh
composer global require phpactor/phpactor
```

### Terraform {#terraform}

```sh
# See https://github.com/hashicorp/terraform-ls
```

Handles `.tf` and `.tfvars` files.

### Markdown {#markdown}

```sh
# See https://github.com/artempyanykh/marksman/releases
```

### Zig {#zig}

```sh
# See https://github.com/zigtools/zls
```

## Custom Language Servers

Add or override language servers in `~/.config/ttt/settings.json` under the `lsp.servers` key. Each entry maps a server key to a config object with a `command` array:

```json
{
  "lsp": {
    "servers": {
      "ocaml": { "command": ["ocamllsp"] }
    }
  }
}
```

For simple cases, the server key is used as the language ID and files are matched via the syntax highlighter name. No extra configuration is needed.

### Overriding built-in configs

To override a built-in server, use the same key. For example, to use `tsserver` instead of the default `typescript-language-server`:

```json
{
  "lsp": {
    "servers": {
      "typescript": {
        "command": ["tsserver", "--stdio"],
        "languages": {
          ".ts": "typescript",
          ".tsx": "typescriptreact",
          ".js": "javascript",
          ".jsx": "javascriptreact"
        }
      }
    }
  }
}
```

To add file extensions to an existing server, override the full entry with the additional extensions included. For example, to add `.svelte` support to the TypeScript server:

```json
{
  "lsp": {
    "servers": {
      "typescript": {
        "command": ["typescript-language-server", "--stdio"],
        "languages": {
          ".ts": "typescript",
          ".tsx": "typescriptreact",
          ".js": "javascript",
          ".jsx": "javascriptreact",
          ".mjs": "javascript",
          ".svelte": "typescript"
        }
      }
    }
  }
}
```

### The `languages` field

The optional `languages` field is for servers that handle multiple file types requiring different `languageId` values. It maps file extensions to language IDs. Without the `languages` field, the server key is used as the language ID and files are matched by their syntax highlighter name.

The server is started lazily on first use and shut down when the editor exits.

## Server Status

The language segment on the right of the status bar shows the state of the server for the current file:

| Indicator | Meaning |
|-----------|---------|
| `Go ◉` | Connected and initialized |
| `Go ◌` | Starting, or not launched yet — servers start on first use |
| `Go ⚠` | Failed to start, exited, or the binary is not installed |
| `Go` | No server configured for this language |

Click the segment to open the OUTPUT panel, where each server logs its startup command, its own stderr, initialization failures and unexpected exits under the `lsp:<server>` prefix. That is the place to look when a feature silently does nothing.

## Supported Features

| Feature | Keybinding | Description |
|---------|-----------|-------------|
| Autocomplete | Ctrl+U | Trigger completion at cursor position |
| Signature Help | *(automatic)* | Parameter hints shown on `(` and `,` |
| Go to Definition | F12 | Jump to the definition of the symbol under the cursor |
| Go to Implementation | Shift+F12 | Jump to the implementation |
| Go to Type Definition | Ctrl+L T | Jump to the type definition |
| Find References | Ctrl+L R | Find all references (results in bottom panel) |
| Rename Symbol | F2 | Rename across all files in the workspace |
| Hover | Ctrl+K I | Show type information and documentation |
| Format Document | Ctrl+L F | Format the entire document |
| Format Selection | Ctrl+L S | Format the selected range |
| Organize Imports | Ctrl+L O | Organize imports via code action |
| Fix All | Ctrl+L X | Apply all available fixes |
| Diagnostics | *(automatic)* | Error/warning squiggles, status bar summary, hover popup |

## Auto-Completion

Completions trigger automatically as you type with a configurable debounce (default 150ms). The completion list filters live as you continue typing, and supports `completionItem/resolve` for additional details and auto-imports.

**Ctrl+U** also works as a manual trigger.

### Configuration

```json
{
  "autocomplete": {
    "enabled": true,
    "autoSuggest": true,
    "debounce": 150,
    "signatureHelp": true
  }
}
```

## Diagnostics

The LSP server publishes diagnostics (errors, warnings, hints) which are displayed as:

- **Inline squiggles** on the affected range, colored by severity
- **Diagnostics panel** in the bottom panel listing all diagnostics grouped by file
- **Hover popup** over a squiggle to see the diagnostic message
- **Status bar** error/warning counts

## Formatting

- **Format Document** (Ctrl+L F) formats the entire file
- **Format Selection** (Ctrl+L S) formats only the selected range
- **Format on Save** can be enabled in settings

```json
{
  "editor": {
    "formatOnSave": true
  }
}
```

## References and Rename

- **Find All References** (Ctrl+L R) shows all usages in the bottom panel REFERENCES tab
- **Rename Symbol** (F2) renames across all files in the workspace, applying multi-file workspace edits automatically

Enable `lsp.saveOnRename` to auto-save files affected by a rename:

```json
{
  "lsp": {
    "saveOnRename": true
  }
}
```

## Code Actions on Save

Configure automatic code actions that run before each save:

```json
{
  "lsp": {
    "codeActionsOnSave": [
      "source.organizeImports",
      "source.fixAll"
    ]
  }
}
```

## LSP Settings Reference

All LSP settings are nested under `lsp.*` in `settings.json`. Autocomplete settings are separate, under `autocomplete.*`.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `lsp.enabled` | boolean | `true` | Enable or disable LSP support entirely |
| `lsp.hover` | boolean | `true` | Enable or disable hover information |
| `lsp.hoverDelay` | number | `500` | Delay in milliseconds before showing hover info |
| `lsp.saveOnRename` | boolean | `false` | Auto-save files affected by a rename |
| `lsp.codeActionsOnSave` | string[] | `[]` | Code actions to run before each save |
| `lsp.notifyAvailability` | boolean | `true` | Show a notification when a language server is not installed |
| `lsp.servers` | object | *(24 built-in)* | Custom or overridden language server configurations |
