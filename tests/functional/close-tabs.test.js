import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("close tabs commands", () => {
  it("should close all saved tabs, keeping dirty ones", () => {
    dir = createTempDir();
    const fileA = createTempFile(dir, "a.txt", "content-a");
    const fileB = createTempFile(dir, "b.txt", "content-b");
    const fileC = createTempFile(dir, "c.txt", "content-c");

    tui.start(fileA, fileB, fileC);
    tui.waitFor("c.txt");

    // Switch to b.txt (second tab) and make it dirty
    tui.press("alt+,");
    tui.waitFor("content-b");
    tui.type("dirty");

    tui.exec("View: Close All Saved Tabs");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // b.txt should remain because it has unsaved changes
    expect(snapshots[s0]).toContain("b.txt");
    // a.txt and c.txt were saved, so they should be closed
    expect(snapshots[s0]).not.toContain("a.txt");
    expect(snapshots[s0]).not.toContain("c.txt");
  });

  it("should close other tabs, keeping only the active one", () => {
    dir = createTempDir();
    const fileA = createTempFile(dir, "a.txt", "content-a");
    const fileB = createTempFile(dir, "b.txt", "content-b");
    const fileC = createTempFile(dir, "c.txt", "content-c");

    tui.start(fileA, fileB, fileC);
    tui.waitFor("c.txt");

    // Switch to b.txt
    tui.press("alt+,");
    tui.waitFor("content-b");

    tui.exec("View: Close Other Tabs");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // Only b.txt should remain
    expect(snapshots[s0]).toContain("b.txt");
    expect(snapshots[s0]).not.toContain("a.txt");
    expect(snapshots[s0]).not.toContain("c.txt");
  });

  it("should close all tabs and show untitled buffer when none are dirty", () => {
    dir = createTempDir();
    const fileA = createTempFile(dir, "a.txt", "content-a");
    const fileB = createTempFile(dir, "b.txt", "content-b");

    tui.start(fileA, fileB);
    tui.waitFor("b.txt");

    tui.exec("View: Close All Tabs");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // All tabs closed, editor should create a new untitled buffer
    expect(snapshots[s0]).toContain("untitled");
    expect(snapshots[s0]).not.toContain("a.txt");
    expect(snapshots[s0]).not.toContain("b.txt");
  });

  it("should show confirmation dialog when closing all tabs with dirty file", () => {
    dir = createTempDir();
    const fileA = createTempFile(dir, "a.txt", "content-a");

    tui.start(fileA);
    tui.waitFor("content-a");

    // Make the file dirty
    tui.type("dirty");

    tui.exec("View: Close All Tabs");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // A confirmation dialog should appear for the unsaved file
    expect(snapshots[s0]).toMatch(/Save|Discard|Cancel/);
  });
});
