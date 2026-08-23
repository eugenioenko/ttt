import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("sort lines", () => {
  it("should sort lines ascending via command palette", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "sort.txt", "cherry\napple\nbanana\n");

    tui.start(file);
    tui.waitFor("cherry");

    tui.press("ctrl+a");

    tui.exec("Sort Lines Ascending");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    const lines = content.split("\n").filter((l) => l !== "");
    expect(lines).toEqual(["apple", "banana", "cherry"]);
  });

  it("should sort lines descending via command palette", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "sortdesc.txt", "apple\ncherry\nbanana\n");

    tui.start(file);
    tui.waitFor("apple");

    tui.press("ctrl+a");

    tui.exec("Sort Lines Descending");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    const lines = content.split("\n").filter((l) => l !== "");
    expect(lines).toEqual(["cherry", "banana", "apple"]);
  });

  it("should reverse lines via command palette", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "reverse.txt", "first\nsecond\nthird\n");

    tui.start(file);
    tui.waitFor("first");

    tui.press("ctrl+a");

    tui.exec("Reverse Lines");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    const lines = content.split("\n").filter((l) => l !== "");
    expect(lines).toEqual(["third", "second", "first"]);
  });

  it("should remove duplicate lines via command palette", () => {
    dir = createTempDir();
    const file = createTempFile(
      dir,
      "unique.txt",
      "apple\nbanana\napple\ncherry\nbanana\n"
    );

    tui.start(file);
    tui.waitFor("apple");

    tui.press("ctrl+a");

    tui.exec("Unique Lines");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    const lines = content.split("\n").filter((l) => l !== "");
    expect(lines).toEqual(["apple", "banana", "cherry"]);
  });

  it("should sort lines ascending with ctrl+k o chord", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "sortchord.txt", "cherry\napple\nbanana\n");

    tui.start(file);
    tui.waitFor("cherry");

    tui.press("ctrl+a");

    tui.pressChord("ctrl+k", "o");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    const lines = content.split("\n").filter((l) => l !== "");
    expect(lines).toEqual(["apple", "banana", "cherry"]);
  });

  it("should undo sort with ctrl+z", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "sortundo.txt", "cherry\napple\nbanana\n");

    tui.start(file);
    tui.waitFor("cherry");

    tui.press("ctrl+a");

    tui.exec("Sort Lines Ascending");

    // Undo
    tui.press("ctrl+z");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    const lines = content.split("\n").filter((l) => l !== "");
    expect(lines).toEqual(["cherry", "apple", "banana"]);
  });
});
