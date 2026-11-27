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
  it("inserting a line above never collapses a region the user did not fold", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "reattach.go", GO_SAMPLE);

    tui.start(file);
    tui.waitFor("outer");

    tui.exec("Go to Line");
    tui.type("4");
    tui.press("enter");
    tui.pressChord("ctrl+k", "["); // fold the if-block
    const folded = tui.snapshot();
    tui.exec("Go to Line");
    tui.type("1");
    tui.press("enter");
    tui.press("home");
    tui.press("enter"); // insert a blank line at the top
    const inserted = tui.snapshot();
    tui.press("ctrl+z");
    const undone = tui.snapshot();
    tui.press("ctrl+y");
    const redone = tui.snapshot();
    const { snapshots } = tui.run();

    for (const s of [folded, inserted, undone, redone]) {
      expect(snapshots[s]).not.toContain("func outer() { ⋯");
      expect(snapshots[s]).toContain("if true { ⋯");
    }
    expect(snapshots[inserted]).toMatch(/5›\s+if true/);
    expect(snapshots[undone]).toMatch(/4›\s+if true/);
    expect(snapshots[redone]).toMatch(/5›\s+if true/);
  });
});

describe("BUG-027: Move Line on a folded header swaps in hidden content", () => {
  it("alt+down on a folded header never reorders invisible code", () => {
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

    const content = readFile(file);
    expect(content.indexOf("if true {")).toBeLessThan(content.indexOf("foo()"));
  });
});
