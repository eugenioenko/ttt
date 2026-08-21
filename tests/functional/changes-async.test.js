import { describe, it, expect, afterEach } from "vitest";
import { chmodSync, writeFileSync, mkdirSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, createGitRepo, createTempFile, git, cleanupDir } from "./helpers.js";

let dir;
let binDir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
  if (binDir) cleanupDir(binDir);
});

// slowGitOn returns a directory holding a `git` that sleeps before delegating to
// the real one, for the listed subcommands. Putting it first on PATH is the only
// way to observe whether the editor waits for git or carries on without it.
// The delay has to comfortably outlast the "during" snapshot and comfortably
// undershoot the wait after it, on a machine running the rest of the suite in
// parallel. Hence generous margins on both sides rather than tight ones.
function slowGitOn(subcommands, seconds = 3) {
  const d = createTempDir();
  mkdirSync(d, { recursive: true });
  const script = [
    "#!/bin/sh",
    'for a in "$@"; do',
    `  case "$a" in ${subcommands.join("|")}) sleep ${seconds} ;; esac`,
    "done",
    'exec /usr/bin/git "$@"',
    "",
  ].join("\n");
  const path = join(d, "git");
  writeFileSync(path, script, "utf8");
  chmodSync(path, 0o755);
  return d;
}

// The committed file and the working-tree files have different names on
// purpose: the changes tree and the expanded commit render into the same panel,
// so a shared name would make every "not yet visible" assertion meaningless.
function repoWithChanges() {
  const d = createGitRepo(createTempDir());
  createTempFile(d, "committed.txt", "in the last commit\n");
  git(d, "add", "-A");
  git(d, "commit", "-qm", "second commit");
  createTempFile(d, "tracked.txt", "original\nchanged\n");
  createTempFile(d, "untracked.txt", "new\n");
  return d;
}

