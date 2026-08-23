import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("unicode editing", () => {
  it("should display and edit text with accented characters", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "accent.txt", "café résumé naïve");

    tui.start(file);
    tui.waitFor("café");

    tui.press("end");
    tui.type(" über");

    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    const content = readFile(file);
    expect(content).toContain("café résumé naïve über");
  });

  it("should handle CJK characters", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "cjk.txt", "hello world");

    tui.start(file);
    tui.waitFor("hello");

    tui.press("end");
    tui.type(" 你好世界");

    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    const content = readFile(file);
    expect(content).toContain("hello world 你好世界");
  });

  it("should handle emoji characters", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "emoji.txt", "start end");

    tui.start(file);
    tui.waitFor("start");

    tui.press("end");
    tui.type(" 🎉🚀");

    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    const content = readFile(file);
    expect(content).toContain("start end 🎉🚀");
  });

  it("should preserve unicode content across edit and save", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "mixed.txt", "línea número één");

    tui.start(file);
    tui.waitFor("línea");

    tui.press("home");
    tui.type("→ ");

    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    const content = readFile(file);
    expect(content).toBe("→ línea número één\n");
  });
});
