import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("undo and redo", () => {
  it("should undo typed text", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "undo.txt", "Base");

    tui.start(file);
    tui.waitFor("Base");

    tui.press("end");
    tui.type(" Added");
    tui.waitFor("Base Added");

    // " Added" is one group (space joins next word) → 1 undo
    tui.press("ctrl+z");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).not.toContain("Added");
    expect(snapshots[s0]).toContain("Base");
  });

  it("should redo after undo", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "redo.txt", "Base");

    tui.start(file);
    tui.waitFor("Base");

    tui.press("end");
    tui.type(" Extra");
    tui.waitFor("Base Extra");

    // " Extra" is one group → 1 undo
    tui.press("ctrl+z");

    const s0 = tui.snapshot();

    tui.press("ctrl+y");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).not.toContain("Extra");
    expect(snapshots[s1]).toContain("Base Extra");
  });

  it("should persist undo state through save", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "persist.txt", "First");

    tui.start(file);
    tui.waitFor("First");

    tui.press("end");
    tui.type(" Second");
    tui.waitFor("First Second");

    tui.press("ctrl+s");

    // Verify screen shows "First Second" after save
    const s0 = tui.snapshot();

    tui.type(" Third");
    tui.waitFor("First Second Third");

    // " Third" is one group → 1 undo
    tui.press("ctrl+z");

    tui.press("ctrl+s");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("First Second");
    expect(snapshots[s1]).not.toContain("Third");
    // After undo and save, file should contain "First Second" without "Third"
    const content = readFile(file);
    expect(content).toBe("First Second\n");
  });

  it("should undo word by word, not char by char", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "group.txt", "");

    tui.start(file);

    tui.type("hello world");
    tui.waitFor("hello world");

    // One undo removes " world" (space belongs with next word)
    tui.press("ctrl+z");

    const s0 = tui.snapshot();

    // Next undo removes "hello"
    tui.press("ctrl+z");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("hello");
    expect(snapshots[s0]).not.toContain("hello world");
    expect(snapshots[s1]).not.toContain("hello");
  });

  it("should break undo group on cursor movement", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "cursor.txt", "");

    tui.start(file);

    tui.type("ab");
    tui.press("arrow_left");
    tui.press("arrow_right");
    tui.type("cd");
    tui.waitFor("abcd");

    // First undo removes "cd" (typed after cursor movement)
    tui.press("ctrl+z");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("ab");
    expect(snapshots[s0]).not.toContain("abcd");
  });
});
