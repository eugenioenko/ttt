import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createMultiLineFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("status bar after a scripted tab switch", () => {
  // The two files must sit on different lines: if both cursors are on line 1 a
  // stale reading and a correct one are identical, and the test cannot fail.
  it("should report the newly active tab's cursor, not the previous tab's", () => {
    dir = createTempDir();
    const first = createMultiLineFile(dir, "first.txt", 80);
    const second = createMultiLineFile(dir, "second.txt", 80);

    tui.start(first, second);

    // second.txt is active; move it to line 20 so the two tabs differ.
    tui.press("ctrl+g");
    tui.type("20");
    tui.press("enter");

    const onSecond = tui.snapshot();

    tui.exec("View: Previous Tab");
    const onFirst = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[onSecond]).toContain("Ln 20, Col 1");
    expect(snapshots[onFirst]).toContain("Ln 1, Col 1");
    expect(snapshots[onFirst]).not.toContain("Ln 20");
  });
});
