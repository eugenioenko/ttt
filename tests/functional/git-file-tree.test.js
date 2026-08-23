import { afterEach, describe, expect, it } from "vitest";
import { mkdirSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { cleanupDir, createGitRepo, createTempDir, createTempFile, git } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

function nestedRepo() {
  const repo = createGitRepo(createTempDir());
  mkdirSync(join(repo, "pkg", "history"), { recursive: true });
  createTempFile(repo, "pkg/history/committed.go", "package history\n");
  git(repo, "add", "-A");
  git(repo, "commit", "-qm", "nested history");
  mkdirSync(join(repo, "src", "work"), { recursive: true });
  createTempFile(repo, "src/work/changed.go", "package work\n");
  return repo;
}

describe("git file tree", () => {
  it("defaults to lists and preserves working and history files across tree bulk controls", () => {
    dir = nestedRepo();
    tui.start(dir);
    tui.pressChord("ctrl+k", "c");
    tui.waitFor("nested history");
    tui.press("tab");
    tui.press("tab");
    tui.press("down");
    tui.press("right");
    tui.waitFor("pkg/history/committed.go");
    const list = tui.snapshot();

    tui.exec("View Git Files as Tree");
    const tree = tui.snapshot();
    tui.exec("Git: Collapse All File Trees");
    const collapsed = tui.snapshot();
    tui.exec("Git: Expand All File Trees");
    const expanded = tui.snapshot();

    tui.rclick(5, 5);
    tui.press("down");
    tui.press("down");
    tui.press("right");
    const context = tui.snapshot();
    tui.press("escape");
    tui.click(29, 2);
    tui.press("down");
    tui.press("down");
    tui.press("right");
    const threeDot = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[list]).toContain("src/work/changed.go");
    expect(snapshots[list]).toContain("pkg/history/committed.go");
    expect(snapshots[tree]).toContain("src/work");
    expect(snapshots[tree]).toContain("changed.go");
    expect(snapshots[tree]).toContain("pkg/history");
    expect(snapshots[tree]).not.toContain("src/work/changed.go");
    expect(snapshots[collapsed]).not.toContain("changed.go");
    expect(snapshots[expanded]).toContain("changed.go");
    for (const menu of [snapshots[context], snapshots[threeDot]]) {
      expect(menu).toContain("Open Current Changes");
      expect(menu).toContain("Git Files");
      expect(menu).toContain("Tree");
      expect(menu).toContain("List");
      expect(menu).toContain("Expand All");
      expect(menu).toContain("Collapse All");
    }
  });
});
