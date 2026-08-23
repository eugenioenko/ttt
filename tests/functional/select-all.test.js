import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("select all and overwrite", () => {
  it("should select all text and replace with new content", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "selectall.txt", "old content\nthat spans\nmultiple lines");

    tui.start(file);
    tui.waitFor("old content");

    tui.press("ctrl+a");

    tui.type("replaced");

    tui.press("ctrl+s");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    const content = readFile(file);
    expect(content).toBe("replaced\n");
  });

  it("should undo select-all overwrite to restore original", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "undosel.txt", "preserve this\nand this");

    tui.start(file);
    tui.waitFor("preserve");

    tui.press("ctrl+a");

    tui.type("x");

    const s0 = tui.snapshot();

    // undo the typed char, then undo the deletion
    tui.press("ctrl+z");
    tui.press("ctrl+z");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).not.toContain("preserve");
    expect(snapshots[s1]).toContain("preserve");
  });
});
