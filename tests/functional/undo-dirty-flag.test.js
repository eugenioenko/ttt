import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("undo/redo dirty flag tracking", () => {
  it("should show dirty indicator after typing", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "clean.txt", "hello");

    tui.start(file);
    tui.waitFor("hello");

    const s0 = tui.snapshot();

    tui.type("x");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).not.toContain("●");
    expect(snapshots[s1]).toContain("●");
  });

  it("should clear dirty indicator after undo", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello");

    tui.start(file);
    tui.waitFor("hello");

    tui.type("x");

    const s0 = tui.snapshot();

    tui.press("ctrl+z");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("●");
    expect(snapshots[s1]).not.toContain("●");
  });

  it("should re-set dirty indicator after redo", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello");

    tui.start(file);
    tui.waitFor("hello");

    tui.type("x");

    tui.press("ctrl+z");

    const s0 = tui.snapshot();

    tui.press("ctrl+y");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).not.toContain("●");
    expect(snapshots[s1]).toContain("●");
  });

  it("should clear dirty indicator after save", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello");

    tui.start(file);
    tui.waitFor("hello");

    tui.type("x");

    const s0 = tui.snapshot();

    tui.press("ctrl+s");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("●");
    expect(snapshots[s1]).not.toContain("●");
  });

  it("should clear dirty indicator when undoing back to save point", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello");

    tui.start(file);
    tui.waitFor("hello");

    tui.press("end");
    tui.type("abc");

    tui.press("ctrl+s");

    tui.type("xyz");

    const s0 = tui.snapshot();

    // Undo "xyz" — back to the saved state "helloabc"
    tui.press("ctrl+z");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("●");
    expect(snapshots[s1]).not.toContain("●");
  });
});
