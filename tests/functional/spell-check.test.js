import { describe, it, expect, afterEach } from "vitest";
import { execSync } from "node:child_process";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

function aspellAvailable() {
  try {
    execSync("which aspell", { stdio: "pipe" });
    return true;
  } catch {
    return false;
  }
}

describe.skipIf(!aspellAvailable())("spell check", () => {
  it("suggests and applies a correction for a misspelled word", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "doc.md", "a smiple test\n");

    tui.start(file);
    tui.waitFor("a smiple test");
    tui.exec("Toggle Spell Check");
    tui.wait(1000);

    tui.press("arrow_right");
    tui.press("arrow_right");
    tui.press("arrow_right");
    tui.exec("Spell: Suggest Corrections");
    tui.waitStable();
    const sPopup = tui.snapshot();

    tui.press("enter");
    tui.waitStable();
    const sFixed = tui.snapshot();

    const { snapshots } = tui.run();

    expect(snapshots[sPopup]).toContain("simple");
    expect(snapshots[sFixed]).toContain("a simple test");
    expect(snapshots[sFixed]).not.toContain("smiple");
  });

  it("reports no misspelling on a correct word", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "doc.md", "a simple test\n");

    tui.start(file);
    tui.waitFor("a simple test");
    tui.exec("Toggle Spell Check");
    tui.wait(1000);

    tui.exec("Spell: Suggest Corrections");
    tui.waitStable();
    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("No misspelling at cursor");
  });
});
