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

describe("BUG-017: ctrl+home/ctrl+end document navigation missing", () => {
  it.fails("ctrl+end moves the cursor to the end of the document", () => {
    dir = createTempDir();
    const lines = Array.from({ length: 50 }, (_, i) => `line ${i + 1}`).join("\n");
    const file = createTempFile(dir, "nav.txt", lines + "\n");

    tui.start(file);
    tui.waitFor("line 1");

    tui.press("ctrl+end");
    tui.type("Z"); // marker: reveals where the cursor actually is

    tui.press("ctrl+s");
    tui.run();

    // Buggy behavior: the KeyEnd handler ignores ModCtrl entirely — the
    // cursor never moves and the marker lands at the top of the file.
    expect(readFile(file)).toMatch(/line 50Z\n$/);
  });
});