describe("changes panel does not wait for git", () => {
  it("should draw the panel before the working-tree scan finishes", () => {
    dir = repoWithChanges();
    binDir = slowGitOn(["status"]);

    tui.start(dir);
    tui.setEnv({ PATH: `${binDir}:${process.env.PATH}` });
    tui.waitStable(300);
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(400);
    // git status is still sleeping here. Reading it on the event path would
    // mean nothing has been drawn yet.
    const during = tui.snapshot();
    tui.waitStable(6000);
    const after = tui.snapshot();

    const { snapshots } = tui.run(45000);
    // Drawn, and drawn empty: the scan has not come back yet.
    expect(snapshots[during]).toContain("No changes");
    expect(snapshots[during]).not.toContain("untracked.txt");
    expect(snapshots[after]).toContain("tracked.txt");
    expect(snapshots[after]).toContain("untracked.txt");
  });

  it("should show a placeholder while a commit's files are read", () => {
    dir = repoWithChanges();
    binDir = slowGitOn(["show"]);

    tui.start(dir);
    tui.setEnv({ PATH: `${binDir}:${process.env.PATH}` });
    tui.waitStable(300);
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(600);
    tui.click(1, 23);
    tui.waitStable(400);
    const loading = tui.snapshot();
    tui.waitStable(6000);
    const loaded = tui.snapshot();

    const { snapshots } = tui.run(45000);
    expect(snapshots[loading]).toContain("Loading");
    expect(snapshots[loading]).not.toContain("committed.txt");
    expect(snapshots[loaded]).toContain("committed.txt");
  });

  it("should show a usable loading tab while the full commit is read", () => {
    dir = repoWithChanges();
    binDir = slowGitOn(["show"], 2);

    tui.start(dir);
    tui.setEnv({ PATH: `${binDir}:${process.env.PATH}` });
    tui.waitStable(300);
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(600);
    // The label activates commit detail; unlike the chevron at x=1, it must
    // not wait for the message, file list, and file diffs before returning.
    tui.click(5, 23);
    tui.waitStable(400);
    const loading = tui.snapshot();
    tui.waitStable(9000);
    const loaded = tui.snapshot();

    const { snapshots } = tui.run(45000);
    expect(snapshots[loading]).toContain("Loading commit");
    expect(snapshots[loading]).toContain("Commit History");
    expect(snapshots[loading]).not.toContain("committed.txt");
    expect(snapshots[loaded]).toContain("second commit");
    expect(snapshots[loaded]).toContain("committed.txt");
    expect(snapshots[loaded]).toContain("in the last commit");
  });

  it("should keep the editor drawn while a working-tree diff is read", () => {
    dir = repoWithChanges();
    // Only `git diff` is delayed, so opening the panel is quick and the wait
    // that matters is the one the click starts.
    binDir = slowGitOn(["diff"]);

    tui.start(dir);
    tui.setEnv({ PATH: `${binDir}:${process.env.PATH}` });
    tui.waitStable(300);
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(600);
    // Row 7 is the modified file under "Changes"; row 8 is the untracked one,
    // which opens immediately and never reads a diff.
    tui.click(5, 7);
    tui.waitStable(400);
    const during = tui.snapshot();
    tui.waitStable(6000);
    const after = tui.snapshot();

    const { snapshots } = tui.run(45000);
    // Still drawn, and saying so, while git diff runs.
    expect(snapshots[during]).toContain("Opening diff");
    expect(snapshots[during]).not.toContain("(diff)");
    expect(snapshots[after]).toContain("(diff)");
    expect(snapshots[after]).not.toContain("Opening diff");
  });

  it("should not replace a tab chosen while a diff is read", () => {
    dir = repoWithChanges();
    binDir = slowGitOn(["diff"]);

    // Start on untracked.txt with tracked.txt available as a second existing
    // tab. The click below starts tracked.txt's slow diff; the tab click then
    // chooses the plain tracked.txt buffer while that read is still running.
    tui.start(join(dir, "tracked.txt"), join(dir, "untracked.txt"), dir);
    tui.setEnv({ PATH: `${binDir}:${process.env.PATH}` });
    tui.waitStable(300);
    tui.pressChord("ctrl+k", "c");
    tui.waitStable(600);
    tui.click(5, 7);
    tui.waitStable(400);
    const pending = tui.snapshot();
    tui.click(48, 2);
    tui.waitStable(300);
    const switched = tui.snapshot();
    tui.waitStable(6000);
    const after = tui.snapshot();

    const { snapshots } = tui.run(45000);
    expect(snapshots[pending]).toContain("Opening diff");
    expect(snapshots[switched]).toContain("changed");
    expect(snapshots[switched]).not.toContain("tracked.txt (diff)");
    expect(snapshots[after]).toContain("changed");
    expect(snapshots[after]).not.toContain("tracked.txt (diff)");
  });

  it("should draw the status bar before the branch name is known", () => {
    dir = repoWithChanges();
    // Only `git rev-parse --abbrev-ref` — the branch lookup — is delayed.
    // Delaying every rev-parse would also stall the explorer, which is still
    // synchronous and would mask what this test is about.
    binDir = slowGitOn(["--abbrev-ref"]);
    git(dir, "checkout", "-q", "-b", "slow-branch-check");

    // Both: the segment needs an active file to attribute to a repository, and
    // the workspace needs the folder for that repository to be known. With only
    // the folder and no open file the branch is correctly blank.
    tui.start(join(dir, "tracked.txt"), dir);
    tui.setEnv({ PATH: `${binDir}:${process.env.PATH}` });
    tui.waitStable(300);
    const during = tui.snapshot();
    tui.waitStable(6000);
    const after = tui.snapshot();

    const { snapshots } = tui.run(45000);
    // The status bar is up — it just has no branch in it yet.
    expect(snapshots[during]).toContain("Ln 1, Col 1");
    expect(snapshots[during]).not.toContain("slow-branch-check");
    expect(snapshots[after]).toContain("slow-branch-check");
  });
});
