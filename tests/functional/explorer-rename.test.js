import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import {
  createTempDir,
  createTempFile,
  cleanupDir,
  fileExists,
  readFile,
} from "./helpers.js";
import { writeFileSync } from "node:fs";
import { join } from "node:path";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

// The rename dialog is pre-filled with the current basename and places no
// selection, so the old name has to be cleared before typing the new one.
function renameSelectedTo(oldName, newName) {
  tui.exec("Explorer: Rename");
  for (let i = 0; i < oldName.length; i++) tui.press("backspace");
  tui.type(newName);
  tui.press("enter");
}

describe("explorer rename", () => {
  it("should retitle the open tab when its file is renamed", () => {
    dir = createTempDir();
    createTempFile(dir, "alpha.txt", "ALPHA CONTENT");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.press("ctrl+0");
    tui.press("arrow_down");
    tui.press("enter");

    renameSelectedTo("alpha.txt", "renamed.txt");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("renamed.txt");
    expect(snapshots[s0]).not.toContain("alpha.txt");
  });

  // Issue #284: the tab kept the old path, so saving wrote the buffer back out
  // under the name the file had been renamed away from.
  it("should save to the new path without recreating the old file", () => {
    dir = createTempDir();
    createTempFile(dir, "alpha.txt", "ALPHA CONTENT");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.press("ctrl+0");
    tui.press("arrow_down");
    tui.press("enter");

    renameSelectedTo("alpha.txt", "renamed.txt");

    tui.type("EDITED ");
    tui.exec("Save File");
    tui.run();

    expect(fileExists(join(dir, "alpha.txt"))).toBe(false);
    expect(fileExists(join(dir, "renamed.txt"))).toBe(true);
    expect(readFile(join(dir, "renamed.txt"))).toContain("EDITED");
  });

  it("should keep editing the same buffer after a rename", () => {
    dir = createTempDir();
    createTempFile(dir, "alpha.txt", "ORIGINAL LINE");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.press("ctrl+0");
    tui.press("arrow_down");
    tui.press("enter");

    renameSelectedTo("alpha.txt", "renamed.txt");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("ORIGINAL LINE");
  });
});

describe("explorer refresh", () => {
  it("should pick up a file created outside the editor when r is pressed", () => {
    dir = createTempDir();
    createTempFile(dir, "alpha.txt", "alpha");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.press("ctrl+0");

    // The explorer has already listed the directory; add a file behind its back.
    writeFileSync(join(dir, "created-outside.txt"), "outside");

    tui.press("r");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("created-outside.txt");
  });
});
