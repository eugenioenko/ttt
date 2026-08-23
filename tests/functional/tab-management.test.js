import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("tab management", () => {
  it("should open multiple files as tabs", () => {
    dir = createTempDir();
    const file1 = createTempFile(dir, "first.txt", "First file");
    const file2 = createTempFile(dir, "second.txt", "Second file");

    tui.start(file1, file2);
    tui.waitFor("second.txt");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("first.txt");
    expect(snapshots[s0]).toContain("second.txt");
  });

  it("should close active tab with ctrl+w", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "closeme.txt", "Close this content");

    tui.start(file);
    tui.waitFor("Close this content");

    tui.press("ctrl+w");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("untitled");
    expect(snapshots[s0]).not.toContain("Close this content");
  });

  it("should show unsaved changes dialog when closing dirty tab", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "unsaved.txt", "Original");

    tui.start(file);
    tui.waitFor("unsaved.txt");

    tui.type("dirty");

    tui.press("ctrl+w");
    tui.waitFor("Save changes");

    const s0 = tui.snapshot();

    // Cancel the dialog
    tui.press("escape");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("Save changes");
    expect(snapshots[s1]).toContain("unsaved.txt");
  });

  it("should discard unsaved changes from dialog", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "discard.txt", "Original content");

    tui.start(file);
    tui.waitFor("Original content");

    tui.type("dirty");

    tui.press("ctrl+w");
    tui.waitFor("Save changes");

    tui.press("enter");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // Tab closed, editor shows untitled, original file unchanged
    expect(snapshots[s0]).toContain("untitled");
    expect(readFile(file)).toBe("Original content");
  });
});
