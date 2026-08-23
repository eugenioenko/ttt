import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createMultiLineFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("large file scroll stability", () => {
  it("should scroll to bottom with ctrl+g to last line", () => {
    dir = createTempDir();
    const file = createMultiLineFile(dir, "big.txt", 500);

    tui.start(file);
    tui.waitFor("big.txt");

    // Jump to the last line using go-to-line
    tui.press("ctrl+g");
    tui.type("500");
    tui.press("enter");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Line 500");
    expect(snapshots[s0]).toContain("500");
    expect(snapshots[s0]).toContain("Ln 500");
  });

  it("should scroll to top with ctrl+g to line 1", () => {
    dir = createTempDir();
    const file = createMultiLineFile(dir, "big2.txt", 500);

    tui.start(file);
    tui.waitFor("big2.txt");

    // Jump to bottom first
    tui.press("ctrl+g");
    tui.type("500");
    tui.press("enter");
    tui.waitFor("Line 500");

    // Now jump back to top
    tui.press("ctrl+g");
    tui.type("1");
    tui.press("enter");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Line 1");
    expect(snapshots[s0]).toContain("Ln 1");
  });

  it("should go to a middle line with ctrl+g", () => {
    dir = createTempDir();
    const file = createMultiLineFile(dir, "big3.txt", 500);

    tui.start(file);
    tui.waitFor("big3.txt");

    tui.press("ctrl+g");
    tui.type("250");
    tui.press("enter");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Line 250");
    expect(snapshots[s0]).toContain("Ln 250");
  });

  it("should page down and page up through a large file", () => {
    dir = createTempDir();
    const file = createMultiLineFile(dir, "big4.txt", 500);

    tui.start(file);
    tui.waitFor("big4.txt");

    // Verify we start at the top
    const s0 = tui.snapshot();

    // Press page down to scroll away from the top
    tui.press("page_down");
    tui.press("page_down");
    tui.press("page_down");

    // Cursor should have moved past line 1
    const s1 = tui.snapshot();

    // Press page up the same number of times to return near the top
    tui.press("page_up");
    tui.press("page_up");
    tui.press("page_up");

    // Should be back at line 1
    const s2 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toMatch(/Ln 1\b/);
    expect(snapshots[s1]).not.toMatch(/Ln 1\b/);
    expect(snapshots[s2]).toMatch(/Ln 1\b/);
  });
});
