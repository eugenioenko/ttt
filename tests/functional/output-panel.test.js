import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("output panel", () => {
  it("should log status notifications", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello world\n");

    tui.start(file);
    tui.waitFor("hello");

    // Any command that reports through the status bar should leave a trail.
    tui.exec("File: Copy Absolute Path");

    tui.exec("Output: Show Panel");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("[notice]");
    expect(snapshots[s0]).toContain("Absolute path copied to clipboard");
  });

  it("should clear logged lines", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello world\n");

    tui.start(file);
    tui.waitFor("hello");

    tui.exec("File: Copy Absolute Path");
    tui.exec("Output: Show Panel");

    const before = tui.snapshot();

    tui.exec("Output: Clear");

    const after = tui.snapshot();
    const { snapshots } = tui.run();

    // Assert on the panel's own line format: the status bar still shows the
    // notification text for a few seconds after the panel has been cleared.
    expect(snapshots[before]).toContain("[notice]");
    expect(snapshots[after]).not.toContain("[notice]");
    expect(snapshots[after]).toContain("No output");
  });

  it("should copy the selected line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello world\n");

    tui.start(file);
    tui.waitFor("hello");

    tui.exec("File: Copy Absolute Path");
    tui.exec("Output: Show Panel");

    tui.exec("Output: Copy Selected Line");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Output line copied to clipboard");
  });
});
