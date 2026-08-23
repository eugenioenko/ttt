import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("quit confirmation dialog", () => {
  it("should quit immediately with no unsaved changes", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "clean.txt", "No changes here");

    tui.start(file);
    tui.waitFor("clean.txt");

    tui.press("ctrl+q");

    // Editor should have quit; screenshot won't execute on exited process
    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).not.toContain("clean.txt");
  });

  it("should show confirm dialog when quitting with unsaved changes", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "dirty.txt", "Original");

    tui.start(file);
    tui.waitFor("dirty.txt");

    tui.type("x");

    tui.press("ctrl+q");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Unsaved changes");
    expect(snapshots[s0]).toContain("Cancel");
    expect(snapshots[s0]).toContain("Quit");
  });

  it("should dismiss dialog with Cancel", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "cancel.txt", "Keep editing");

    tui.start(file);
    tui.waitFor("cancel.txt");

    tui.type("x");

    tui.press("ctrl+q");
    tui.waitFor("Unsaved changes");

    tui.type("c");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).not.toContain("Unsaved changes");
    expect(snapshots[s0]).toContain("cancel.txt");
  });

  it("should quit when pressing Q in the dialog", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "quit-q.txt", "Will quit");

    tui.start(file);
    tui.waitFor("quit-q.txt");

    tui.type("x");

    tui.press("ctrl+q");
    tui.waitFor("Unsaved changes");

    tui.type("q");

    // Editor should have quit; screenshot won't execute on exited process
    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).not.toContain("quit-q.txt");
  });

  it("should force quit with second Ctrl+Q", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "force.txt", "Force quit");

    tui.start(file);
    tui.waitFor("force.txt");

    tui.type("x");

    tui.press("ctrl+q");
    tui.waitFor("Unsaved changes");

    tui.press("ctrl+q");

    // Editor should have quit; screenshot won't execute on exited process
    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).not.toContain("force.txt");
  });
});
