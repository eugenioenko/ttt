import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir, readFile, fileExists } from "./helpers.js";
import { join } from "node:path";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("save file", () => {
  it("should type text, save to disk via Save As, and verify contents", () => {
    dir = createTempDir();
    const filePath = join(dir, "hello.txt");

    tui.start();

    tui.type("Hello World");

    const s0 = tui.snapshot();

    tui.press("ctrl+s");

    tui.type(filePath);
    tui.press("enter");

    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("Hello World");
    expect(fileExists(filePath)).toBe(true);
    expect(readFile(filePath)).toContain("Hello World");
  });
});
