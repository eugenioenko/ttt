import { describe, it, expect, afterEach } from "vitest";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile, fileExists } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("save behavior edge cases", () => {
  it("save on clean file does not alter content", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "clean.txt", "original content\n");

    tui.start(file);
    tui.waitFor("original content");

    // Save without making any edits
    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    // Content must remain identical
    const content = readFile(file);
    expect(content).toBe("original content\n");
  });

  it("save adds final newline to file missing one", () => {
    dir = createTempDir();
    // Write file without trailing newline
    const file = createTempFile(dir, "noeol.txt", "no newline here");

    tui.start(file);
    tui.waitFor("no newline here");

    // Make a small edit then save to ensure the buffer is dirty and a write occurs
    tui.press("end");
    tui.type("!");
    tui.waitFor("no newline here!");

    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    const content = readFile(file);
    expect(content).toBe("no newline here!\n");
  });

  it("dirty indicator clears after save", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "dirty-clear.txt", "Clean start");

    tui.start(file);
    tui.waitFor("dirty-clear.txt");

    // Type something to make the buffer dirty
    tui.type("x");

    const s0 = tui.snapshot();

    // Save the file
    tui.press("ctrl+s");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("●"); // dirty dot
    expect(snapshots[s1]).not.toContain("●"); // dirty dot should be gone
  });

  it("save on new buffer shows Save As dialog", () => {
    dir = createTempDir();
    const newFilePath = join(dir, "created.txt");

    tui.start();

    tui.type("new content here");

    tui.press("ctrl+s");

    const s0 = tui.snapshot();

    tui.type(newFilePath);
    tui.press("enter");

    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("Save As");
    expect(fileExists(newFilePath)).toBe(true);
    expect(readFile(newFilePath)).toContain("new content here");
  });
});
