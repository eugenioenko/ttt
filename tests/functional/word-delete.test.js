import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("word delete", () => {
  it("should delete word to the left via command palette", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "wordleft.txt", "hello world");

    tui.start(file);
    tui.waitFor("hello");

    tui.press("end");

    tui.exec("Delete Word Left");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("hello \n");
  });

  it("should delete word to the right via command palette", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "wordright.txt", "hello world today");

    tui.start(file);
    tui.waitFor("hello");

    tui.press("home");

    tui.exec("Delete Word Right");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe(" world today\n");
  });

  it("should undo word delete", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "undoword.txt", "keep these words");

    tui.start(file);
    tui.waitFor("keep");

    tui.press("end");

    tui.exec("Delete Word Left");

    tui.press("ctrl+z");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("keep these words\n");
  });
});
