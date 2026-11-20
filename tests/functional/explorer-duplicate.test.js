import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, fileExists, readFile } from "./helpers.js";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("explorer duplicate", () => {
  it("copies a file to the Zed-style name without opening it", () => {
    dir = createTempDir();
    createTempFile(dir, "note.txt", "ORIGINAL CONTENT");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.press("ctrl+0");
    tui.press("arrow_down");
    tui.exec("Explorer: Duplicate");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("note copy.txt");
    expect(fileExists(join(dir, "note.txt"))).toBe(true);
    expect(readFile(join(dir, "note copy.txt"))).toBe("ORIGINAL CONTENT");
  });

  it("increments the copy number when the name is taken", () => {
    dir = createTempDir();
    createTempFile(dir, "note.txt", "first");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.press("ctrl+0");
    tui.press("arrow_down");
    tui.exec("Explorer: Duplicate");
    tui.exec("Explorer: Duplicate");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("note copy 2.txt");
    expect(readFile(join(dir, "note copy.txt"))).toBe("first");
    expect(readFile(join(dir, "note copy 2.txt"))).toBe("first");
  });

  it("copies a folder recursively under a copy name", () => {
    dir = createTempDir();
    mkdirSync(join(dir, "folder", "nested"), { recursive: true });
    writeFileSync(join(dir, "folder", "top.txt"), "top", "utf8");
    writeFileSync(join(dir, "folder", "nested", "child.txt"), "child", "utf8");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.press("ctrl+0");
    tui.press("arrow_down");
    tui.exec("Explorer: Duplicate");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("folder copy");
    expect(fileExists(join(dir, "folder", "top.txt"))).toBe(true);
    expect(readFile(join(dir, "folder copy", "top.txt"))).toBe("top");
    expect(readFile(join(dir, "folder copy", "nested", "child.txt"))).toBe("child");
  });
});
