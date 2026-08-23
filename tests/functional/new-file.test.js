import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile, fileExists } from "./helpers.js";
import { join } from "node:path";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("new file", () => {
  it("should create a new untitled tab with ctrl+n", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "existing.txt", "Existing content");

    tui.start(file);
    tui.waitFor("existing.txt");

    tui.press("ctrl+n");
    tui.waitFor("untitled");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("untitled");
  });

  it("should create a distinct tab when current untitled has content", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "existing.txt", "Existing content");

    tui.start(file);
    tui.waitFor("existing.txt");

    tui.press("ctrl+n");
    tui.waitFor("untitled");
    tui.type("some text");

    tui.press("ctrl+n");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("untitled-");
  });

  it("should save a new file via Save As", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "existing.txt", "Existing content");
    const newFile = join(dir, "new-file.txt");

    tui.start(file);
    tui.waitFor("existing.txt");

    tui.press("ctrl+n");
    tui.waitFor("untitled");

    tui.type("Brand new content");
    tui.waitFor("Brand new content");

    tui.press("ctrl+s");
    tui.waitFor("Save As");

    tui.type(newFile);
    tui.press("enter");

    const { snapshots } = tui.run();

    expect(fileExists(newFile)).toBe(true);
    expect(readFile(newFile)).toBe("Brand new content");
  });
});
