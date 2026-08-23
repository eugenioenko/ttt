import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("rapid input", () => {
  it("should handle a long string burst without losing characters", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "burst.txt", "");

    // Build a 200-character string with varied characters
    const chars = "abcdefghijklmnopqrstuvwxyz0123456789";
    let longString = "";
    for (let i = 0; i < 200; i++) {
      longString += chars[i % chars.length];
    }

    tui.start(file);

    tui.type(longString);

    // Save and verify every character was captured
    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe(longString + "\n");
  });

  it("should handle rapid line creation", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "lines.txt", "");

    tui.start(file);

    // Type 20 lines rapidly
    for (let i = 1; i <= 20; i++) {
      tui.type("Line " + i);
      if (i < 20) {
        tui.press("enter");
      }
    }

    // The status bar should show we are on line 20
    const s0 = tui.snapshot();

    // Save and verify all lines are present
    tui.press("ctrl+s");

    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Ln 20");

    const content = readFile(file);
    for (let i = 1; i <= 20; i++) {
      expect(content).toContain("Line " + i);
    }
  });

  it("should not lose data on type followed by immediate save", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "immediate.txt", "");

    const paragraph =
      "The quick brown fox jumps over the lazy dog. " +
      "Pack my box with five dozen liquor jugs. " +
      "How vexingly quick daft zebras jump.";

    tui.start(file);

    tui.type(paragraph);
    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe(paragraph + "\n");
  });
});
