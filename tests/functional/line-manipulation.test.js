import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("line manipulation", () => {
  it("should delete a line with ctrl+k k chord", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "lines.txt", "Line 1\nLine 2\nLine 3");

    tui.start(file);
    tui.waitFor("Line 1");

    // Move to line 2
    tui.press("arrow_down");

    // Delete line (ctrl+k k chord)
    tui.pressChord("ctrl+k", "k");

    const s0 = tui.snapshot();

    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("Line 1");
    expect(snapshots[s0]).toContain("Line 3");
    expect(snapshots[s0]).not.toContain("Line 2");

    const content = readFile(file);
    expect(content).not.toContain("Line 2");
    expect(content).toContain("Line 1");
    expect(content).toContain("Line 3");
  });

  it("should undo line deletion with ctrl+z", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "undoline.txt", "Alpha\nBeta\nGamma");

    tui.start(file);
    tui.waitFor("Alpha");

    tui.press("arrow_down");

    tui.pressChord("ctrl+k", "k");

    const s0 = tui.snapshot();

    tui.press("ctrl+z");

    const s1 = tui.snapshot();

    const { snapshots } = tui.run();

    expect(snapshots[s0]).not.toContain("Beta");
    expect(snapshots[s1]).toContain("Beta");
  });
});
