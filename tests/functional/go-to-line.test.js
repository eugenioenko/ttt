import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createMultiLineFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("go to line", () => {
  it("should jump to a specific line with ctrl+g", () => {
    dir = createTempDir();
    const file = createMultiLineFile(dir, "lines.txt", 50);

    tui.start(file);
    tui.waitFor("Line 1");

    tui.press("ctrl+g");

    tui.type("25");
    tui.press("enter");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Ln 25");
  });

  it("should dismiss go to line with escape", () => {
    dir = createTempDir();
    const file = createMultiLineFile(dir, "lines2.txt", 10);

    tui.start(file);
    tui.waitFor("Line 1");

    tui.press("ctrl+g");

    tui.press("escape");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Ln 1");
  });
});
