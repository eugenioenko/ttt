import { describe, it, expect, afterEach } from "vitest";
import { mkdirSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, createGitRepo, createTempFile, git, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

function repoWithNestedGitFiles() {
  const d = createGitRepo(createTempDir());
  mkdirSync(join(d, "pkg/history"), { recursive: true });
  createTempFile(d, "pkg/history/committed.go", "package history\n");
  git(d, "add", "-A");
  git(d, "commit", "-qm", "nested commit");

  mkdirSync(join(d, "src/work"), { recursive: true });
  createTempFile(d, "src/work/changed.go", "package work\n");
  return d;
}

describe("git file tree", () => {
  it("groups working and commit files and can switch back to full-path lists", () => {
    dir = repoWithNestedGitFiles();

    tui.start(dir);
    tui.waitFor("Changes");
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(400);
    // Expand the newest commit. The resizable history begins at row 22, with
    // the branch header at 22 and the newest commit at 23 at this geometry.
    tui.click(1, 23);
    tui.waitStable(400);
    const tree = tui.snapshot();

    // Open the Changes panel's contextual three-dot menu, then use its nested
    // Git Files group to switch to List view.
    tui.click(29, 2);
    tui.waitStable();
    const panelMenu = tui.snapshot();
    // Skip the new Expand All and Collapse All actions to reach Git Files.
    tui.press("down");
    tui.press("down");
    tui.press("down");
    tui.press("right");
    tui.waitStable();
    const gitFilesMenu = tui.snapshot();
    tui.press("down");
    tui.press("enter");
    tui.waitStable(300);
    const list = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[tree]).toContain("src/work");
    expect(snapshots[tree]).toContain("changed.go");
    expect(snapshots[tree]).toContain("pkg/history");
    expect(snapshots[tree]).toContain("committed.go");
    expect(snapshots[tree]).not.toContain("src/work/changed.go");
    expect(snapshots[tree]).not.toContain("pkg/history/committed.go");

    expect(snapshots[panelMenu]).toContain("Git Files");
    expect(snapshots[panelMenu]).toContain("Diff View");
    expect(snapshots[gitFilesMenu]).toContain("Tree");
    expect(snapshots[gitFilesMenu]).toContain("List");

    expect(snapshots[list]).toContain("src/work/changed.go");
    expect(snapshots[list]).toContain("pkg/history/committed.go");
  });
});
