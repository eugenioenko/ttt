import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import {
  createTempDir,
  createTempFile,
  createMultiLineFile,
  cleanupDir,
} from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

function switchToNextTab() {
  tui.press("alt+.");
}

function switchToPrevTab() {
  tui.press("alt+,");
}

describe("multi-tab state isolation", () => {
  it("should show correct content per tab when switching", () => {
    dir = createTempDir();
    const fileA = createTempFile(dir, "alpha.txt", "content-alpha");
    const fileB = createTempFile(dir, "beta.txt", "content-beta");

    tui.start(fileA, fileB);
    tui.waitFor("content-beta");

    // File B is the active tab (last opened)
    const s0 = tui.snapshot();

    // Switch to file A via command palette
    switchToPrevTab();
    tui.waitFor("content-alpha");

    const s1 = tui.snapshot();

    // Switch back to file B
    switchToNextTab();
    tui.waitFor("content-beta");

    const s2 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("content-beta");

    expect(snapshots[s1]).toContain("content-alpha");
    expect(snapshots[s1]).not.toContain("content-beta");

    expect(snapshots[s2]).toContain("content-beta");
    expect(snapshots[s2]).not.toContain("content-alpha");
  });

  it("should preserve cursor position when switching tabs", () => {
    dir = createTempDir();
    const fileA = createMultiLineFile(dir, "cursa.txt", 10);
    const fileB = createMultiLineFile(dir, "cursb.txt", 10);

    tui.start(fileA, fileB);
    tui.waitFor("cursb.txt");

    // File B is active. Switch to file A.
    switchToPrevTab();
    tui.waitFor("cursa.txt");

    // Move cursor to line 5 in file A
    tui.press("ctrl+g");
    tui.type("5");
    tui.press("enter");

    const s0 = tui.snapshot();

    // Switch to file B and move cursor to line 3
    switchToNextTab();
    tui.waitFor("cursb.txt");

    tui.press("ctrl+g");
    tui.type("3");
    tui.press("enter");

    const s1 = tui.snapshot();

    // Switch back to file A - cursor should still be on line 5
    switchToPrevTab();
    tui.waitFor("cursa.txt");

    const s2 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("Ln 5");
    expect(snapshots[s1]).toContain("Ln 3");
    expect(snapshots[s2]).toContain("Ln 5");
  });

  it("should isolate edits between tabs", () => {
    dir = createTempDir();
    const fileA = createTempFile(dir, "edita.txt", "original-a");
    const fileB = createTempFile(dir, "editb.txt", "original-b");

    tui.start(fileA, fileB);
    tui.waitFor("original-b");

    // File B is active. Switch to file A.
    switchToPrevTab();
    tui.waitFor("original-a");

    // Edit file A
    tui.press("end");
    tui.type(" EDIT-A");
    tui.waitFor("EDIT-A");

    // Switch to file B and verify EDIT-A is not present
    switchToNextTab();
    tui.waitFor("original-b");

    const s0 = tui.snapshot();

    // Edit file B
    tui.press("end");
    tui.type(" EDIT-B");
    tui.waitFor("EDIT-B");

    // Switch back to file A - should have EDIT-A but not EDIT-B
    switchToPrevTab();
    tui.waitFor("EDIT-A");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).not.toContain("EDIT-A");

    expect(snapshots[s1]).toContain("EDIT-A");
    expect(snapshots[s1]).not.toContain("EDIT-B");
  });

  it("should track dirty indicator per tab independently", () => {
    dir = createTempDir();
    const fileA = createTempFile(dir, "dirtya.txt", "clean-a");
    const fileB = createTempFile(dir, "dirtyb.txt", "clean-b");

    tui.start(fileA, fileB);
    tui.waitFor("dirtyb.txt");

    // File B is active. Switch to file A.
    switchToPrevTab();
    tui.waitFor("clean-a");

    // Neither file is dirty yet
    const s0 = tui.snapshot();

    // Edit file A to make it dirty
    tui.type("x");

    const s1 = tui.snapshot();

    // Switch to file B
    switchToNextTab();
    tui.waitFor("clean-b");

    // File B content is clean
    const s2 = tui.snapshot();

    // Switch back to file A - should still show dirty indicator
    switchToPrevTab();
    tui.waitFor("clean-a");

    const s3 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).not.toContain("●");
    expect(snapshots[s1]).toContain("●");
    expect(snapshots[s2]).toContain("clean-b");
    expect(snapshots[s3]).toContain("●");
  });
});
