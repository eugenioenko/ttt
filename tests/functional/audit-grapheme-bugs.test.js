// Repro test for confirmed bug from audit/2026-07-12-ux-bug-audit.md (branch audit/bug-hunt).
// Asserts the CORRECT behavior with `it.fails` — passes while the bug
// exists, goes red when fixed. Remove `.fails` + audit entry when fixing.
import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("BUG-009: cursor and backspace split ZWJ grapheme clusters", () => {
  it.fails("backspace after crossing an emoji deletes the whole cluster", () => {
    dir = createTempDir();
    // Family emoji = 7 runes: MAN ZWJ WOMAN ZWJ GIRL ZWJ BOY
    const file = createTempFile(dir, "zwj.txt", "a👨‍👩‍👧‍👦b\n");

    tui.start(file);
    tui.waitFor("a");

    // Grapheme-atomic movement: right lands after "a", the next right
    // crosses the ENTIRE family emoji; backspace then removes it whole.
    tui.press("arrow_right");
    tui.press("arrow_right");
    tui.press("backspace");

    tui.press("ctrl+s");
    tui.run();

    // Buggy behavior: the second right stops mid-cluster (rune col 2) and
    // backspace deletes only the MAN rune, leaving a dangling ZWJ — the
    // emoji renders exploded ("a 👩 👧 👦b").
    expect(readFile(file)).toBe("ab\n");
  });
});
