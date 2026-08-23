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

describe("BUG-016: tab-bar overflow chevron switches tabs instead of scrolling", () => {
  it.fails("clicking the ◀ chevron does not change the active tab", () => {
    dir = createTempDir();
    const files = [1, 2, 3, 4, 5].map((i) =>
      createTempFile(dir, `tf${i}.txt`, `content${i}\n`),
    );

    tui.start(...files);
    tui.setSize(50, 20); // narrow screen so the tab strip overflows
    tui.waitFor("content");

    tui.click(2, 2); // the "◀" overflow chevron
    tui.press("home");
    tui.type("Z");

    tui.press("ctrl+s");
    tui.run();

    // Correct behavior: the chevron only scrolls the tab strip; tf5 (last
    // opened) stays active, so the marker lands there. Buggy behavior:
    // the click calls PrevTab() and the marker lands in tf4.
    expect(readFile(files[4])).toBe("Zcontent5\n");
  });
});
