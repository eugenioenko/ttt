import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("duplicate line", () => {
  it("should duplicate current line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "dup.txt", "AAA\nBBB\nCCC");

    tui.start(file);
    tui.waitFor("AAA");

    tui.press("arrow_down");

    tui.exec("Duplicate Line");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("AAA\nBBB\nBBB\nCCC\n");
  });

  it("should duplicate last line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "duplast.txt", "First\nLast");

    tui.start(file);
    tui.waitFor("First");

    tui.press("arrow_down");

    tui.exec("Duplicate Line");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("First\nLast\nLast\n");
  });

  it("should undo duplicate line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "undodup.txt", "Only\nTwo");

    tui.start(file);
    tui.waitFor("Only");

    tui.exec("Duplicate Line");

    tui.press("ctrl+z");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("Only\nTwo\n");
  });
});
