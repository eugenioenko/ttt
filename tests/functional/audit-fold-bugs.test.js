// Repro tests for confirmed bugs from audit/2026-07-12-ux-bug-audit.md (branch audit/bug-hunt).
// Each test asserts the CORRECT behavior and is declared with `it.fails`,
// so it passes while the bug exists and goes red the moment the bug is
// fixed — at that point remove the `.fails` marker and the audit entry.
import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

const GO_SAMPLE =
  "package main\n\nfunc outer() {\n\tif true {\n\t\tfoo()\n\t}\n}\n\nfunc other() {\n}\n";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("BUG-026: fold reattaches to an unrelated block after edits above", () => {
  it.fails("inserting a line above never collapses a region the user did not fold", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "reattach.go", GO_SAMPLE);

    tui.start(file);
    tui.waitFor("outer");

    tui.exec("Go to Line");
    tui.type("4");
    tui.press("enter");
    tui.pressChord("ctrl+k", "["); // fold the if-block
    tui.exec("Go to Line");
    tui.type("1");
    tui.press("enter");
    tui.press("home");
    tui.press("enter"); // insert a blank line at the top
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    // Correct: the fold either follows the if-block or is cleared — but
    // func outer()'s body must NEVER become collapsed (the user never
    // folded it). Buggy: SetRanges matches collapsed state by raw
    // StartLine equality, so the shifted outer function inherits the
    // inner block's fold and silently hides different code.
    expect(snapshots[s]).not.toContain("func outer() { ⋯");
  });
});

describe("BUG-027: Move Line on a folded header swaps in hidden content", () => {
  it.fails("alt+down on a folded header never reorders invisible code", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "swap.go", GO_SAMPLE);

    tui.start(file);
    tui.waitFor("outer");

    tui.exec("Go to Line");
    tui.type("4");
    tui.press("enter");
    tui.pressChord("ctrl+k", "["); // fold the if-block
    tui.press("alt+down"); // move line down on the folded header

    tui.press("ctrl+s");
    tui.run();

    // Correct: the whole folded block moves as a unit (VS Code) or the
    // command is a no-op while folded — either way foo() stays inside
    // its if-block. Buggy: the raw header line swaps with the HIDDEN
    // foo() line, reordering code the user cannot see, while the stale
    // fold marker keeps rendering.
    const content = readFile(file);
    expect(content.indexOf("if true {")).toBeLessThan(content.indexOf("foo()"));
  });
});
