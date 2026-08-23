import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("delete line", () => {
  it("should delete first line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "del1.txt", "AAA\nBBB\nCCC");

    tui.start(file);
    tui.waitFor("AAA");

    tui.pressChord("ctrl+k", "k");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("BBB\nCCC\n");
  });

  it("should delete middle line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "del2.txt", "AAA\nBBB\nCCC");

    tui.start(file);
    tui.waitFor("AAA");

    tui.press("arrow_down");

    tui.pressChord("ctrl+k", "k");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("AAA\nCCC\n");
  });

  it("should delete last line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "del3.txt", "AAA\nBBB\nCCC");

    tui.start(file);
    tui.waitFor("AAA");

    tui.press("arrow_down");
    tui.press("arrow_down");

    tui.pressChord("ctrl+k", "k");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("AAA\nBBB\n");
  });

  it("should delete only line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "del4.txt", "ONLY");

    tui.start(file);
    tui.waitFor("ONLY");

    tui.pressChord("ctrl+k", "k");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("");
  });

  it("should delete multiple lines in sequence", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "del5.txt", "AAA\nBBB\nCCC\nDDD");

    tui.start(file);
    tui.waitFor("AAA");

    tui.pressChord("ctrl+k", "k");
    tui.pressChord("ctrl+k", "k");
    tui.pressChord("ctrl+k", "k");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("DDD\n");
  });

  it("should undo delete line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "del6.txt", "AAA\nBBB");

    tui.start(file);
    tui.waitFor("AAA");

    tui.pressChord("ctrl+k", "k");

    tui.press("ctrl+z");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("AAA\nBBB\n");
  });
});
