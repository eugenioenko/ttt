import { afterEach, describe, expect, it } from "vitest";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { cleanupDir, createGitRepo, createTempDir, createTempFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("tree actions", () => {
  it("expands every visible Explorer descendant once and collapses them together", () => {
    dir = createGitRepo(createTempDir());
    for (const path of ["deep/one/two", "visible/inner", "visible/.hidden", "visible/.ignored"]) {
      mkdirSync(join(dir, path), { recursive: true });
    }
    createTempFile(dir, "deep/one/two/deep.txt", "deep\n");
    createTempFile(dir, "visible/inner/inner.txt", "inner\n");
    createTempFile(dir, "visible/.hidden/hidden.txt", "hidden\n");
    createTempFile(dir, "visible/.ignored/ignored.txt", "ignored\n");
    writeFileSync(join(dir, ".gitignore"), "visible/.ignored/\n", "utf8");

    tui.start(dir);
    tui.setSize(100, 50);
    tui.pressChord("ctrl+k", "e");
    tui.waitStable();
    tui.exec("Explorer: Collapse All");
    const collapsed = tui.snapshot();
    tui.exec("Explorer: Expand All");
    const expanded = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[collapsed]).not.toContain("deep");
    for (const label of ["one", "two", "deep.txt", "inner", "inner.txt", ".hidden", "hidden.txt", ".ignored", "ignored.txt"]) {
      expect(snapshots[expanded]).toContain(label);
    }
    expect(snapshots[expanded]).toContain(".git");
    expect(snapshots[expanded]).not.toContain("COMMIT_EDITMSG");
    expect(snapshots[expanded]).not.toContain("objects");
    expect(snapshots[expanded]).not.toContain("hooks");
  });
});
