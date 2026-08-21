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

Below is a complete theme file showing every configurable section. All color values are hex strings (`"#rrggbb"`). Optional `"bold"` and `"italic"` flags are supported where noted. Any field left empty (`{}`) inherits from the `default` colors.

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
  "border": {
    "fg": "#75715e"
  },
  "statusBar": {},
  "commitMessage": {
    "fg": "#f8f8f2",
    "bg": "#34352d"
  },
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
    "collapsed": {
      "fg": "#272822",
      "bg": "#66d9ef",
      "bold": true
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
    "brightWhite": "#f9f8f5"
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

### Section Reference

| Section | Description |
|---------|-------------|
| `default` | Base foreground and background colors inherited by all other sections |
| `success`, `danger`, `warning` | Semantic colors used for status indicators and messages |
| `border` | Color for UI borders and dividers |
| `statusBar` | Status bar at the bottom of the editor |
| `commitMessage` | Commit message block in the commit detail view |
| `tabs` | Active and inactive editor tab colors |
| `sidebar` | File explorer sidebar: section headers, items, and selected item |
| `dialog` | Command palette and dialog boxes: input field, items, selection, muted text |
| `editor` | Editor pane: line numbers, active line highlight, selection, search matches, and bracket pair colors. `bracketColors` accepts terminal color names (`yellow`, `magenta`, `cyan`, `red`, `green`, `blue`, `black`, `white`, `brightRed`, `brightGreen`, `brightYellow`, `brightBlue`, `brightMagenta`, `brightCyan`, `brightBlack`, `brightWhite`), syntax style names (`keyword`, `function`, `type`, `comment`, `string`, `number`, `operator`, `builtin`, `variable`, `punctuation`, `tag`, `attribute`), or hex colors (`#rrggbb`). Up to 6 colors cycle by nesting depth. |
| `menu` | Menu bar dropdown items and active/hovered item |
| `diff` | Diff view colors for added, deleted, and modified lines, plus `collapsed` region separators |
| `scrollbar` | Scrollbar thumb (`fg`) and track (`bg`) colors |
| `syntax` | Syntax highlighting colors for language tokens |
| `terminal` | ANSI color palette for the integrated terminal (16 colors) |
| `borders` | Unicode characters used for drawing box borders. Overridden when `borderStyle` in settings is set to a named preset (e.g. `"rounded"`, `"double"`). Use `"default"` or `"theme"` to respect the theme's borders. |
