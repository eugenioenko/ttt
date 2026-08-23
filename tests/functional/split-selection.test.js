import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("split selection into lines", () => {
  it("should create per-line cursors from multi-line selection", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "aaa\nbbb\nccc");

    tui.start(file);
    tui.waitFor("aaa");

    // Select all text (3 lines)
    tui.press("ctrl+a");

    tui.exec("Split Selection into Lines");

    // Type 'X' — should appear on each of the 3 lines with cursors
    tui.type("X");

    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    const content = readFile(file);
    expect(content).toContain("aaaX");
    expect(content).toContain("bbbX");
    expect(content).toContain("cccX");
  });

  it("should do nothing with no selection", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "nosel.txt", "hello\nworld\n");

    tui.start(file);
    tui.waitFor("hello");

    tui.exec("Split Selection into Lines");

    // Type 'Z' — should only appear once (single cursor)
    tui.type("Z");

    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    const content = readFile(file);
    const zCount = (content.match(/Z/g) || []).length;
    expect(zCount).toBe(1);
  });
});
