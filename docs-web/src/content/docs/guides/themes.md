---
title: Themes
description: Customizing the look and feel of TTT.
---

TTT supports fully customizable themes via JSON files. You can change every color in the editor, from syntax highlighting and diff backgrounds to the sidebar, tabs, status bar, terminal ANSI colors, borders, and semantic colors.

## Built-in Themes

TTT ships with a collection of built-in themes:

- Aurora
- Bubblegum
- Default Dark
- Default Light
- Dracula
- High Contrast Dark
- High Contrast Light
- Hotline
- Monokai
- Nord
- One Dark
- Rainy Day
- Rainy Day Dark
- Solarized Dark
- Solarized Light
- Turbo Vision
- Virtru Dark
- Zenith

## Switching Themes

Use **View > Switch Theme** from the menu bar, or search for **Switch Theme** in the command palette (**Ctrl+P**), to open the theme picker. The picker shows a live preview of each theme as you navigate the list, so you can try them out before committing to one.

## Customizing

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

## Theme Files

Theme files are stored as `<name>.json` in the `themes/` subdirectory of your config directory (`~/.config/ttt/themes/`). The filename (without `.json`) is the theme name used in `settings.json` and the theme picker.

## Full Theme Reference

Below is a complete theme file showing every configurable section. Ordinary color values are hex strings (`"#rrggbb"`); `editor.bracketColors` also accepts the named terminal colors documented below. Optional `"bold"` and `"italic"` flags are supported where noted. Any field left empty (`{}`) inherits from the `default` colors, except `diff.collapsedHover.bg`, which inherits from `editor.activeLine.bg`. `diff.collapsedEmphasis` defaults to the normal theme colors with bold text.

```json
{
  "default": {
    "fg": "#f8f8f2",
    "bg": "#272822"
  },
  "success": {
    "fg": "#a6e22e"
  },
  "danger": {
    "fg": "#f92672"
  },
  "warning": {
    "fg": "#e6db74"
  },
  "conflict": {
    "fg": "#c586c0"
  },
  "border": {
    "fg": "#75715e"
  },
  "statusBar": {},
  "tabs": {
    "active": {
      "fg": "#f8f8f2",
      "bg": "#3e3d32",
      "bold": true
    },
    "inactive": {
      "fg": "#75715e"
    }
  },
  "sidebar": {
    "header": {
      "fg": "#f8f8f2",
      "bold": true
    },
    "item": {},
    "selected": {
      "fg": "#f8f8f2",
      "bg": "#3e3d32"
    }
  },
  "dialog": {
    "input": {},
    "item": {},
    "selected": {
      "fg": "#f8f8f2",
      "bg": "#3e3d32"
    },
    "muted": {
      "fg": "#75715e"
    }
  },
  "editor": {
    "lineNumber": {
      "fg": "#75715e"
    },
    "activeLine": {
      "bg": "#3e3d32"
    },
    "selection": {
      "bg": "#49483e"
    },
    "searchMatch": {
      "bg": "#e6db74",
      "fg": "#272822"
    },
    "searchActive": {
      "bg": "#f92672",
      "fg": "#f8f8f2"
    },
    "bracketColors": ["yellow", "magenta", "blue"]
  },
  "menu": {
    "item": {},
    "active": {
      "fg": "#f8f8f2",
      "bg": "#3e3d32",
      "bold": true
    }
  },
  "diff": {
    "added": {
      "bg": "#1e2e1e"
    },
    "deleted": {
      "bg": "#3b1e1e"
    },
    "modified": {
      "bg": "#2e2e1a"
    },
    "collapsedEmphasis": {
      "bold": true
    },
    "collapsedHover": {
      "bg": "#282828"
    }
  },
  "scrollbar": {
    "fg": "#75715e",
    "bg": "#333428"
  },
  "syntax": {
    "comment": {
      "fg": "#75715e"
    },
    "string": {
      "fg": "#e6db74"
    },
    "keyword": {
      "fg": "#f92672"
    },
    "number": {
      "fg": "#ae81ff"
    },
    "operator": {
      "fg": "#f92672"
    },
    "function": {
      "fg": "#a6e22e"
    },
    "type": {
      "fg": "#66d9ef"
    },
    "builtin": {
      "fg": "#66d9ef"
    },
    "variable": {
      "fg": "#f8f8f2"
    },
    "punctuation": {
      "fg": "#f8f8f2"
    },
    "tag": {
      "fg": "#f92672"
    },
    "attribute": {
      "fg": "#a6e22e"
    }
  },
  "fileIcons": {
    "red": { "fg": "#f92672" },
    "yellow": { "fg": "#e6db74" },
    "green": { "fg": "#a6e22e" },
    "cyan": { "fg": "#a1efe4" },
    "blue": { "fg": "#66d9ef" },
    "magenta": { "fg": "#ae81ff" }
  },
  "terminal": {
    "black": "#272822",
    "red": "#f92672",
    "green": "#a6e22e",
    "yellow": "#e6db74",
    "blue": "#66d9ef",
    "magenta": "#ae81ff",
    "cyan": "#a1efe4",
    "white": "#f8f8f2",
    "brightBlack": "#75715e",
    "brightRed": "#f44747",
    "brightGreen": "#b6e354",
    "brightYellow": "#eae07e",
    "brightBlue": "#78dce8",
    "brightMagenta": "#c0a0ff",
    "brightCyan": "#a4f4e8",
    "brightWhite": "#f9f8f5",
    "selection": "#49483e"
  },
  "borders": {
    "horizontal": "─",
    "vertical": "│",
    "topLeft": "┌",
    "topRight": "┐",
    "bottomLeft": "└",
    "bottomRight": "┘",
    "topTee": "┬",
    "bottomTee": "┴",
    "leftTee": "├",
    "rightTee": "┤"
  }
}
```

