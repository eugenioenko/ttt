import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
  dir = null;
});

describe("find navigation (F3 / Shift+F3)", () => {
  it("F3 cycles through matches", () => {
    dir = createTempDir();
    const file = createTempFile(
      dir,
      "cycle.txt",
      "cat here\ndog there\ncat again\nbird\ncat end"
    );

    tui.start(file);
    tui.waitFor("cat here");

    tui.press("ctrl+f");

    tui.type("cat");

    // Initial match is on line 1
    const s0 = tui.snapshot();

    // F3 moves to next match (line 3)
    tui.press("f3");

    const s1 = tui.snapshot();

    // F3 moves to next match (line 5)
    tui.press("f3");

    const s2 = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("1/3");
    expect(snapshots[s0]).toContain("Ln 1");
    expect(snapshots[s1]).toContain("Ln 3");
    expect(snapshots[s2]).toContain("Ln 5");
  });

  it("Shift+F3 goes to previous match", () => {
    dir = createTempDir();
    const file = createTempFile(
      dir,
      "prev.txt",
      "cat here\ndog there\ncat again\nbird\ncat end"
    );

    tui.start(file);
    tui.waitFor("cat here");

    tui.press("ctrl+f");

    tui.type("cat");

    // Start at match 1 on line 1
    const s0 = tui.snapshot();

    // F3 advances to match 2 on line 3
    tui.press("f3");

    const s1 = tui.snapshot();

    // Shift+F3 goes back to match 1 on line 1
    tui.press("shift+f3");

    const s2 = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Ln 1");
    expect(snapshots[s1]).toContain("Ln 3");
    expect(snapshots[s2]).toContain("Ln 1");
  });

  it("F3 wraps around from last match to first", () => {
    dir = createTempDir();
    const file = createTempFile(
      dir,
      "wrap.txt",
      "cat start\ndog middle\ncat end"
    );

    tui.start(file);
    tui.waitFor("cat start");

    tui.press("ctrl+f");

    tui.type("cat");

    // Start at match 1 on line 1
    const s0 = tui.snapshot();

    // F3 advances to match 2 on line 3
    tui.press("f3");

    const s1 = tui.snapshot();

    // F3 wraps around back to match 1 on line 1
    tui.press("f3");

    const s2 = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("1/2");
    expect(snapshots[s0]).toContain("Ln 1");
    expect(snapshots[s1]).toContain("Ln 3");
    expect(snapshots[s2]).toContain("Ln 1");
  });

  it("find with no matches shows zero count", () => {
    dir = createTempDir();
    const file = createTempFile(
      dir,
      "nomatch.txt",
      "cat here\ndog there\nbird end"
    );

    tui.start(file);
    tui.waitFor("cat here");

    tui.press("ctrl+f");

    tui.type("zzzzz");

    const s0 = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("0/0");
  });
});
