import { afterEach, describe, expect, it } from "vitest";
import * as tui from "./tui.js";
import { cleanupDir, createGitRepo, createTempDir, createTempFile, git } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("commit history detail", () => {
  it("opens one read-only document with metadata and every changed file", () => {
    dir = createGitRepo(createTempDir());
    createTempFile(dir, "first-detail.txt", "first old\n");
    createTempFile(dir, "second-detail.txt", "second old\n");
    git(dir, "add", "-A");
    git(dir, "commit", "-qm", "detail subject", "-m", "Full detail body.");

    tui.start(dir);
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(500);
    // Changes focus starts in the working tree. The next two stops are the
    // commit input and the responsive Commit History tree.
    tui.press("tab");
    tui.press("tab");
    tui.press("down");
    tui.press("enter");
    tui.waitStable(900);
    const detail = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[detail]).toMatch(/Commit [0-9a-f]{7,}/);
    expect(snapshots[detail]).toContain("detail subject");
    expect(snapshots[detail]).toContain("Full detail body.");
    expect(snapshots[detail]).toMatch(/Authored [A-Z][a-z]{2} \d{1,2}, \d{4} at \d{1,2}:\d{2}:\d{2} [AP]M [+-]\d{4}/);
    expect(snapshots[detail]).toContain("first-detail.txt");
    expect(snapshots[detail]).toContain("first old");
    expect(snapshots[detail]).toContain("second-detail.txt");
    expect(snapshots[detail]).toContain("second old");
  });

  it("uses shared Git bulk commands for commit-detail file groupings", () => {
    dir = createGitRepo(createTempDir());
    createTempFile(dir, "bulk-detail.txt", "visible detail line\n");
    git(dir, "add", "-A");
    git(dir, "commit", "-qm", "bulk detail");

    tui.start(dir);
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(500);
    tui.press("tab");
    tui.press("tab");
    tui.press("down");
    tui.press("enter");
    tui.waitStable(900);
    const expanded = tui.snapshot();
    tui.exec("Git: Collapse All File Trees");
    const collapsed = tui.snapshot();
    tui.exec("Git: Expand All File Trees");
    const restored = tui.snapshot();
    tui.rclick(40, 12);
    const context = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[expanded]).toContain("visible detail line");
    expect(snapshots[collapsed]).not.toContain("visible detail line");
    expect(snapshots[restored]).toContain("visible detail line");
    expect(snapshots[context]).toContain("Expand All Files");
    expect(snapshots[context]).toContain("Collapse All Files");
  });
});
