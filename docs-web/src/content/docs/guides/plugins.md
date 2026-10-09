---
title: Plugins
description: Install, manage, and configure plugins in TTT.
---

TTT supports Lua plugins that add panels, commands, and keybindings to the editor. Plugins run in a sandboxed Lua VM with a permission system — no plugin can access your filesystem, run commands, or make network requests without your explicit approval.

## Vim Mode

If you prefer modal editing, the [Vim Mode](https://github.com/eugenioenko/ttt-vim) plugin adds a Vim compatibility layer on top of TTT: Normal, Insert and Visual modes, the operator/motion/text-object grammar, registers, marks, macros, dot-repeat, a `:` command line and `/` search. It is a single Lua plugin written against the public plugin API (key interception, status bar segments, multi cursor, undo grouping).

Because it depends on plugin API additions from ttt 1.1.0 (the key interceptor's precedence over Escape, needed to leave Insert mode, and the command-line API for `:` and `/`), it requires ttt 1.1.0 or newer. Its manifest declares `api: 2`, so an older editor declines to load it instead of running it half-broken.

Install it from the **Plugins** sidebar tab by searching for "vim", or run **Plugins: Install from URL** with `https://github.com/eugenioenko/ttt-vim`. Once loaded it starts in Normal mode and shows the current mode in the status bar.

## Installing Plugins

### From the Plugins Panel

1. Open the sidebar and click the **Plugins** tab (or run **Plugins: Show Panel** from the command palette)
2. Browse or search available plugins in the top section
3. Click a plugin to see its details, then click **Install**
4. An approval dialog shows the requested permissions — click **Allow** to load the plugin

### From URL

1. Open the command palette (**Ctrl+P**) and run **Plugins: Install from URL**
2. Enter a git repository URL (e.g. `https://github.com/eugenioenko/ttt-plugins`)
3. The plugin is cloned and an approval dialog appears

### Manual Installation

Clone or copy the plugin directory into `~/.config/ttt/plugins/`:

```sh
git clone https://github.com/author/my-plugin ~/.config/ttt/plugins/my-plugin
```

Restart TTT. If the plugin is new, you'll see an approval dialog.

## Managing Plugins

### Plugins Panel

The **Plugins** sidebar tab has two sections:

- **Available** — plugins fetched from the community registry. Search and click to view details or install.
- **Installed** — your installed plugins with status icons. Click to toggle enabled/disabled. Action buttons for update (↑) and uninstall (×).

The **⋮** menu in the Plugins panel header provides:

- **Install from URL** — install a plugin from a git repository
- **Refresh** — refresh the list of available plugins from the registry
- **Help** — information about the plugin system

### Enabling / Disabling

Click an installed plugin in the Plugins panel to toggle it. A green **●** means enabled, an empty **○** means disabled. The state persists across restarts. Disabled plugins retain their permissions and can be re-enabled without re-approval.

### Updating

Click the **↑** button next to an installed plugin, or run **Plugins: Update** from the command palette. This runs `git pull` in the plugin directory. If the updated plugin requests new permissions, you'll be prompted to re-approve.

### Uninstalling

Click the **×** button next to an installed plugin, or run **Plugins: Uninstall** from the command palette. This removes the plugin directory and its registry entry.

### Reloading

Run **Plugins: Reload** from the command palette to reload a plugin without restarting TTT. This is useful during plugin development — edit your Lua file, reload, and see the changes immediately.

## Plugin Locations

Plugins can live in two locations:

- **Global:** `~/.config/ttt/plugins/<plugin-name>/` — available in all sessions
- **Workspace-local:** `<workspace-root>/plugins/<plugin-name>/` — scoped to a specific project

If a plugin with the same name exists in both locations, the global one takes precedence.

## Permissions

Plugins declare their required permissions in their manifest file. When you install a plugin, TTT shows a dialog listing each permission. Once approved, the permissions are saved and the plugin won't ask again unless it requests new ones.

Available permissions include:

| Permission | Description |
|------------|-------------|
| `panel.sidebar` | Add a panel to the sidebar |
| `panel.bottom` | Add a tab to the bottom panel |
| `panel.drawer` | Open drawer panels |
| `panel.editor` | Open custom editor tabs |
| `commands` | Register commands in the command palette |
| `keybindings` | Bind keyboard shortcuts |
| `editor.read` | Read editor buffer contents |
| `editor.write` | Modify editor buffers |
| `fs.read` | Read files and directories |
| `fs.write` | Write files |
| `system.exec` | Execute specific system commands |
| `system.env` | Read environment variables |
| `network.http` | Make HTTP requests (`true` for any host, or a list of allowed hostnames) |
| `events.file` | Listen for file open/close/save events |
| `events.editor` | Listen for buffer and cursor changes |
| `settings` | Read/write editor settings |
| `settings_keys` | Which settings keys the plugin can access (e.g. `formatters.*`) |

## Disabling the Plugin System

To completely disable the plugin system, add this to your `settings.json`:

```json
{
  "plugins": {
    "enabled": false
  }
}
```

When disabled, no plugins are loaded, the Plugins sidebar tab is hidden, and plugin commands are not registered. The `--plugin` debug flag still works regardless of this setting.

## Available Plugins

Community plugins are maintained in the [ttt-plugins](https://github.com/eugenioenko/ttt-plugins) repository:

| Plugin | Description |
|--------|-------------|
| color-picker | Color picker with hex/RGB swatches |
| docker-manager | Docker container, image, and volume management |
| go-test-runner | Run Go tests and view results |
| http-client | HTTP request client for testing APIs |
| json-viewer | Interactive JSON tree viewer |
| markdown-preview | Markdown preview panel |
| notepad | Persistent scratchpad for quick notes |
| todo-scanner | Scan workspace for TODO/FIXME/HACK/NOTE comments |

## Creating Plugins

See the [Plugin Authoring Guide](/guides/plugin-authoring/) for the full API reference covering the widget system, editor API, filesystem API, system commands, networking, events, and more.

### Adding to the Community Registry

To list your plugin in TTT's built-in plugin browser:

1. Create a git repository for your plugin (or add it to an existing plugins repo with a `path` field)
2. Ensure your plugin has a valid `plugin.ttt.json` manifest
3. Open a pull request to [eugenioenko/ttt](https://github.com/eugenioenko/ttt) adding an entry to `community-plugins.json`:

```json
{
  "name": "my-plugin",
  "author": "your-name",
  "description": "Short description of what it does",
  "repo": "https://github.com/your-name/ttt-my-plugin",
  "version": "0.1.0",
  "tags": ["relevant", "search", "tags"]
}
```

If your plugin lives in a subdirectory of a monorepo, add a `path` field:

```json
{
  "name": "my-plugin",
  "repo": "https://github.com/your-name/ttt-plugins",
  "path": "my-plugin",
  ...
}
```

Registry fields:

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes | Plugin name (must match the directory name and manifest name) |
| `author` | yes | Author name |
| `description` | yes | Short description shown in the plugin browser |
| `repo` | yes | Git repository URL |
| `version` | no | Current version string |
| `tags` | no | Array of search tags for discoverability |
| `path` | no | Subdirectory path within the repo (for monorepos) |
