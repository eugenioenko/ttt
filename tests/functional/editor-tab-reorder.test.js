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
    tui.drag(38, 2, 2, 2);
    tui.waitStable();
    const dragged = tui.snapshot();
    tui.exec("View: Move Tab Right");
    tui.waitStable();
    const moved = tui.snapshot();

    const { snapshots } = tui.run();
    const beforeHeader = snapshots[before].split("\n")[2];
    const draggedHeader = snapshots[dragged].split("\n")[2];
    const movedHeader = snapshots[moved].split("\n")[2];
    expect(beforeHeader.indexOf("gamma.txt")).toBeGreaterThan(beforeHeader.indexOf("untitled"));
    expect(draggedHeader.indexOf("gamma.txt")).toBeLessThan(draggedHeader.indexOf("untitled"));
    expect(movedHeader.indexOf("gamma.txt")).toBeGreaterThan(movedHeader.indexOf("untitled"));
  });
});
