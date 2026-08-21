---
title: Testing Plugins
description: Drive plugins headlessly with --plugin and --exec to test them like a browser end-to-end harness.
---

TTT ships a scripted-interaction harness that lets you load a plugin, click and type through it, and read back both the rendered screen and the editor's internal state — all headless, in a single command, no real terminal required. It's the fastest way to develop and regression-test a plugin, and it's how the built-in plugins are tested.

## The flags

- **`--plugin FILE`** loads a single Lua file as a plugin on startup, granted **all permissions** (no approval dialog). Use it to iterate on a plugin without installing it. `ttt.plugin_dir()` resolves to the file's directory.
- **`--exec "commands"`** runs a semicolon-separated script after startup, then the editor exits. Combine with `--size WxH` for deterministic layout.
- **`--listen`** starts an HTTP server on `127.0.0.1:4242` instead — `POST /exec` accepts the same script format, run synchronously against an **already-running** editor. Useful when a bug only reproduces from live, organic interaction and you want to capture a screenshot or debug dump at the moment it happens instead of scripting the repro in advance. See [Driving a running editor with `--listen`](#driving-a-running-editor-with---listen) below.

```sh
bin/ttt --size 100x30 --plugin ./my-plugin/init.lua README.md \
  --exec "wait 300; panel plugin.init; wait 100; screenshot /tmp/out.txt; quit"
```

The plugin's name (used for its panel id, `plugin.<name>`) is the Lua file's basename without `.lua` — so `init.lua` registers as `plugin.init`.

## `--exec` commands

| Command | Description |
|---------|-------------|
| `wait MS` | Pause (let timers, async callbacks, and renders settle) |
| `panel ID` | Open a bottom-panel tab by id (e.g. `panel output`, `panel plugin.init`) |
| `key COMBO` | Press a key or chord (`key enter`, `key ctrl+k p`, `key tab`) |
| `type TEXT` | Type a string |
| `click X Y` | Click at screen coordinates |
| `hover X Y` | Move the mouse to coordinates |
| `drag X1 Y1 X2 Y2` | Drag between two points |
| `exec "Command Name"` | Run a command by its palette title |
| `screenshot PATH` | Write the current screen (plain text) to a file |
| `debug PATH` | Write the editor's full state as JSON to a file |
| `quit` / `shutdown` | Exit |

## Driving a running editor with `--listen`

`--exec` only runs its script once, at startup, then exits. `--listen` keeps the editor running and lets you POST the same commands to it at any point:

```sh
bin/ttt --listen &
curl -X POST --data "type hi; wait 100; screenshot /tmp/screen.txt" http://127.0.0.1:4242/exec
curl -X POST --data "shutdown" http://127.0.0.1:4242/exec
```

This is for bugs that only show up during real, organic interaction (typing, clicking, a live child process in the integrated terminal) — you drive the editor by hand until the bug happens, then fire a `debug`/`screenshot` command over `--listen` to capture it, instead of trying to script the repro blind. The server binds to `127.0.0.1` only. Pass `?sep=` to use a separator other than `;`, mirroring `--exec-split-on`.

## Reading results: screen vs. state

The harness gives you two windows into the editor, and the gap between them is where bugs hide.

**`screenshot`** is what the user sees — the rendered grid as text. Grep it for labels, widget content, dialog text.

**`debug`** is what actually happened — a JSON dump of cursor, selection, active tab, open panels, the widget tree, and the **OUTPUT log** (everything the plugin logged with `ttt.log`). This is your assertion channel: have your plugin `ttt.log` what it did, then read it back.

```sh
bin/ttt --size 100x30 --plugin ./init.lua file.txt \
  --exec "wait 300; panel plugin.init; key tab; key enter; wait 100; debug /tmp/state.json; quit"
```

```sh
# Pull the plugin's log lines out of the dump
python3 -c "import json; print([l for l in json.load(open('/tmp/state.json'))['output']])"
```

A common pattern: the screenshot shows the panel *looks* unchanged, but the OUTPUT log shows a callback fired with the wrong data — that mismatch is the bug. (This is exactly how the widget system's scrollview mouse-routing bugs were found: clicks visually did nothing, but the debug dump proved events never reached the widgets.)

## Isolate from your real config

By default the editor reads and writes your real `~/.config/ttt` — settings toggles, installed plugins, keybindings. When scripting, set **`TTT_CONFIG_DIR`** to a throwaway directory so tests can't mutate your actual configuration and don't pick up your installed plugins:

```sh
TTT_CONFIG_DIR=/tmp/ttt-test bin/ttt --plugin ./init.lua --exec "..."
```

Always do this in any automated test — a scripted run of a settings-toggling command will otherwise persist into your real config.

## Testing a scoped plugin (with a manifest)

`--plugin` grants all permissions, so it can't exercise permission scoping. To test a plugin the way it will really run — with its manifest permissions — install it into an isolated config dir and pre-approve it in the registry:

```sh
CFG=/tmp/ttt-test
mkdir -p "$CFG/plugins"
cp -r ./my-plugin "$CFG/plugins/"
# pre-approve so it loads enabled without the permission dialog
cat > "$CFG/plugins.ttt.json" <<'JSON'
[{"name":"my-plugin","version":"","enabled":true,"permissions":{"network.http":["cheat.sh"]}}]
JSON

TTT_CONFIG_DIR="$CFG" bin/ttt --size 100x30 somefile.txt \
  --exec "wait 400; panel output; wait 100; screenshot /tmp/out.txt; quit"
```

The `permissions` block in the registry entry must match the plugin's manifest, or the editor will re-prompt for approval.

## Example: assert an interval timer fires

```lua
-- timers.lua
local ttt = require("ttt")
local ticks = 0
ttt.set_interval(100, function()
  ticks = ticks + 1
  ttt.log("info", "tick " .. ticks)
end)
```

```sh
bin/ttt --size 100x30 --plugin ./timers.lua file.txt \
  --exec "wait 550; panel output; wait 100; screenshot /tmp/out.txt; quit"
grep -c "tick" /tmp/out.txt   # ~5 ticks in 550ms
```

## Wiring it into a test runner

The functional test suite (`tests/functional/`) wraps this harness in a small JS helper (`tui.js`) that accumulates commands and runs them in one batch, then asserts on the returned screenshots. The pattern:

```js
tui.start("--plugin", pluginFile, file);
tui.waitFor("ready");
tui.wait(550);
tui.panel("output");
const s = tui.snapshot();
const { snapshots } = tui.run();
expect(snapshots[s]).toContain("tick 3");
```

`tui.js` sets `TTT_CONFIG_DIR` to a per-test temp directory automatically, so tests are isolated by default. See existing tests like `plugin-timers.test.js` for a complete example.

## Lua debug helpers

The same capture functions are available from Lua, so a plugin can screenshot or dump state from inside a callback:

- `ttt.screenshot(path)` — write the screen to a file
- `ttt.debug(path)` — write the state dump to a file
- `ttt.click(x, y)` / `ttt.drag(x1, y1, x2, y2)` — simulate input
- `ttt.quit()` — exit the editor
