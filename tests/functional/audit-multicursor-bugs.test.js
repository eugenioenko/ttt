// Repro tests for confirmed bugs from audit/2026-07-12-ux-bug-audit.md (branch audit/bug-hunt).
// Each test asserts the CORRECT behavior and is declared with `it.fails`,
// so it passes while the bug exists and goes red the moment the bug is
// fixed — at that point remove the `.fails` marker and the audit entry.
//
// Common root cause (BUG-005..008): the multiExec* keyboard paths keep
// e.Multi.Cursors consistent, but line commands, transforms, paste, and
// undo operate only on the primary cursor/selection and never touch
// e.Multi — leaving stale cursors that corrupt the buffer on the next
// multicursor keystroke.
import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

// 4 occurrences of "foo": two on line0, one each on line1/line2
const FOO_LINES = "foo bar foo baz\nfoo qux\nbar foo end\n";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("BUG-006: case transforms under multicursor only affect primary cursor", () => {
  it.fails("Transform to Uppercase applies to all selected occurrences", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "upper.txt", FOO_LINES);

    tui.start(file);
    tui.waitFor("foo");

    tui.pressChord("ctrl+k", "l");
    tui.exec("Transform to Uppercase");

    tui.press("ctrl+s");
    tui.run();

    // Buggy behavior uppercases only the first occurrence while the
    // status bar still reports "(4 cursors)".
    expect(readFile(file)).toBe("FOO bar FOO baz\nFOO qux\nbar FOO end\n");
  });
});

describe("BUG-007: paste under multicursor only replaces the primary selection", () => {
  it.fails("paste applies at every cursor's selection", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "paste.txt", FOO_LINES);

    tui.start(file);
    tui.waitFor("foo");

    // Copy "bar" (chars 4-6 of line0), then select all "foo" and paste
    for (let i = 0; i < 4; i++) tui.press("right");
    for (let i = 0; i < 3; i++) tui.press("shift+right");
    tui.press("ctrl+c");
    tui.press("home");
    tui.pressChord("ctrl+k", "l");
    tui.press("ctrl+v");

    tui.press("ctrl+s");
    tui.run();

    // Buggy behavior replaces only the first "foo".
    expect(readFile(file)).toBe("bar bar bar baz\nbar qux\nbar bar end\n");
  });
});

describe("BUG-008: undo under multicursor collapses instead of corrupting", () => {
  it("a keystroke after undo inserts exactly one character", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "undo.txt", FOO_LINES);

    tui.start(file);
    tui.waitFor("foo");

    tui.pressChord("ctrl+k", "l");
    tui.type("X"); // replaces all 4 occurrences
    tui.press("ctrl+z"); // collapses multicursor, then undoes the edit
    tui.type("Z");

    tui.press("ctrl+s");
    tui.run();

    // Undo/redo has no concept of e.Multi, so it collapses to a single
    // cursor first. Text is restored and exactly one "Z" is inserted —
    // never the old "fZoZo end" double-write from stale secondary cursors.
    const out = readFile(file);
    expect(out.match(/Z/g).length).toBe(1);
    expect(out).not.toMatch(/f[^o]oo|fo[^o]o/); // no Z spliced inside "foo"
    expect(out).toContain("foo bar foo baz");
    expect(out).toContain("foo qux");
  });
});

describe("BUG-005: line commands under multicursor no longer corrupt the buffer", () => {
  it("Move Line Down carries every cursor with its line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "move.txt", "aaa\nfoo\nfoo\nfoo\nbbb\n");

    tui.start(file);
    tui.waitFor("aaa");

    tui.press("down"); // cursor onto the first "foo"
    tui.pressChord("ctrl+k", "l"); // 3 cursors, one per "foo"
    tui.press("alt+down"); // move the three "foo" lines down as a block
    tui.type("Y"); // replaces each still-selected "foo"

    tui.press("ctrl+s");
    tui.run();

    expect(readFile(file)).toBe("aaa\nbbb\nY\nY\nY\n");
  });

  it("Duplicate Line collapses multicursor instead of corrupting", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "dup.txt", FOO_LINES);

    tui.start(file);
    tui.waitFor("foo");

    tui.pressChord("ctrl+k", "l"); // Select All Occurrences (4 cursors)
    tui.exec("Duplicate Line");
    tui.type("Y");

    tui.press("ctrl+s");
    tui.run();

    // Collapses to the primary cursor: line 0 is duplicated and a single
    // "Y" is typed — no character lands in text no cursor covered.
    expect(readFile(file)).toBe(
      "foo bar foo baz\nfooY bar foo baz\nfoo qux\nbar foo end\n",
    );
  });
});
