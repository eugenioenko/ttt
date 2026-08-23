// Repro tests for confirmed bugs from audit/2026-07-12-ux-bug-audit.md (branch audit/bug-hunt).
// Each test asserts the CORRECT behavior and is declared with `it.fails`,
// so it passes while the bug exists and goes red the moment the bug is
// fixed — at that point remove the `.fails` marker and the audit entry.
//
// Selection convention under test (used by JoinLines/ToggleLineComment):
// a selection ending at col 0 of a line does NOT include that line.
import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

const FIVE_LINES = "line0\nline1\nline2\nline3\nline4\n";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("BUG-003: Duplicate/Delete Line ignore active multi-line selection", () => {
  it("Duplicate Line duplicates the selected block", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "dupblock.txt", FIVE_LINES);

    tui.start(file);
    tui.waitFor("line0");

    // Select lines 1-2 (selection ends at line3 col 0)
    tui.press("arrow_down");
    tui.press("shift+down");
    tui.press("shift+down");
    tui.exec("Duplicate Line");

    tui.press("ctrl+s");
    tui.run();

    // Buggy behavior duplicates only the cursor's line (line3), which per
    // the col-0 convention is not even part of the selection.
    expect(readFile(file)).toBe("line0\nline1\nline2\nline1\nline2\nline3\nline4\n");
  });

  it("Delete Line deletes the selected block", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "delblock.txt", FIVE_LINES);

    tui.start(file);
    tui.waitFor("line0");

    tui.press("arrow_down");
    tui.press("shift+down");
    tui.press("shift+down");
    tui.exec("Delete Line");

    tui.press("ctrl+s");
    tui.run();

    // Buggy behavior deletes only the cursor's line (line3), leaving the
    // selected lines untouched and the selection range stale.
    expect(readFile(file)).toBe("line0\nline3\nline4\n");
  });
});

describe("BUG-002: Tab indent with selection ending at col 0", () => {
  it("indents only lines the selection actually covers", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "indent.txt", FIVE_LINES);

    tui.start(file);
    tui.waitFor("line0");

    // Select line1 only (selection ends at line2 col 0)
    tui.press("arrow_down");
    tui.press("shift+down");
    tui.press("tab");

    tui.press("ctrl+s");
    tui.run();

    // Buggy behavior indents line2 as well.
    expect(readFile(file)).toBe("line0\n    line1\nline2\nline3\nline4\n");
  });
});

describe("BUG-001: Move Line with selection ending at col 0", () => {
  it("moves only the selected block, not the trailing col-0 line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "moveblock.txt", FIVE_LINES);

    tui.start(file);
    tui.waitFor("line0");

    // Cursor to line2, then select lines 2-3 (selection ends at line4 col 0)
    tui.press("arrow_down");
    tui.press("arrow_down");
    tui.press("shift+down");
    tui.press("shift+down");
    tui.exec("Move Line Down");

    tui.press("ctrl+s");
    tui.run();

    // Block line2+line3 swaps past line4. Buggy behavior instead swaps the
    // invisible trailing empty line into the buffer, injecting a blank line.
    expect(readFile(file)).toBe("line0\nline1\nline4\nline2\nline3\n");
  });
});
