import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("join lines", () => {
  it("should join current line with next using ctrl+k j", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "join.txt", "hello\n    world\nfoo\n");

    tui.start(file);
    tui.waitFor("hello");

    // Cursor starts on line 1
    tui.pressChord("ctrl+k", "j");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("hello world");
  });

  it("should do nothing on the last line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "joinlast.txt", "only line");

    tui.start(file);
    tui.waitFor("only line");

    tui.pressChord("ctrl+k", "j");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("only line");
  });

  it("should undo join with ctrl+z", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "joinundo.txt", "AAA\n    BBB\nCCC\n");

    tui.start(file);
    tui.waitFor("AAA");

    tui.pressChord("ctrl+k", "j");
    const s0 = tui.snapshot();

    tui.press("ctrl+z");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("AAA BBB");
    expect(snapshots[s1]).toContain("AAA");
    expect(snapshots[s1]).toContain("BBB");
  });

  it("should save the joined result", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "joinsave.txt", "Line1\n  Line2\nLine3\n");

    tui.start(file);
    tui.waitFor("Line1");

    tui.pressChord("ctrl+k", "j");

    tui.press("ctrl+s");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Line1 Line2");

    const content = readFile(file);
    expect(content).toContain("Line1 Line2");
  });
});
