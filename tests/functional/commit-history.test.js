import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createGitRepo, createTempFile, git, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

// The resizable commit log starts at half the changes-panel height. At 120x40
// the branch header is row 22 and the newest commit is row 23 until the reader
// drags the divider.
const BRANCH_ROW = 22;
const FIRST_COMMIT_ROW = 23;

function repoWithTwoCommits() {
  const d = createGitRepo(createTempDir());
  createTempFile(d, "tracked.txt", "original\nsecond line\n");
  createTempFile(d, "added.txt", "brand new\n");
  git(d, "add", "-A");
  git(d, "commit", "-qm", "second commit");
  return d;
}

function repoWithLongHistory() {
  const d = createGitRepo(createTempDir());
  for (let i = 1; i <= 12; i++) {
    createTempFile(d, "history.txt", `revision ${i}\n`);
    git(d, "add", "-A");
    git(d, "commit", "-qm", `history ${String(i).padStart(2, "0")}`);
  }
  createTempFile(d, "history.txt", "working tree change\n");
  return d;
}

function repoWithDetailedCommit() {
  const d = createGitRepo(createTempDir());
  const lines = (prefix) => Array.from({ length: 24 }, (_, i) => `${prefix} line ${i + 1}`).join("\n") + "\n";
  createTempFile(d, "first-detail.txt", lines("first"));
  createTempFile(d, "second-detail.txt", lines("second"));
  git(d, "add", "-A");
  git(d, "commit", "-qm", "detail subject", "-m", "Full commit body paragraph.");
  return d;
}

function repoWithStickyDetail() {
  const d = createGitRepo(createTempDir());
  const lines = (prefix, count) => Array.from({ length: count }, (_, i) => `${prefix} line ${i + 1}`).join("\n") + "\n";
  createTempFile(d, "first-detail.txt", lines("first", 12));
  createTempFile(d, "second-detail.txt", lines("second", 60));
  git(d, "add", "-A");
  git(d, "commit", "-qm", "detail subject", "-m", "Full commit body paragraph.");
  return d;
}

function repoWithLongDetailLines() {
  const d = createGitRepo(createTempDir());
  createTempFile(d, "tracked.txt", `left-prefix-${"L".repeat(70)}-LEFT-SUFFIX\n`);
  git(d, "add", "-A");
  git(d, "commit", "-qm", "long detail");
  createTempFile(d, "tracked.txt", `right-prefix-${"R".repeat(70)}-RIGHT-SUFFIX\n`);
  git(d, "add", "-A");
  git(d, "commit", "-qm", "wrapped detail");
  return d;
}

