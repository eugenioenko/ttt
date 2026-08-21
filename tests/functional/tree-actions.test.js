import { describe, it, expect, afterEach } from "vitest";
import { mkdirSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, createGitRepo, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("tree actions", () => {
  it("expands and collapses Git file trees and exposes panel actions on right click", () => {
    dir = createGitRepo(createTempDir());
    mkdirSync(join(dir, "src", "nested"), { recursive: true });
    createTempFile(dir, "src/nested/change.txt", "new file\n");

    tui.start(dir);
    tui.waitFor("Changes");
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(500);
    const expanded = tui.snapshot();
    tui.exec("Git: Collapse All File Trees");
    tui.waitStable();
    const collapsed = tui.snapshot();
    tui.exec("Git: Expand All File Trees");
    tui.waitStable();
    const restored = tui.snapshot();
    tui.rclick(5, 5);
    tui.waitStable();
    const menu = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[expanded]).toContain("change.txt");
    expect(snapshots[collapsed]).not.toContain("change.txt");
    expect(snapshots[restored]).toContain("change.txt");
    expect(snapshots[menu]).toContain("Expand All");
    expect(snapshots[menu]).toContain("Collapse All");
    expect(snapshots[menu]).toContain("Git Files");
    expect(snapshots[menu]).toContain("Diff View");
  });

  it("expands represented Explorer folders progressively and collapses them together", () => {
    dir = createGitRepo(createTempDir());
    mkdirSync(join(dir, "src", "nested"), { recursive: true });
    createTempFile(dir, "src/nested/file.txt", "hello\n");

    tui.start(dir);
    tui.pressChord("ctrl+k", "e");
    tui.waitStable();
    tui.exec("Explorer: Collapse All");
    tui.waitStable();
    const collapsed = tui.snapshot();
    tui.exec("Explorer: Expand All");
    tui.waitStable();
    const rootExpanded = tui.snapshot();
    tui.exec("Explorer: Expand All");
    tui.waitStable();
    const nestedExpanded = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[collapsed]).not.toContain("src");
    expect(snapshots[rootExpanded]).toContain("src");
    expect(snapshots[nestedExpanded]).toContain("nested");
  });
});
