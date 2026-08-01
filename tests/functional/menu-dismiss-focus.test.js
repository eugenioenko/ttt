import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("focus after dismissing a menu", () => {
  it("should type into the editor after closing a menu bar dropdown", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "focus.txt", "hello\n");

    tui.start(file);
    tui.waitFor("hello");

    tui.press("alt+v");
    tui.waitFor("Command Palette");
    tui.press("escape");
    tui.waitStable();
    tui.type("ZZZ");
    tui.waitStable();

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("ZZZhello");
  });

  it("should type into the editor after closing the right-click menu", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "focus.txt", "hello\n");

    tui.start(file);
    tui.waitFor("hello");

    tui.rclick(30, 6);
    tui.waitFor("Go to Definition");
    tui.press("escape");
    tui.waitStable();
    tui.type("QQQ");
    tui.waitStable();

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).not.toContain("Go to Definition");
    expect(snapshots[s0]).toContain("QQQ");
  });
});
