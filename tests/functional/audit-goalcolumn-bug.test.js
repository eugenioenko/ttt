// Repro test for confirmed bug from audit/2026-07-12-ux-bug-audit.md (branch audit/bug-hunt).
// Asserts the CORRECT behavior with `it.fails` — passes while the bug
// exists, goes red when fixed. Remove `.fails` + audit entry when fixing.
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

describe("BUG-051: goal column not preserved through a shorter line", () => {
  it.fails("vertical movement restores the column after passing a short line", () => {
    dir = createTempDir();
    // line0: 20 'a', line1: short (5), line2: 20 'b'
    writeFileSync(
      join(dir, "goal.txt"),
      "aaaaaaaaaaaaaaaaaaaa\nshort\nbbbbbbbbbbbbbbbbbbbb\n",
    );

    tui.start(join(dir, "goal.txt"));
    tui.waitFor("aaaa");

    for (let i = 0; i < 12; i++) tui.press("right"); // col 12 on line0
    tui.press("arrow_down"); // line1 "short" — clamps to col 5
    tui.press("arrow_down"); // line2 (long) — goal should restore to col 12
    tui.type("Z"); // marker at the landing column
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    // Correct: goal column 12 restores on the long line →
    // "bbbbbbbbbbbbZbbbbbbbb". Buggy: the clamp to the short line
    // overwrote Col to 5 (no goal field on Cursor) → "bbbbbZbbb...".
    expect(snapshots[s]).toContain("bbbbbbbbbbbbZbbbbbbbb");
  });
});
