import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("insert line", () => {
  it("should insert line below with ctrl+enter", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "below.txt", "AAA\nBBB\nCCC");

    tui.start(file);
    tui.waitFor("AAA");

    tui.press("ctrl+enter");
    tui.type("NEW");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("AAA\nNEW\nBBB\nCCC\n");
  });

  it("should insert line above via command palette", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "above.txt", "AAA\nBBB\nCCC");

    tui.start(file);
    tui.waitFor("AAA");

    tui.press("arrow_down");

    tui.exec("Insert Line Above");
    tui.type("NEW");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("AAA\nNEW\nBBB\nCCC\n");
  });

  it("should insert line below on last line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "last.txt", "AAA\nBBB\nCCC");

    tui.start(file);
    tui.waitFor("AAA");

    tui.press("arrow_down");
    tui.press("arrow_down");

    tui.press("ctrl+enter");
    tui.type("END");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("AAA\nBBB\nCCC\nEND\n");
  });

  it("should insert empty line below indented line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "indent.txt", "  indented\nnormal");

    tui.start(file);
    tui.waitFor("indented");

    tui.press("ctrl+enter");
    tui.type("x");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("  indented\nx\nnormal\n");
  });

  it("should undo insert line below", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "undo.txt", "AAA\nBBB\nCCC");

    tui.start(file);
    tui.waitFor("AAA");

    tui.press("ctrl+enter");
    tui.type("NEW");

    tui.press("ctrl+z");
    tui.press("ctrl+z");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("AAA\nBBB\nCCC\n");
  });
});
