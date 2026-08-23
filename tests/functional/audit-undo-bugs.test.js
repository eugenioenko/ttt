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

describe("BUG-020: undo of line commands does not restore the cursor", () => {
  it.fails("undo after Delete Line returns the cursor to the edit site", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "cur.txt", "A\nB\nC\nD\nE\n");

    tui.start(file);
    tui.waitFor("A");

    tui.press("arrow_down");
    tui.press("arrow_down");
    tui.exec("Delete Line"); // deletes "C" at line 2
    tui.press("arrow_down");
    tui.press("arrow_down"); // wander away
    tui.press("ctrl+z");
    tui.type("Z"); // marker: reveals where the cursor is after undo

    tui.press("ctrl+s");
    tui.run();

    // Correct: cursor returns to the restored line ("C"), like it does for
    // typed text/paste. Buggy: cursorAfterUndo has no case for line
    // commands, cursor stays where it wandered ("E" gets the marker).
    expect(readFile(file)).toBe("A\nB\nZC\nD\nE\n");
  });
});

describe("BUG-021: multi-line indent is not one undo step", () => {
  it("one undo reverts a selection indent completely", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "atom.txt", "one\ntwo\nthree\n");

    tui.start(file);
    tui.waitFor("one");

    tui.press("shift+down");
    tui.press("shift+down");
    tui.press("tab"); // indents all three lines (BUG-002 makes it three)
    tui.press("ctrl+z");

    tui.press("ctrl+s");
    tui.run();

    // Buggy: each line's indent is a separate undo command (no
    // BatchCommand, unlike Toggle Line Comment) — one undo only
    // un-indents the last line.
    expect(readFile(file)).toBe("one\ntwo\nthree\n");
  });
});

describe("BUG-022: Enter with auto-indent is not one undo step", () => {
  it("one undo fully reverts an auto-indented line split", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "enter.go", "func foo() {\n\tx := 1\n}\n");

    tui.start(file);
    tui.waitFor("foo");

    tui.press("arrow_down");
    tui.press("end");
    tui.press("enter"); // split + auto-indent insert (2 undo commands)
    tui.press("ctrl+z");

    tui.press("ctrl+s");
    tui.run();

    // Buggy: first undo removes only the auto-indent whitespace, leaving
    // a stray blank line; bracket-pair Enter needs 4 undos.
    expect(readFile(file)).toBe("func foo() {\n\tx := 1\n}\n");
  });
});

describe("BUG-023: viewport does not follow an off-screen undo", () => {
  it.fails("undo scrolls the restored cursor line into view", () => {
    dir = createTempDir();
    const lines = Array.from({ length: 100 }, (_, i) => `L${i + 1}`).join("\n");
    const file = createTempFile(dir, "far.txt", lines + "\n");

    tui.start(file);
    tui.waitFor("L1");

    tui.press("ctrl+g");
    tui.type("90");
    tui.press("enter");
    tui.type("X"); // edit at line 90
    tui.press("ctrl+g");
    tui.type("1");
    tui.press("enter"); // back to the top
    tui.press("ctrl+z"); // undo the far-away edit
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    // Correct: cursor moves to line 90 AND the viewport scrolls there
    // (Undo()/Redo() should call scrollViewport() like other cursor-moving
    // paths). Buggy: cursor.line becomes 89 but top_line stays 0.
    expect(snapshots[s]).toContain("L90");
  });
});

describe("BUG-024: undo on a folded header line silently unfolds it", () => {
  it.fails("fold stays collapsed after undoing an edit on its header", () => {
    dir = createTempDir();
    const file = createTempFile(
      dir,
      "fold.go",
      "package main\n\nfunc foo() {\n\ta := 1\n\tb := 2\n}\n",
    );

    tui.start(file);
    tui.waitFor("foo");

    tui.press("arrow_down");
    tui.press("arrow_down"); // on "func foo() {"
    tui.pressChord("ctrl+k", "["); // fold
    tui.type("X");
    tui.press("ctrl+z"); // buffer now byte-identical to pre-edit
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    // Correct: the fold marker (⋯) survives an undo that restores the
    // exact pre-edit text. Buggy: the region is silently expanded.
    expect(snapshots[s]).toContain("⋯");
  });
});
