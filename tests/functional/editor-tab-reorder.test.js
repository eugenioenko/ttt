import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("editor tab order", () => {
  it("drags visible file tabs and supports the command fallback", () => {
    dir = createTempDir();
    const alpha = createTempFile(dir, "alpha.txt", "alpha\n");
    const beta = createTempFile(dir, "beta.txt", "beta\n");
    const gamma = createTempFile(dir, "gamma.txt", "gamma\n");

    tui.start(alpha, beta, gamma);
    const before = tui.snapshot();
    tui.drag(29, 2, 2, 2);
    const dragged = tui.snapshot();
    tui.exec("View: Move Tab Right");
    const moved = tui.snapshot();

    const { snapshots } = tui.run();
    const beforeHeader = snapshots[before].split("\n")[2];
    const draggedHeader = snapshots[dragged].split("\n")[2];
    const movedHeader = snapshots[moved].split("\n")[2];
    // Opened files consume the pristine tab, so the strip holds only the
    // three files (no untitled anchor to drag against anymore).
    expect(beforeHeader).not.toContain("untitled");
    expect(beforeHeader.indexOf("alpha.txt")).toBeLessThan(beforeHeader.indexOf("gamma.txt"));
    expect(draggedHeader.indexOf("gamma.txt")).toBeLessThan(draggedHeader.indexOf("alpha.txt"));
    expect(movedHeader.indexOf("gamma.txt")).toBeGreaterThan(movedHeader.indexOf("alpha.txt"));
  });
});
