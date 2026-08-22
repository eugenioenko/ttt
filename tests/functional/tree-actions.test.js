import { afterEach, describe, expect, it } from "vitest";
import { mkdirSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { cleanupDir, createGitRepo, createTempDir, createTempFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("tree actions", () => {
  it("expands represented Explorer folders progressively and collapses them together", () => {
    dir = createGitRepo(createTempDir());
    mkdirSync(join(dir, "src", "nested"), { recursive: true });
    createTempFile(dir, "src/nested/file.txt", "hello\n");

    tui.start(dir);
    tui.pressChord("ctrl+k", "e");
    tui.waitStable();
    tui.exec("Explorer: Collapse All");
    const collapsed = tui.snapshot();
    tui.exec("Explorer: Expand All");
    const rootExpanded = tui.snapshot();
    tui.exec("Explorer: Expand All");
    const nestedExpanded = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[collapsed]).not.toContain("src");
    expect(snapshots[rootExpanded]).toContain("src");
    expect(snapshots[nestedExpanded]).toContain("nested");
  });
});
