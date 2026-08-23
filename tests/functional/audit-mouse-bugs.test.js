// Repro tests for confirmed bugs from audit/2026-07-12-ux-bug-audit.md (branch audit/bug-hunt).
// Each test asserts the CORRECT behavior and is declared with `it.fails`,
// so it passes while the bug exists and goes red the moment the bug is
// fixed — at that point remove the `.fails` marker and the audit entry.
import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("BUG-018: clicking another menu header closes instead of switching", () => {
  it.fails("File menu open + click on View switches to the View menu", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "menu.txt", "hello\n");

    tui.start(file);
    tui.waitFor("hello");

    tui.click(4, 0); // open "File" menu
    tui.click(30, 0); // click "View" header while File is open
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    // Correct behavior: the View dropdown is now open ("Command Palette"
    // is its first item). Buggy behavior: the click only dismisses the
    // File menu and no menu is open.
    expect(snapshots[s]).toContain("Command Palette");
  });
});

describe("BUG-019: rightmost column of explorer tree rows is click-dead", () => {
  it.fails("clicking the last column of a file row opens the file", () => {
    dir = createTempDir();
    createTempFile(dir, "a.txt", "aaa\n");
    createTempFile(dir, "b.txt", "bbb\n");

    tui.start(dir);
    tui.exec("Show Explorer");

    // Tree widget rect is {x:1, w:30} → x=30 is the rightmost column of
    // the row rect; x=29 (control, verified) opens the file fine.
    tui.click(30, 6); // a.txt row
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    // Correct behavior: every column of the row is clickable; opening
    // a.txt shows its content in the editor.
    expect(snapshots[s]).toContain("aaa");
  });
});
