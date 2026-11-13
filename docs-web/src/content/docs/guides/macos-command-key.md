---
title: Command Key on macOS
description: Configure Ghostty, Kitty, and WezTerm to send Command-key shortcuts to TTT.
---

TTT 1.12.0 and later can use **Command (⌘)** for shortcuts such as save, select all, copy, and paste. Two settings need to agree: TTT must accept Command, and your terminal must send the key to TTT instead of handling it itself.

## Enable Command in TTT

Open **Settings → General → Keyboard → Shortcut modifier**, choose **Control or Super (Cmd)**, and click **Apply**. This takes effect immediately and lets you use either Ctrl or Cmd.

The equivalent setting in `settings.json` is:

```json
{
  "keyboard": {
    "modifierKey": "control+super"
  }
}
```

After editing the JSON file, run **Reload Settings**. Choose `"super"` if you want registered shortcuts to use Cmd instead of Ctrl; `"control"` is the default.

TTT calls Command **Super**. Shortcut labels show **Mod** when either Ctrl or Cmd is accepted, and **Super** in Cmd-only mode. Keep writing `ctrl+…` in TTT's `keybindings.json`; the modifier setting applies to those definitions automatically.

For chords, release the modifier before an unmodified second key: **Ctrl+K E** becomes **Cmd+K**, release, then **E**. See the [keyboard settings reference](/reference/settings/#keyboard) for the full behavior.

## Ghostty

To let TTT handle Command shortcuts, you can clear Ghostty's keybindings in one step. Add this to your [Ghostty configuration](https://ghostty.org/docs/config):

```ini
keybind = clear
```

Ghostty's [`keybind = clear`](https://ghostty.org/docs/config/reference#keybind) removes all bindings defined up to that point, including its defaults. Place it after any bindings you want removed. Any bindings defined afterward remain active, so put shortcuts you want Ghostty to keep after this line.

This also removes Ghostty's shortcuts for copy/paste, windows, tabs, quitting, and reloading configuration. Use Ghostty's menus for those actions, or add back the specific bindings you need. The change affects every program running in that Ghostty configuration. TTT still needs **Control or Super (Cmd)** or **Super (Cmd)** enabled as described above.

On macOS, supported configuration locations include `~/.config/ghostty/config` and `~/Library/Application Support/com.mitchellh.ghostty/config`; newer versions also use the filename `config.ghostty`. Edit the file you already use, then reload through Ghostty's menu or restart Ghostty after saving your work. After clearing bindings, the default **Cmd+Shift+,** reload shortcut is no longer available unless you add it back.

### Forward individual shortcuts instead

If you want to keep Ghostty's other shortcuts, remove the `keybind = clear` line and unbind only selected keys. For example, Ghostty normally uses **Cmd+A** for terminal selection. This setup was tested in [PR #1280](https://github.com/eugenioenko/ttt/pull/1280):

```ini
keybind = super+a=unbind
```

Reload Ghostty's configuration. With Command enabled in TTT, **Cmd+A** should now select the editor's entire document.

[`unbind`](https://ghostty.org/docs/config/keybind) removes Ghostty's action for a key so the key can reach the application. `ignore` discards the input instead. The `unconsumed:` prefix runs a terminal action while also sending the key, so it is not a replacement for removing an unwanted terminal action.

:::note[Suggested mappings — not tested with TTT in Ghostty]
The additional mappings below extend the Cmd+A setup. They were not part of the Ghostty testing in #1280. Add only the shortcuts you want to forward, reload Ghostty's configuration, and check each one in TTT.
:::

```ini
keybind = super+c=unbind
keybind = super+v=unbind
keybind = super+x=unbind
keybind = super+s=unbind
keybind = super+p=unbind
keybind = super+f=unbind
keybind = super+z=unbind
keybind = super+k=unbind
keybind = super+l=unbind
```

### Explicit forwarding fallback

If clearing or unbinding does not deliver the expected key, Ghostty can send an explicit sequence. For Cmd+A, add this after `keybind = clear`, or replace its individual `unbind` line:

```ini
keybind = super+a=csi:97;9u
```

Use one mapping per shortcut. This Cmd+A fallback was also tested in [PR #1280](https://github.com/eugenioenko/ttt/pull/1280); use the explicit mapping only if needed.

The [`csi` action](https://ghostty.org/docs/config/keybind/reference#csi) adds the escape-sequence prefix. In `97;9u`, `97` identifies `a` and `9` encodes Super/Command. The [sequence table below](#explicit-key-sequences) covers other common shortcuts.

## Kitty

:::note[Suggested configuration — not tested with TTT]
These examples follow Kitty's documentation. The configuration syntax was checked locally, but forwarding through Kitty to TTT has not been tested, including on macOS.
:::

Kitty can also release its own shortcuts to the application. Open `~/.config/kitty/kitty.conf` with **Cmd+,**, and add:

```text
map cmd+a no_op
map cmd+c no_op
map cmd+v no_op
map cmd+x no_op
map cmd+s no_op
map cmd+p no_op
map cmd+f no_op
map cmd+z no_op
map cmd+k no_op
map cmd+l no_op
```

Kitty's [`no_op` action](https://sw.kovidgoyal.net/kitty/actions/#action-no_op) removes a binding and passes the key to the program. `discard_event` drops the key instead.

Reload with **Ctrl+Cmd+,**. Changes to shortcuts attached to the macOS menu bar require a full Kitty restart. See [Kitty's configuration reference](https://sw.kovidgoyal.net/kitty/conf/#shortcut-kitty.Reload-kitty.conf).

If a key still needs explicit forwarding, replace that key's `no_op` line with a [`send_text` mapping](https://sw.kovidgoyal.net/kitty/conf/#shortcut-kitty.Send-arbitrary-text-on-key-presses). For Cmd+A:

```text
map cmd+a send_text all \x1b[97;9u
```

The `all` argument applies the mapping in every keyboard mode. `\x1b[` is the escape-sequence prefix; use the same suffixes as the table below.

## WezTerm

:::note[Suggested configuration — not tested with TTT]
These examples follow WezTerm's documentation. They have not been tested in WezTerm with TTT, including on macOS.
:::

WezTerm supports explicit forwarding through [`SendString`](https://wezterm.org/config/lua/keyassignment/SendString.html). Add these entries to the `config.keys` table in your [WezTerm configuration](https://wezterm.org/config/files.html), commonly `~/.wezterm.lua`:

```lua
{ key = 'a', mods = 'CMD', action = wezterm.action.SendString '\x1b[97;9u' },
{ key = 's', mods = 'CMD', action = wezterm.action.SendString '\x1b[115;9u' },
{ key = 'k', mods = 'CMD', action = wezterm.action.SendString '\x1b[107;9u' },
```

These are entries for an existing configuration that imports `local wezterm = require 'wezterm'`; keep your other key mappings. Reload the configuration or restart WezTerm after saving your work. Add entries for the other keys you want using the sequence table.

WezTerm also provides [`DisableDefaultAssignment`](https://wezterm.org/config/lua/keyassignment/DisableDefaultAssignment.html) to release a default shortcut. For native extended-key forwarding, enable [`config.enable_kitty_keyboard = true`](https://wezterm.org/config/lua/config/enable_kitty_keyboard.html) and verify the result in TTT. The explicit `SendString` examples above send the specified sequences directly.

## Explicit key sequences

These sequences preserve Command as a distinct modifier. The suffix follows the [Kitty keyboard protocol](https://sw.kovidgoyal.net/kitty/keyboard-protocol/#modifiers); Ghostty's `csi:` adds the prefix, while Kitty's `send_text` and WezTerm's `SendString` need `\x1b[` before it.

| Shortcut | Default TTT action | Sequence suffix |
|----------|--------------------|-----------------|
| Cmd+A | Select all | `97;9u` |
| Cmd+C | Copy | `99;9u` |
| Cmd+V | Paste | `118;9u` |
| Cmd+X | Cut | `120;9u` |
| Cmd+S | Save | `115;9u` |
| Cmd+P | Command palette | `112;9u` |
| Cmd+F | Find | `102;9u` |
| Cmd+Z | Undo | `122;9u` |
| Cmd+K | Start a Ctrl+K chord | `107;9u` |
| Cmd+L | Start a Ctrl+L chord | `108;9u` |

For example, Ghostty's Cmd+S mapping is `keybind = super+s=csi:115;9u`.

## Choose which shortcuts to forward

Terminal configuration applies to other programs running in that terminal too. Forward only the shortcuts you want TTT to own. Forwarding Cmd+C and Cmd+V moves copy/paste handling from the outer terminal to the running application; forwarding Cmd+K or Cmd+L replaces terminal actions such as clearing the screen or scrollback.

The individual-key examples leave **Cmd+Q**, **Cmd+W**, **Cmd+T**, and **Cmd+N** with the terminal. Ghostty's `keybind = clear` removes those bindings too. Keep the terminal's menus available or define replacement bindings after `clear` for the terminal actions you need.

These mappings configure the terminal that runs TTT. TTT's **Forward shortcuts to terminal** setting controls the next step: whether TTT sends its reserved shortcuts to its own integrated terminal. See [integrated terminal shortcut forwarding](/guides/terminal/#shortcut-forwarding).

## Check the setup

Start TTT directly in the configured terminal first, outside tmux or another multiplexer. With the editor focused:

1. Open a scratch file, type a few words, and press **Cmd+A**. The editor text should be selected, without selecting the terminal's surrounding cells.
2. If you also forwarded **Cmd+P**, press it. TTT's command palette should open. Press Escape to close it.
3. If you also forwarded **Cmd+K**, press it, release it, then press **E**. The File Explorer should open.

If a shortcut still triggers a terminal action, check for a conflicting or later terminal binding and reload its configuration. If it reaches TTT but does nothing, check **Shortcut modifier** and your custom TTT bindings. If it works directly but fails inside a multiplexer, that layer also needs to preserve the extended key sequences.

In Kitty, `kitten show-key -m kitty` displays the extended key events received from the terminal. See [Kitty's keyboard debugging guide](https://sw.kovidgoyal.net/kitty/mapping/#debugging-mapping-issues).

A terminal that cannot report Command distinctly may still let you map a Cmd shortcut to the corresponding Ctrl input. In that case TTT receives Ctrl, so use `"control"` or `"control+super"`; it cannot distinguish the remapped key from physical Ctrl. Keyboard forwarding does not also configure Cmd+click mouse reporting.
