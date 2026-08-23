// Repro tests for confirmed bugs from audit/2026-07-12-ux-bug-audit.md (branch audit/bug-hunt).
// Each test asserts the CORRECT behavior and is declared with `it.fails`,
// so it passes while the bug exists and goes red the moment the bug is
// fixed — at that point remove the `.fails` marker and the audit entry.
import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("BUG-010: search matches go stale after buffer edits", () => {
  it.fails("find-next after inserting a line lands on a matching line", () => {
    dir = createTempDir();
    const file = createTempFile(
      dir,
      "stale.txt",
      "alpha\nbeta\nalpha again\ndelta\nalpha alpha\nfoo bar\n",
    );

    tui.start(file);
    tui.waitFor("alpha");

    tui.press("ctrl+f");
    tui.type("alpha");
    // Click into the editor (find bar stays open but unfocused), then
    // insert a line above the matches so every match shifts down by one.
    tui.click(10, 4);
    tui.press("home");
    tui.type("NEWLINE");
    tui.press("enter");
    tui.press("f3"); // find next — must land on a line still containing "alpha"
    tui.press("home");
    tui.type("Z"); // marker: reveals which line the cursor is on

    tui.press("ctrl+s");
    tui.run();

    // Buggy behavior: matches are never recomputed, f3 jumps to the stale
    // line index, now "beta" — the marker produces "Zbeta".
    expect(readFile(file)).toMatch(/^Z.*alpha/m);
  });
});

describe("BUG-011: Replace All ignores case-sensitive toggle", () => {
  it("replace-all with case-sensitive on replaces only exact matches", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "case.txt", "Foo\nfoo\nFOO\nbar\n");

    tui.start(file);
    tui.waitFor("Foo");

    tui.press("ctrl+r");
    tui.type("Foo");
    tui.press("alt+c"); // case-sensitive on — bar shows 1/1
    tui.press("tab");
    tui.type("X");
    tui.press("alt+r"); // replace all
    tui.press("escape");

    tui.press("ctrl+s");
    tui.run();

    // Fixed: ReplaceAll now threads the bar's current Options (case/regex)
    // into FindInLines, so case-sensitive replaces only the exact match.
    expect(readFile(file)).toBe("X\nfoo\nFOO\nbar\n");
  });
});

describe("BUG-012: Replace All is not a single undo step", () => {
  it.fails("one undo fully reverts a replace-all", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "undoall.txt", "cat cat cat\ndog\ncat\n");

    tui.start(file);
    tui.waitFor("cat");

    tui.press("ctrl+r");
    tui.type("cat");
    tui.press("tab");
    tui.type("dog");
    tui.press("alt+r");
    tui.press("escape");
    tui.press("ctrl+z");

    tui.press("ctrl+s");
    tui.run();

    // Buggy behavior: each of the 4 replacements pushes 2 ungrouped undo
    // commands (8 undos to revert); one undo leaves " dog dog" — a state
    // the user never saw.
    expect(readFile(file)).toBe("cat cat cat\ndog\ncat\n");
  });
});

describe("BUG-013: stale search follows tab switch", () => {
  it.fails("find-next in a tab with no matches does not move the cursor", () => {
    dir = createTempDir();
    const fileC = createTempFile(
      dir,
      "tabC.txt",
      "one\ntwo\nthree\nfour\nfive\nalpha six\n",
    );
    const fileD = createTempFile(dir, "tabD.txt", "short\n");

    tui.start(fileC, fileD);
    tui.waitFor("short");

    tui.press("alt+,"); // to tabC
    tui.press("ctrl+f");
    tui.type("alpha");
    tui.press("alt+."); // to tabD — no "alpha" anywhere in it
    tui.press("f3"); // must be a no-op here
    tui.press("escape");
    tui.press("home");
    tui.type("Z");

    tui.press("ctrl+s");
    tui.run();

    // Buggy behavior: tabC's matches survive the switch, f3 clamps the
    // stale line-5 target into tabD and strands the cursor on line 1.
    expect(readFile(fileD)).toBe("Zshort\n");
  });
});

describe("BUG-014: replace bar swallows global keybindings", () => {
  it.fails("tab-switch works while the replace bar is open", () => {
    dir = createTempDir();
    const fileE = createTempFile(dir, "tabE.txt", "a\n");
    const fileF = createTempFile(dir, "tabF.txt", "b\n");

    tui.start(fileE, fileF);
    tui.waitFor("a");

    tui.press("alt+,"); // to tabE
    tui.press("ctrl+r");
    tui.press("alt+."); // should switch to tabF (works with the FIND bar)
    tui.press("escape");
    tui.press("home");
    tui.type("Z");

    tui.press("ctrl+s");
    tui.run();

    // Buggy behavior: ReplaceBarWidget.handleKey unconditionally returns
    // EventConsumed, so alt+. is eaten and the marker lands in tabE.
    expect(readFile(fileF)).toBe("Zb\n");
  });
});

describe("BUG-015: find does not seed query from selection", () => {
  it.fails("ctrl+f pre-fills the search box with the selected word", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "seed.txt", "hello world\n");

    tui.start(file);
    tui.waitFor("hello");

    tui.press("end");
    tui.press("shift+ctrl+left"); // select "world"
    tui.press("ctrl+f");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    // "world" selected, then ctrl+f: the query box should be pre-filled
    // (VS Code behavior), so the term appears twice on screen — once in
    // the buffer, once in the find bar. Buggy behavior shows the empty
    // "Search" placeholder instead.
    const count = (snapshots[s].match(/world/g) || []).length;
    expect(count).toBeGreaterThanOrEqual(2);
  });
});
