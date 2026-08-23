import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("undo/redo stress", () => {
  it("should fully roundtrip diverse edits via undo and redo", () => {
    dir = createTempDir();
    const original = "The quick brown fox";
    const file = createTempFile(dir, "stress.txt", original);

    tui.start(file);
    tui.waitFor("The quick brown fox");

    // Move to end of line and make diverse edits
    tui.press("end");

    // Edit 1-2: type " jumps"
    tui.type(" jumps");

    // Edit 3: press Enter to create a new line
    tui.press("enter");

    // Edit 4-5: type "over the"
    tui.type("over the");

    // Edit 6: press Enter again
    tui.press("enter");

    // Edit 7-8: type "lazy dog"
    tui.type("lazy dog");

    // Edit 9: backspace to delete "g"
    tui.press("backspace");

    // Edit 10: backspace to delete "o"
    tui.press("backspace");

    // Edit 11-12: type "og!"
    tui.type("og!");

    // Edit 13: go to beginning of "lazy" line and select the word
    tui.press("home");
    tui.press("ctrl+d");

    // Edit 14: delete the selection ("lazy")
    tui.press("backspace");

    // Edit 15: type replacement
    tui.type("LAZY");

    // Snapshot after all edits
    const s0 = tui.snapshot();

    // Undo ALL edits (press ctrl+z many times)
    for (let i = 0; i < 20; i++) {
      tui.press("ctrl+z");
    }

    // Snapshot after full undo - should show original
    const s1 = tui.snapshot();

    // Redo ALL edits (press ctrl+y many times)
    for (let i = 0; i < 20; i++) {
      tui.press("ctrl+y");
    }

    // Save the final redo state
    tui.press("ctrl+s");

    // Snapshot after full redo
    const s2 = tui.snapshot();
    const { snapshots } = tui.run();

    // After editing: should contain edited content
    expect(snapshots[s0]).toContain("LAZY");
    expect(snapshots[s0]).toContain("jumps");

    // After undo: should show original, not edited content
    expect(snapshots[s1]).toContain("The quick brown fox");
    expect(snapshots[s1]).not.toContain("LAZY");

    // After redo: should show edited content again
    expect(snapshots[s2]).toContain("LAZY");
    expect(snapshots[s2]).toContain("jumps");

    // Verify file on disk matches redo state
    const content = readFile(file);
    expect(content).toContain("LAZY");
    expect(content).toContain("jumps");
    expect(content).toContain("over the");
  });

  it("should undo select-all delete to restore multi-line content", () => {
    dir = createTempDir();
    const original = "line one\nline two\nline three";
    const file = createTempFile(dir, "selectall.txt", original);

    tui.start(file);
    tui.waitFor("line one");

    // Navigate to end of last line (line 3)
    tui.press("ctrl+g");
    tui.type("3");
    tui.press("enter");
    tui.press("end");
    tui.press("enter");
    tui.type("line four");
    tui.press("enter");
    tui.type("line five");

    // Select all and delete
    tui.press("ctrl+a");
    tui.press("backspace");

    // Verify the buffer is empty (no original lines visible)
    const s0 = tui.snapshot();

    // Undo the delete and the typed text to get back to original
    tui.press("ctrl+z");

    // After undoing the delete, all content (including typed lines) should be restored
    const s1 = tui.snapshot();

    // Keep undoing to remove the typed lines
    for (let i = 0; i < 10; i++) {
      tui.press("ctrl+z");
    }

    // Save and verify original content is restored
    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    expect(snapshots[s0]).not.toContain("line one");
    expect(snapshots[s0]).not.toContain("line five");

    expect(snapshots[s1]).toContain("line one");

    const content = readFile(file);
    expect(content).toBe(original + "\n");
  });

  it("should discard redo stack when new edits are made after undo", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "interleave.txt", "start");

    tui.start(file);
    tui.waitFor("start");

    tui.press("end");

    // Make 5 edits (each separated by cursor movement to force separate undo groups)
    tui.type(" one");
    tui.press("arrow_left");
    tui.press("end");
    tui.type(" two");
    tui.press("arrow_left");
    tui.press("end");
    tui.type(" three");
    tui.press("arrow_left");
    tui.press("end");
    tui.type(" four");
    tui.press("arrow_left");
    tui.press("end");
    tui.type(" five");

    const s0 = tui.snapshot();

    // Undo 3 times
    tui.press("ctrl+z");
    tui.press("ctrl+z");
    tui.press("ctrl+z");

    const s1 = tui.snapshot();

    // Make 2 new edits (this should discard the redo stack)
    tui.type(" alpha");
    tui.press("arrow_left");
    tui.press("end");
    tui.type(" beta");

    const s2 = tui.snapshot();

    // Try to redo -- should do nothing since redo stack was discarded
    tui.press("ctrl+y");
    tui.press("ctrl+y");
    tui.press("ctrl+y");

    // Verify content is unchanged (redo did nothing)
    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("start one two three four five");

    expect(snapshots[s1]).toContain("start one two");
    expect(snapshots[s1]).not.toContain("five");

    expect(snapshots[s2]).toContain("start one two alpha beta");

    const content = readFile(file);
    expect(content).toBe("start one two alpha beta\n");
  });
});
