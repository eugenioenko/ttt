import { afterEach, describe, expect, it } from "vitest";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { cleanupDir, createGitRepo, createTempDir, createTempFile, git, gitStatus } from "./helpers.js";

let dir;
let configDir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
  if (configDir) cleanupDir(configDir);
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
    // File icons take two columns and would truncate the full paths this test reads.
    configDir = createTempDir();
    writeFileSync(join(configDir, "settings.json"), JSON.stringify({ git: { icons: "none" } }));
    tui.start(dir);
    tui.setEnv({ TTT_CONFIG_DIR: configDir });
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

    // A folder row owns its own context menu now; the panel menu stays in the
    // same list and on the panel's ⋮ button.
    tui.rclick(5, 5);
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
    expect(snapshots[context]).toContain("Stage Folder");
    expect(snapshots[context]).toContain("Discard Folder Changes");
    expect(snapshots[context]).toContain("Open Current Changes");
    expect(snapshots[context]).toContain("Git Files");
    expect(snapshots[threeDot]).toContain("Tree");
    expect(snapshots[threeDot]).toContain("List");
    expect(snapshots[threeDot]).toContain("Expand All");
    expect(snapshots[threeDot]).toContain("Collapse All");
  });

  it("stages every file under a folder row from the tree", () => {
    dir = nestedRepo();
    configDir = createTempDir();
    writeFileSync(join(configDir, "settings.json"), JSON.stringify({ git: { fileView: "tree", icons: "none" } }));
    tui.start(dir);
    tui.setEnv({ TTT_CONFIG_DIR: configDir });
    tui.pressChord("ctrl+k", "c");
    tui.waitFor("Changes (1)");

    tui.rclick(5, 5);
    tui.waitFor("Stage Folder");
    tui.press("enter");
    tui.waitFor("Staged (1)");
    const staged = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[staged]).toContain("Staged (1)");
    expect(gitStatus(dir)).toEqual(["A  src/work/changed.go"]);
  });
});
