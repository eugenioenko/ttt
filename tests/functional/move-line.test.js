import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("move line", () => {
  it("should move line down via command palette", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "movedown.txt", "AAA\nBBB\nCCC");

    tui.start(file);
    tui.waitFor("AAA");

    tui.exec("Move Line Down");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("BBB\nAAA\nCCC\n");
  });

  it("should move line up via command palette", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "moveup.txt", "AAA\nBBB\nCCC");

    tui.start(file);
    tui.waitFor("AAA");

    tui.press("arrow_down");
    tui.press("arrow_down");

    tui.exec("Move Line Up");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("AAA\nCCC\nBBB\n");
  });

  it("should not move first line up", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "nomove.txt", "First\nSecond");

    tui.start(file);
    tui.waitFor("First");

    tui.exec("Move Line Up");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("First\nSecond\n");
  });

  it("should undo move line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "undomove.txt", "One\nTwo\nThree");

    tui.start(file);
    tui.waitFor("One");

    tui.exec("Move Line Down");

    tui.press("ctrl+z");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("One\nTwo\nThree\n");
  });
});
