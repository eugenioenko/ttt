import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("word wrap", () => {
  it("should wrap long lines making off-screen content visible", () => {
    dir = createTempDir();
    const longText = "VISIBLE_" + "x".repeat(500) + "_ENDMARK";
    const file = createTempFile(dir, "wrap.txt", longText);

    tui.start(file);
    tui.waitFor("VISIBLE_");

    const s0 = tui.snapshot();

    tui.exec("Toggle Word Wrap");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();

    const hadEndmark = snapshots[s0].includes("ENDMARK");
    const hasEndmark = snapshots[s1].includes("ENDMARK");
    expect(hadEndmark).not.toBe(hasEndmark);
  });

  it("should show wrapped content on multiple screen rows", () => {
    dir = createTempDir();
    const longText = "HELLO".repeat(200);
    const file = createTempFile(dir, "wrap2.txt", longText);

    tui.start(file);
    tui.waitFor("HELLO");

    const s0 = tui.snapshot();

    // Toggle word wrap
    tui.exec("Toggle Word Wrap");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();

    const before = snapshots[s0].split("\n").filter((l) => l.includes("HELLO")).length;
    const after = snapshots[s1].split("\n").filter((l) => l.includes("HELLO")).length;
    expect(before).not.toBe(after);
    expect(Math.max(before, after)).toBeGreaterThan(1);
  });

  it("should toggle word wrap off and hide off-screen content again", () => {
    dir = createTempDir();
    const longText = "VISIBLE_" + "x".repeat(500) + "_ENDMARK";
    const file = createTempFile(dir, "wrap3.txt", longText);

    tui.start(file);
    tui.waitFor("VISIBLE_");

    tui.exec("Toggle Word Wrap");

    const s0 = tui.snapshot();

    tui.exec("Toggle Word Wrap");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();

    const first = snapshots[s0].includes("ENDMARK");
    const second = snapshots[s1].includes("ENDMARK");
    expect(first).not.toBe(second);
  });
});