### Syntax styles

The `syntax` section has twelve base styles: `comment`, `string`, `keyword`, `number`, `operator`, `function`, `type`, `builtin`, `variable`, `punctuation`, `tag`, and `attribute`.

Highlighting comes from VS Code's TextMate grammars, which distinguish more than that. The finer styles below are optional: a style the theme leaves out uses the style in the last column, so a theme only sets the ones it wants to distinguish. A style the theme does set is used exactly as written.

| Key | Colors | When unset |
|-----|--------|------------|
| `regexp` | Regular expression literals | `string` |
| `heading` | Markdown headings | `keyword`, bold |
| `bold` | Bold markup | default text, bold |
| `italic` | Italic markup | default text, italic |
| `quote` | Block quotes | `comment` |
| `inserted` | Inserted lines in diffs | `diff.added` |
| `deleted` | Deleted lines in diffs | `diff.deleted` |
| `invalid` | Invalid or illegal code, as marked by the grammar | `danger` |
| `control` | Control-flow keywords: `if`, `for`, `return` | `keyword` |
| `storage` | Declaration keywords and modifiers: `const`, `class`, `public` | `keyword` |
| `constant` | Language constants: `true`, `null` | `keyword` |
| `escape` | Escape sequences in strings: `\n`, `\t` | `string` |
| `parameter` | Function parameters | `variable` |
| `property` | Object properties | `variable` |
| `self` | `this`, `self` | `keyword` |
| `namespace` | Namespaces and modules | `type` |
| `decorator` | Decorators and annotations | `function` |
| `link` | Links in markup | `string` |
| `code` | Inline code in markup | `string` |
| `interpolation` | Template interpolation delimiters: `${` `}` | `punctuation` |

### Section Reference

| Section | Description |
|---------|-------------|
| `default` | Base foreground and background colors inherited by all other sections |
| `success`, `danger`, `warning` | Semantic colors used for status indicators and messages |
| `conflict` | Color for merge-conflicted files, e.g. in the explorer's git status decoration |
| `border` | Color for UI borders and dividers |
| `statusBar` | Status bar at the bottom of the editor |
| `tabs` | Active and inactive editor tab colors |
| `sidebar` | File explorer sidebar: section headers, items, and selected item |
| `dialog` | Command palette and dialog boxes: input field, items, selection, muted text |
| `editor` | Editor pane: line numbers, active line highlight, selection, search matches, and bracket pair colors. `bracketColors` accepts terminal color names (`yellow`, `magenta`, `cyan`, `red`, `green`, `blue`, `black`, `white`, `brightRed`, `brightGreen`, `brightYellow`, `brightBlue`, `brightMagenta`, `brightCyan`, `brightBlack`, `brightWhite`), syntax style names (any key of the `syntax` section, e.g. `keyword`, `function`, `type`, `string`), or hex colors (`#rrggbb`). Up to 6 colors cycle by nesting depth. |
| `menu` | Menu bar dropdown items and active/hovered item |
| `diff` | Diff presentation styles: `added`, `deleted`, and `modified` backgrounds; `gutterAdded`, `gutterDeleted`, and `gutterModified` semantic foregrounds; `collapsedEmphasis` for opt-in emphasized idle rows; and the `collapsedHover` accent. Emphasis defaults to contrast-safe normal theme colors with bold text, while an omitted `collapsedHover` background inherits the editor active-line background. The legacy `collapsed` field remains accepted only as a `collapsedHover` migration alias. |
| `scrollbar` | Scrollbar thumb (`fg`) and track (`bg`) colors |
| `syntax` | Syntax highlighting colors for language tokens. See [Syntax styles](#syntax-styles). |
| `fileIcons` | File icon colors in the Explorer and Changes panel, by hue family (`red`, `yellow`, `green`, `cyan`, `blue`, `magenta`). Each entry defaults to the matching `terminal` color, so most themes need no `fileIcons` section. Neutral icons use the row's normal text color |
| `terminal` | ANSI color palette for the integrated terminal (16 colors), plus `selection`, the highlight background for selected terminal text. `selection` inherits `editor.selection.bg` when omitted. |
| `borders` | Unicode characters used for drawing box borders. Overridden when `borderStyle` in settings is set to a named preset (e.g. `"rounded"`, `"double"`). Use `"default"` or `"theme"` to respect the theme's borders. |
