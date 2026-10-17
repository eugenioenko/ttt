// Repro tests for confirmed bugs from audit/2026-07-12-ux-bug-audit.md (branch audit/bug-hunt).
// Tests for fixed bugs assert correct behavior; remaining audit bugs use `it.fails`.
import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("BUG-010: search matches go stale after buffer edits", () => {
  it("find-next after inserting a line lands on a matching line", () => {
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

  it("find-next after an edit with the find bar closed lands on a matching line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "closed-stale.txt", "one\ntwo\nfoo\nfour\nfive\nfoo\n");

    tui.start(file);
    tui.waitFor("foo");

    tui.press("ctrl+f");
    tui.type("foo");
    tui.press("escape");
    // Closing the bar clears highlights, so this F3 recomputes and caches the
    // matches. The following edit must not leave that cache in place.
    tui.press("f3");
    tui.press("ctrl+home");
    tui.press("enter");
    tui.press("enter");
    tui.press("f3");
    tui.press("home");
    tui.type("Z");

    tui.press("ctrl+s");
    tui.run();

    // Buggy behavior: the cached matches still point at the old lines, so
    // f3 lands on "one" and the marker produces "Zone".
    expect(readFile(file)).toMatch(/^Zfoo/m);
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
  it("find-next in a tab with no matches does not move the cursor", () => {
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
    const switched = tui.snapshot();
    tui.press("f3"); // must be a no-op here
    tui.press("escape");
    tui.press("home");
    tui.type("Z");

    tui.press("ctrl+s");
    const { snapshots } = tui.run();

    expect(snapshots[switched]).toContain("0/0");

    // Buggy behavior: tabC's matches survive the switch, f3 clamps the
    // stale line-5 target into tabD and strands the cursor on line 1.
    expect(readFile(fileD)).toBe("Zshort\n");
  });
});

describe("BUG-048: find-next uses active tab matches", () => {
  it("recomputes matches after switching tabs with find open", () => {
    dir = createTempDir();
    const fileA = createTempFile(dir, "tabA.txt", "alpha first\none\ntwo\nalpha second\n");
    const fileB = createTempFile(dir, "tabB.txt", "top\nalpha active\nbottom\n");

    tui.start(fileA, fileB);
    tui.waitFor("top");

    tui.press("alt+,"); // to fileA
    tui.press("ctrl+f");
    tui.type("alpha");
    tui.press("f3"); // select fileA's second match
    tui.press("alt+."); // to fileB; find remains open
    const switched = tui.snapshot();
    tui.click(10, 4); // focus the editor without dismissing find
    tui.press("f3"); // must use fileB's matches, not fileA's coordinates
    tui.press("home");
    tui.type("Z");

    tui.press("ctrl+s");
    const { snapshots } = tui.run();

    expect(snapshots[switched]).toContain("1/1");
    expect(readFile(fileB)).toBe("top\nZalpha active\nbottom\n");
  });

  it("continues to the next match after find closes on the same buffer", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "sameTab.txt", "alpha first\nmiddle\nalpha second\nbottom\nalpha third\n");

    tui.start(file);
    tui.waitFor("alpha first");

    tui.press("ctrl+f");
    tui.type("alpha");
    tui.press("f3");
    tui.press("escape");
    tui.press("f3");
    tui.press("home");
    tui.type("Z");

    tui.press("ctrl+s");
    tui.run();

    expect(readFile(file)).toBe("alpha first\nmiddle\nalpha second\nbottom\nZalpha third\n");
  });

  it("reuses the last query after the find bar closes", () => {
    dir = createTempDir();
    const fileA = createTempFile(dir, "closedA.txt", "alpha first\none\ntwo\nalpha second\n");
    const fileB = createTempFile(
      dir,
      "closedB.txt",
      "top\nalpha active first\nmiddle\nalpha active second\nbottom\nalpha active third\n",
    );

    tui.start(fileA, fileB);
    tui.waitFor("top");

    tui.press("alt+,"); // to fileA
    tui.press("ctrl+f");
    tui.type("alpha");
    tui.press("f3");
    tui.press("escape"); // close find, but keep its query for F3
    tui.press("alt+."); // to fileB
    tui.press("f3");
    tui.press("home");
    tui.type("Z");

    tui.press("ctrl+s");
    tui.run();

    expect(readFile(fileB)).toBe(
      "top\nZalpha active first\nmiddle\nalpha active second\nbottom\nalpha active third\n",
    );
  });

  it("starts at the last match for find-previous after switching tabs", () => {
    dir = createTempDir();
    const fileA = createTempFile(dir, "previousA.txt", "alpha one\nalpha two\n");
    const fileB = createTempFile(
      dir,
      "previousB.txt",
      "top\nalpha first\nmiddle\nalpha second\nbottom\nalpha third\n",
    );

    tui.start(fileA, fileB);
    tui.waitFor("top");

    tui.press("alt+,"); // to fileA
    tui.press("ctrl+f");
    tui.type("alpha");
    tui.press("f3");
    tui.press("escape");
    tui.press("alt+."); // to fileB
    tui.press("shift+f3");
    tui.press("home");
    tui.type("Z");

    tui.press("ctrl+s");
    tui.run();

    expect(readFile(fileB)).toBe("top\nalpha first\nmiddle\nalpha second\nbottom\nZalpha third\n");
  });

  it("preserves case-sensitive regex options after closing find", () => {
    dir = createTempDir();
    const fileA = createTempFile(dir, "optionsA.txt", "alpha first\nalpha second\n");
    const fileB = createTempFile(
      dir,
      "optionsB.txt",
      "Alpha upper\nalpha 123\nalpha match\nalpha next\n",
    );

    tui.start(fileA, fileB);
    tui.waitFor("Alpha upper");

    tui.press("alt+,"); // to fileA
    tui.press("ctrl+f");
    tui.press("alt+c"); // case-sensitive
    tui.press("alt+r"); // regex
    tui.type("^alpha [a-z]+$");
    tui.waitFor("1/2");
    tui.press("f3");
    tui.press("escape");
    tui.press("alt+."); // to fileB
    tui.press("f3");
    tui.press("home");
    tui.type("Z");

    tui.press("ctrl+s");
    tui.run();

    expect(readFile(fileB)).toBe("Alpha upper\nalpha 123\nZalpha match\nalpha next\n");
  });

  it("refreshes the find count when the active tab closes", () => {
    dir = createTempDir();
    const fileA = createTempFile(dir, "closeA.txt", "alpha one\nalpha two\n");
    const fileB = createTempFile(dir, "closeB.txt", "alpha active\n");

    tui.start(fileA, fileB);
    tui.waitFor("alpha active");

    tui.press("alt+,"); // to fileA
    tui.press("ctrl+f");
    tui.type("alpha");
    tui.waitFor("1/2");
    tui.press("ctrl+w"); // close fileA; fileB becomes active
    const switched = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[switched]).toContain("1/1");
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