describe("commit history", () => {
  it("should not focus commit history when small geometry does not render it", () => {
    dir = repoWithTwoCommits();

    tui.start(dir);
    tui.setSize(80, 10);
    tui.waitFor("Changes");
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(400);
    const hidden = tui.snapshot();

    tui.press("tab");
    tui.press("tab");
    tui.press("down");
    tui.press("enter");
    tui.waitStable(400);
    const afterKeyboard = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[hidden].match(/Commit History/g) ?? []).toHaveLength(0);
    expect(snapshots[afterKeyboard]).not.toMatch(/Commit [0-9a-f]{7}/);
  });

  it("should resize the history while preserving both trees and overflow navigation", () => {
    dir = repoWithLongHistory();

    tui.start(dir);
    tui.waitFor("Changes");
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(400);

    const initial = tui.snapshot();
    // Shrink past the minimum. The divider clamps near the bottom, leaving the title
    // and three log rows. Keyboard navigation must still scroll TreeWidget to
    // the oldest row.
    tui.drag(5, 20, 5, 38);
    tui.waitStable(300);
    const clampedHistory = tui.snapshot();
    tui.click(5, 36); // branch header inside the clamped history tree
    for (let i = 0; i < 10; i++) tui.press("down");
    tui.waitStable(300);
    const scrolled = tui.snapshot();

    // Grow history past the opposite limit. The input, divider, and three rows
    // of working-tree changes remain above it.
    tui.drag(5, 34, 5, 2);
    tui.waitStable(300);
    const clampedTree = tui.snapshot();

    const { snapshots } = tui.run();
    // The history query is capped at ten entries; a half-height default fits
    // the complete query at this geometry.
    expect(snapshots[initial]).toContain("history 03");
    expect(snapshots[clampedHistory]).toContain("Commit History");
    expect(snapshots[scrolled]).toContain("history 03");
    expect(snapshots[clampedTree]).toContain("❯ Commit to");
    expect(snapshots[clampedTree]).toContain("Changes (1)");
    expect(snapshots[clampedTree]).toContain("history.txt");
    expect(snapshots[clampedTree]).toContain("Commit History");
  });

  it("should list a commit's files when the commit is expanded", () => {
    dir = repoWithTwoCommits();

    tui.start(dir);
    tui.waitFor("Changes");
    tui.press("ctrl+k");
    tui.press("c");
    tui.waitStable(400);

    const collapsed = tui.snapshot();
    // Only the disclosure chevron expands; the label opens commit detail.
    tui.click(1, FIRST_COMMIT_ROW);
    tui.waitStable(400);
    const expanded = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[collapsed]).toContain("second commit");
    expect(snapshots[collapsed]).not.toContain("added.txt");
    expect(snapshots[expanded]).toContain("added.txt");
    expect(snapshots[expanded]).toContain("tracked.txt");
  });

  it("should open a diff for a file inside a commit", () => {
    dir = repoWithTwoCommits();

    tui.start(dir);
    tui.waitFor("Changes");
    tui.press("ctrl+k");
    tui.press("c");
    tui.waitStable(400);

    tui.click(1, FIRST_COMMIT_ROW);
    tui.waitStable(400);
    // First child of the expanded commit.
    tui.click(5, FIRST_COMMIT_ROW + 1);
    tui.waitStable(400);

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    // The tab is labelled "<file> @ <hash>" so two commits touching the same
    // file do not collapse into one tab.
    expect(snapshots[s0]).toMatch(/added\.txt @ [0-9a-f]{7}/);
    expect(snapshots[s0]).toContain("brand new");
  });

  it("should route registered diff commands to the focused commit file", () => {
    dir = repoWithTwoCommits();

    tui.start(dir);
    tui.waitFor("Changes");
    tui.press("ctrl+k");
    tui.press("c");
    tui.waitStable(400);

    // Clicking the commit expands it and focuses the history tree. Move onto
    // its first child without activating it, then use the same registered
    // command surface exposed by ttt.exec_command and the debug harness.
    tui.click(1, FIRST_COMMIT_ROW);
    tui.waitStable(400);
    tui.press("down");
    tui.exec("Git: Open Compact Diff");
    tui.waitStable(400);

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toMatch(/added\.txt @ [0-9a-f]{7}/);
    expect(snapshots[s0]).toContain("brand new");
  });

  it("should keep the commit-file context through the command palette", () => {
    dir = repoWithTwoCommits();

    tui.start(dir);
    tui.waitFor("Changes");
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(400);

    tui.click(1, FIRST_COMMIT_ROW);
    tui.waitStable(400);
    tui.press("down");
    tui.press("ctrl+p");
    tui.type("Git: Open Compact Diff");
    tui.press("enter");
    tui.waitStable(400);

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toMatch(/added\.txt @ [0-9a-f]{7}/);
    expect(snapshots[s0]).toContain("brand new");
  });

  it("should keep the branch header inert", () => {
    dir = repoWithTwoCommits();

    tui.start(dir);
    tui.waitFor("Changes");
    tui.press("ctrl+k");
    tui.press("c");
    tui.waitStable(400);

    tui.click(5, BRANCH_ROW);
    tui.waitStable(300);

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).not.toContain("added.txt");
    expect(snapshots[s0]).not.toContain("(diff)");
  });

  it("should open the full commit message and every file diff from the commit label", () => {
    dir = repoWithDetailedCommit();

    tui.start(dir);
    tui.waitFor("Changes");
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(400);

    // The label begins after the chevron and commit icon. Activating it opens
    // detail without changing disclosure state.
    tui.click(5, FIRST_COMMIT_ROW);
    tui.waitStable(800);
    const top = tui.snapshot();
    // Jump near the end through the detail view's global scrollbar. Editor
    // keybindings own End before content widgets see it.
    tui.click(118, 36);
    tui.waitStable(300);
    const bottom = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[top]).toMatch(/Commit [0-9a-f]{7}/);
    expect(snapshots[top]).toContain("detail subject");
    expect(snapshots[top]).toContain("Full commit body paragraph.");
    expect(snapshots[top]).toContain("first-detail.txt");
    expect(snapshots[top]).toContain("first line 1");
    expect(snapshots[top]).not.toContain("second line 24");
    expect(snapshots[bottom]).toContain("second-detail.txt");
    expect(snapshots[bottom]).toContain("second line 24");
  });

  it("should wrap and unwrap the active commit detail through the shared diff command", () => {
    dir = repoWithLongDetailLines();

    tui.start(dir);
    tui.waitFor("Changes");
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(400);
    tui.click(5, FIRST_COMMIT_ROW);
    tui.waitStable(800);

    const clipped = tui.snapshot();
    tui.exec("Git: Toggle Diff Wrap");
    tui.waitStable(300);
    const wrapped = tui.snapshot();
    tui.exec("Git: Toggle Diff Wrap");
    tui.waitStable(300);
    const restored = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[clipped]).not.toContain("LEFT-SUFFIX");
    expect(snapshots[clipped]).not.toContain("RIGHT-SUFFIX");
    expect(snapshots[wrapped]).toContain("LEFT-SUFFIX");
    expect(snapshots[wrapped]).toContain("RIGHT-SUFFIX");
    expect(snapshots[restored]).not.toContain("LEFT-SUFFIX");
    expect(snapshots[restored]).not.toContain("RIGHT-SUFFIX");
  });

  it("should keep an explicit unified choice made while commit detail is loading", () => {
    dir = repoWithLongDetailLines();

    tui.start(dir);
    tui.waitFor("Changes");
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(400);
    tui.click(5, FIRST_COMMIT_ROW);
    // The tab is active before its background Git read completes. Set the mode
    // immediately so the eventual detail must honor existing reader state.
    tui.exec("Git: Unified Diff");
    tui.waitStable(800);
    const unified = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[unified]).toContain("○ Split");
    expect(snapshots[unified]).toContain("● Unified");
    const rows = snapshots[unified].split("\n");
    const removed = rows.findIndex((row) => row.includes("left-prefix"));
    const added = rows.findIndex((row) => row.includes("right-prefix"));
    expect(removed).toBeGreaterThanOrEqual(0);
    expect(added).toBeGreaterThan(removed);
  });

  it("should collapse and expand commit files, including from the sticky heading", () => {
    dir = repoWithStickyDetail();

    tui.start(dir);
    tui.waitFor("Changes");
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(400);
    tui.click(5, FIRST_COMMIT_ROW);
    tui.waitStable(800);

    tui.exec("Git: Collapse All Commit Files");
    tui.waitStable(300);
    const collapsed = tui.snapshot();
    tui.exec("Git: Expand All Commit Files");
    tui.waitStable(300);
    const expanded = tui.snapshot();

    // At the bottom of the document, the second file's path remains visible
    // in the sticky heading while its real heading is above the viewport.
    tui.click(118, 36);
    tui.waitStable(300);
    const sticky = tui.snapshot();
    tui.exec("View: Focus Editor");
    tui.click(33, 4);
    tui.waitStable(300);
    const oneCollapsed = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[collapsed]).toContain("first-detail.txt");
    expect(snapshots[collapsed]).toContain("second-detail.txt");
    expect(snapshots[collapsed]).toContain("Expand all");
    expect(snapshots[collapsed]).not.toContain("first line 1");
    expect(snapshots[collapsed]).not.toContain("second line 1");
    expect(snapshots[expanded]).toContain("Collapse all");
    expect(snapshots[expanded]).toContain("first line 1");
    expect(snapshots[sticky].split("\n")[4]).toContain("second-detail.txt");
    expect(snapshots[oneCollapsed]).not.toContain("second line");
  });

  it("should copy original text across wrapped commit-detail rows", () => {
    dir = repoWithLongDetailLines();

    tui.start(dir);
    tui.waitFor("Changes");
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(400);
    tui.click(5, FIRST_COMMIT_ROW);
    tui.waitStable(800);
    tui.exec("Git: Toggle Diff Wrap");
    tui.waitStable(300);

    // Select left-side text from the first visual segment through the last.
    // Copy must join those visual segments back into the original source line.
    tui.drag(38, 8, 58, 10);
    tui.copy();
    tui.exec("New File");
    tui.press("ctrl+v");
    const atEnd = tui.snapshot();
    tui.press("home");
    const atStart = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[atEnd]).toContain("LEFT-SUFFIX");
    expect(snapshots[atStart]).toContain("ft-prefix-");
  });
});
