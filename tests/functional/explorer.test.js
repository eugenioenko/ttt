import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("explorer", () => {
  it("should show files in directory", () => {
    dir = createTempDir();
    createTempFile(dir, "alpha.txt", "alpha content");
    createTempFile(dir, "beta.txt", "beta content");
    createTempFile(dir, "gamma.txt", "gamma content");

    tui.start(dir);
    tui.waitFor("Explore");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("alpha.txt");
    expect(snapshots[s0]).toContain("beta.txt");
    expect(snapshots[s0]).toContain("gamma.txt");
  });

  it("should open file from explorer", () => {
    dir = createTempDir();
    createTempFile(dir, "test.txt", "HELLO WORLD");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.press("ctrl+0");

    // Navigate down to the file and open it
    tui.press("arrow_down");
    tui.press("arrow_down");
    tui.press("arrow_down");
    tui.press("enter");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("HELLO WORLD");
  });

  it("should show subdirectories", () => {
    dir = createTempDir();
    mkdirSync(join(dir, "subdir"), { recursive: true });
    writeFileSync(join(dir, "subdir", "inner.txt"), "inner content");

    tui.start(dir);
    tui.waitFor("Explore");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("subdir");
  });

  it("should expand subdirectory to show contents", () => {
    dir = createTempDir();
    mkdirSync(join(dir, "subdir"), { recursive: true });
    writeFileSync(join(dir, "subdir", "inner.txt"), "inner content");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.press("ctrl+0");

    // Navigate to subdir and expand it
    tui.press("arrow_down");
    tui.press("arrow_down");
    tui.press("enter");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("inner.txt");
  });

  it("should create new file from explorer", () => {
    dir = createTempDir();
    createTempFile(dir, "existing.txt", "existing content");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.press("ctrl+0");

    tui.exec("Explorer: New File");

    tui.type("newfile.txt");
    tui.press("enter");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("newfile.txt");
  });
});
