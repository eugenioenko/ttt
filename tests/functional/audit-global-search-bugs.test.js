// Regression tests for fixed bugs from audit/2026-07-12-ux-bug-audit.md.
import { describe, it, expect, afterEach } from "vitest";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("BUG-047: global-search navigation ignores the match column (lands at col 0)", () => {
  it("activating a result places the cursor at the match column", () => {
    dir = createTempDir();
    writeFileSync(
      join(dir, "alpha.txt"),
      "line one\nneedle here in alpha\nanother needle line\nlast line no match\n",
    );

    tui.start(dir);
    tui.waitFor("Explore");
    tui.pressChord("ctrl+k", "f"); // open global search
    tui.type("needle");
    tui.waitFor("another needle line");
    tui.press("arrow_down");
    tui.press("arrow_down"); // to the "another needle line" match
    tui.press("enter"); // navigate
    tui.type("Z"); // marker at the cursor's landing column
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s]).toContain("another Zneedle line");
  });

  it("lands on the rune column when the line has multi-byte text before the match", () => {
    dir = createTempDir();
    writeFileSync(join(dir, "alpha.txt"), "héllo wörld needle\n");

    tui.start(dir);
    tui.waitFor("Explore");
    tui.pressChord("ctrl+k", "f");
    tui.type("needle");
    tui.waitFor("1: héllo wörld needle");
    tui.press("arrow_down");
    tui.press("enter");
    tui.type("Z");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s]).toContain("héllo wörld Zneedle");
  });
});
