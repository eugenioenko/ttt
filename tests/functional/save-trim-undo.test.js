import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

const EDITORCONFIG = "root = true\n\n[*]\ntrim_trailing_whitespace = true\ninsert_final_newline = false\n";

// Trailing whitespace is invisible on screen, so these tests read the cursor
// column from the status bar: pressing End puts the cursor past whatever
// trailing spaces the line still has.
describe("save-time trim is undoable", () => {
  it("should undo the trim and the edit as two separate steps", () => {
    dir = createTempDir();
    createTempFile(dir, ".editorconfig", EDITORCONFIG);
    const file = createTempFile(dir, "test.txt", "abc   ");

    tui.start(file);
    tui.waitFor("abc");

    // One typed character, so the edit is a single undo group and cannot be
    // confused with the save cleanup.
    tui.press("home");
    tui.type("X");
    tui.press("end");
    const typed = tui.snapshot();

    tui.press("ctrl+s");
    tui.waitStable();
    tui.press("end");
    const saved = tui.snapshot();

    tui.press("ctrl+z");
    tui.waitStable();
    tui.press("end");
    const undoneOnce = tui.snapshot();

    tui.press("ctrl+z");
    tui.waitStable();
    tui.press("end");
    const undoneTwice = tui.snapshot();

    const { snapshots } = tui.run();

    // "Xabc" plus three trailing spaces.
    expect(snapshots[typed]).toContain("Col 8");
    // Saving trimmed them.
    expect(snapshots[saved]).toContain("Col 5");
    // First undo restores only what the save removed...
    expect(snapshots[undoneOnce]).toContain("Col 8");
    // ...and the second reaches the typed character.
    expect(snapshots[undoneTwice]).toContain("Col 7");
  });

  it("should leave the buffer matching the file it wrote", () => {
    dir = createTempDir();
    createTempFile(dir, ".editorconfig", EDITORCONFIG);
    const file = createTempFile(dir, "test.txt", "abc   ");

    tui.start(file);
    tui.waitFor("abc");

    tui.press("home");
    tui.type("X");
    tui.press("ctrl+s");
    tui.waitStable();

    const saved = tui.snapshot();
    const { snapshots } = tui.run();

    // The dirty dot clears: the trim was applied to the buffer, not just to
    // the bytes on disk behind the editor's back.
    expect(snapshots[saved]).not.toContain("● test.txt");
  });

  it("should not add an undo step when there is nothing to clean", () => {
    dir = createTempDir();
    // A file that already ends in a newline and has no trailing whitespace
    // needs no cleanup at all, so the save must push nothing.
    createTempFile(dir, ".editorconfig", "root = true\n\n[*]\ntrim_trailing_whitespace = true\ninsert_final_newline = true\n");
    const file = createTempFile(dir, "test.txt", "abc\n");

    tui.start(file);
    tui.waitFor("abc");

    tui.press("home");
    tui.type("X");
    tui.press("ctrl+s");
    tui.waitStable();

    // A single undo must reach the typing, not a no-op cleanup command.
    tui.press("ctrl+z");
    tui.waitStable();
    tui.press("end");
    const undone = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[undone]).toContain("Col 4");
  });
});
