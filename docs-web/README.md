# Documentation and playground

With Go (the version in `go.mod`), Node (the version in `.nvmrc`), and pnpm installed:

```sh
pnpm install --frozen-lockfile
pnpm dev
```

Open `/demo/` to try the editor. `pnpm build` generates the static site, including the playground. `pnpm test:playground` checks the browser filesystem bridge.

The terminal uses bundled JetBrains Mono 2.304 at 14px. Fonts load before terminal measurement. The grid is centered and capped at 120 columns by 50 rows, with a smaller grid on narrow or short screens. The font's OFL license is included in `public/fonts/jetbrains-mono/`.

The playground builds `cmd/ttt-web` for `js/wasm`, reuses tcell's browser frontend, and seeds a ZenFS in-memory filesystem from tracked text files in the repository. Generated assets live in the ignored `public/playground/` directory. GitHub Pages builds a fresh source snapshot on each push to main.

Edits and settings live only in the iframe's memory. Reloading discards them. Git, workspace search with ripgrep, language servers, plugins, and the integrated shell are unavailable; supported editing commands share the desktop implementation. F6 moves keyboard focus back to the docs navigation.

The build includes tcell and Ghostty Web license notices. Unmodified ZenFS is bundled under its [LGPL license](https://www.gnu.org/licenses/lgpl-3.0.html) and [web application exception](https://github.com/zen-fs/core/blob/v2.7.7/COPYING.md); source and license links appear in the loading screen.
