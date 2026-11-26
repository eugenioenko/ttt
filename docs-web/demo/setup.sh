#!/usr/bin/env bash
# Generates a throwaway demo project for the ttt hero recording (demo.tape).
# Everything lives outside the ttt repo, in its own git repo, so recording
# mutates nothing tracked. Regenerate anytime; no cleanup needed.
#
#   bash docs-web/demo/setup.sh [target-dir] [config-dir]
set -euo pipefail
REPO="$(cd "$(dirname "$0")/../.." && pwd)"
DIR="${1:-/tmp/ttt-demo}"
CFG="${2:-/tmp/ttt-demo-cfg}"
rm -rf "$DIR"
mkdir -p "$DIR/src" "$DIR/scripts"

[[ "$(fc-list : family)" == *"Symbols Nerd Font Mono"* ]] || {
  echo "setup.sh: needs the Symbols Nerd Font Mono font installed for file icons" >&2
  exit 1
}
[[ "$(tsc --version 2>/dev/null)" == "Version 7"* ]] || {
  echo "setup.sh: needs TypeScript 7 (tsc --lsp) on PATH: npm install -g typescript@7" >&2
  exit 1
}

# Isolated config: keeps your real ~/.config/ttt untouched and makes the demo
# reproducible. VHS can't send ctrl+s, alt+arrows or function keys, and some
# commands have no default key, so the tape drives them with ctrl+l chords
# (merged over defaults).
rm -rf "$CFG"; mkdir -p "$CFG/themes"
cat > "$CFG/keybindings.json" <<'JSON'
[
  { "key": "ctrl+l s",     "command": "editor.splitDown" },
  { "key": "ctrl+l x",     "command": "editor.closeSplit" },
  { "key": "ctrl+l w",     "command": "file.save" },
  { "key": "ctrl+l a",     "command": "tab.closeAll" },
  { "key": "ctrl+l t",     "command": "terminal.newInEditor" },
  { "key": "ctrl+l q",     "command": "workspace.close" },
  { "key": "ctrl+l d",     "command": "editor.toggleDiff" },
  { "key": "ctrl+l e",     "command": "editor.goToLineEnd" },
  { "key": "alt+n",        "command": "sidebar.narrower" },
  { "key": "alt+w",        "command": "sidebar.wider" }
]
JSON
cat > "$CFG/settings.json" <<'JSON'
{
  "version": 1,
  "theme": "default-dark",
  "appearance": { "icons": "nerd-font" },
  "editor": { "lineNumbers": true, "borderStyle": "rounded" },
  "lsp": {
    "servers": {
      "typescript": {
        "command": ["tsc", "--lsp", "--stdio"],
        "languages": { ".ts": "typescript" }
      }
    }
  }
}
JSON
cat > "$CFG/state.json" <<'JSON'
{ "panelPosition": "right", "panelWidth": 56 }
JSON
cp "$REPO/config/themes/13-tokyo-neon.json" "$CFG/themes/"

# btop for the editor-terminal shot: CPU and memory only, so the recording
# never shows this machine's process list or network address.
mkdir -p "$CFG/xdg/btop"
cat > "$CFG/xdg/btop/btop.conf" <<'CONF'
shown_boxes = "cpu mem"
theme_background = False
update_ms = 500
CONF


cd "$DIR"

# Types are named Item/Tracker so the lowercase loop variable `task` is the
# ONLY case-insensitive match for a Find of "task": the multi-cursor shot in
# demo.tape lands cleanly on it (9 whole-word occurrences, no collisions).
cat > src/tasks.ts <<'TS'
import { Store } from "./store";

export interface Item {
  id: number;
  title: string;
  done: boolean;
}

export class Tracker {
  constructor(private store: Store<Item>) {}

  add(title: string): Item {
    const task = { id: Date.now(), title, done: false };
    this.store.save(task);
    return task;
  }

  complete(id: number): void {
    const task = this.store.get(id);
    if (task) {
      task.done = true;
      this.store.save(task);
    }
  }

  pending(): Item[] {
    return this.store.all().filter((task) => !task.done);
  }
}
TS

cat > src/store.ts <<'TS'
export class Store<T extends { id: number }> {
  private items = new Map<number, T>();

  save(item: T): void {
    this.items.set(item.id, item);
  }

  get(id: number): T | undefined {
    return this.items.get(id);
  }

  all(): T[] {
    return [...this.items.values()];
  }
}
TS

cat > src/index.ts <<'TS'
import { Store } from "./store";
import { Item, Tracker } from "./tasks";

const tracker = new Tracker(new Store<Item>());

tracker.add("Ship v1.0.0");
tracker.add("Record the demo");
const t = tracker.add("Write the docs");
tracker.complete(t.id);

for (const task of tracker.pending()) {
  console.log(`- [ ] ${task.title}`);
}
TS

cat > scripts/test.sh <<'SH'
#!/usr/bin/env bash
g="\033[32m"; d="\033[2m"; r="\033[0m"
for t in "adds a task" "completes a task" "lists pending tasks" \
         "ignores done tasks" "persists through the store"; do
  printf "  ${g}✓${r} ${d}%s${r}\n" "$t"; sleep 0.12
done
printf "\n  ${g}5 passed${r} ${d}(0.38s)${r}\n"
SH
chmod +x scripts/test.sh

cat > Makefile <<'MK'
test:
	@bash ./scripts/test.sh
MK

cat > tsconfig.json <<'JSON'
{
  "compilerOptions": { "target": "es2022", "module": "esnext", "strict": true },
  "include": ["src"]
}
JSON

cat > package.json <<'JSON'
{
  "name": "task-service",
  "version": "1.0.0",
  "scripts": { "test": "make test" }
}
JSON

cat > README.md <<'MD'
# task-service

A tiny task tracker, the demo project for **ttt**.

- `src/tasks.ts`: the service
- `src/store.ts`: in-memory store
- `make test`: run the suite
MD

git init -q -b main
git add -A
git -c user.email=demo@ttt.dev -c user.name=ttt commit -qm "initial commit" >/dev/null
echo "demo project ready at $DIR"
