import { describe, it, expect, afterEach } from "vitest";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir, createGitRepo, git, gitStatus, gitLog, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

// The Changes panel's action icons are not reachable through synthetic clicks,
// so these drive the keyboard equivalents: a = stage all, u = unstage all,
// D = discard all.
function openChanges() {
  tui.exec("Show Changes");
  tui.waitStable();
}

describe("git changes panel", () => {
  it("renders raw Unicode Git paths and stages the selected nested basename exactly", () => {
    dir = createTempDir();
    createGitRepo(dir);
    writeFileSync(join(dir, "界-wide.txt"), "wide\n", "utf8");
    git(dir, "add", "--", "界-wide.txt");
    git(dir, "commit", "-qm", "wide path");
    writeFileSync(join(dir, "界-wide.txt"), "changed\n", "utf8");
    mkdirSync(join(dir, "新規", "深い"), { recursive: true });
    writeFileSync(join(dir, "新規", "深い", "同名.go"), "package exact\n", "utf8");

    tui.start(dir);
    openChanges();
    tui.exec("View Git Files as Tree");
    const rendered = tui.snapshot();
    tui.press("down");
    tui.press("down");
    tui.exec("Git: Stage File");
    tui.waitStable();

    const { snapshots } = tui.run();
    expect(snapshots[rendered]).toContain("界-wide.txt");
    expect(snapshots[rendered]).toContain("新規/深い");
    expect(snapshots[rendered]).toContain("同名.go");
    expect(git(dir, "diff", "--cached", "--name-only", "-z")).toBe("新規/深い/同名.go\0");
  });

  it("should stage every file in one action", () => {
    dir = createTempDir();
    createGitRepo(dir);
    writeFileSync(join(dir, "tracked.txt"), "modified\n", "utf8");
    writeFileSync(join(dir, "untracked.txt"), "new\n", "utf8");

    tui.start(dir);
    tui.waitFor("Explore");
    openChanges();

    const before = tui.snapshot();
    tui.press("a");
    tui.waitStable();
    const after = tui.snapshot();

    const { snapshots } = tui.run();

    expect(snapshots[before]).toContain("Changes (2)");
    expect(snapshots[after]).toContain("Staged (2)");

    // Both kinds of change stage: a tracked modification and a new file.
    const status = gitStatus(dir).sort();
    expect(status).toEqual(["A  untracked.txt", "M  tracked.txt"]);
  });

  it("should unstage every file in one action", () => {
    dir = createTempDir();
    createGitRepo(dir);
    writeFileSync(join(dir, "tracked.txt"), "modified\n", "utf8");
    writeFileSync(join(dir, "untracked.txt"), "new\n", "utf8");

    tui.start(dir);
    tui.waitFor("Explore");
    openChanges();

    tui.press("a");
    tui.waitStable();
    tui.press("u");
    tui.waitStable();
    const after = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[after]).toContain("Changes (2)");

    const status = gitStatus(dir).sort();
    expect(status).toEqual([" M tracked.txt", "?? untracked.txt"]);
  });

  it("should commit staged changes", () => {
    dir = createTempDir();
    createGitRepo(dir);
    writeFileSync(join(dir, "tracked.txt"), "modified\n", "utf8");

    tui.start(dir);
    tui.waitFor("Explore");
    openChanges();

    tui.press("a");
    tui.waitStable();
    // Tab moves focus from the file tree to the commit message input.
    tui.press("tab");
    tui.type("a commit from the editor");
    tui.press("enter");
    tui.waitStable();

    const after = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[after]).toContain("No changes");
    expect(gitLog(dir)[0]).toBe("a commit from the editor");
    expect(gitStatus(dir)).toEqual([]);
  });

  it("should report the commit in the output panel", () => {
    dir = createTempDir();
    createGitRepo(dir);
    writeFileSync(join(dir, "tracked.txt"), "modified\n", "utf8");

    tui.start(dir);
    tui.waitFor("Explore");
    openChanges();

    tui.press("a");
    tui.waitStable();
    tui.press("tab");
    tui.type("logged commit");
    tui.press("enter");
    tui.waitStable();

    tui.exec("Output: Show Panel");
    tui.waitStable();
    const output = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[output]).toContain("[notice] Committed: logged commit");
  });

  // Discarding splits into two batched calls: untracked files are deleted,
  // tracked ones are restored from HEAD.
  it("should discard tracked and untracked changes together", () => {
    dir = createTempDir();
    createGitRepo(dir);
    writeFileSync(join(dir, "tracked.txt"), "modified\n", "utf8");
    writeFileSync(join(dir, "untracked.txt"), "new\n", "utf8");

    tui.start(dir);
    tui.waitFor("Explore");
    openChanges();

    tui.type("D");
    tui.waitStable();
    const dialog = tui.snapshot();
    // The dialog defaults to Cancel; move to Discard before confirming.
    tui.press("right");
    tui.press("enter");
    tui.waitStable();
    const after = tui.snapshot();

    const { snapshots } = tui.run();

    expect(snapshots[dialog]).toContain("irreversible");
    expect(snapshots[after]).toContain("No changes");
    expect(gitStatus(dir)).toEqual([]);
  });

  // Only untracked files means the tracked list handed to git.Discard is empty,
  // which must be a no-op rather than an argument-less git invocation.
  it("should discard when every change is untracked", () => {
    dir = createTempDir();
    createGitRepo(dir);
    writeFileSync(join(dir, "one.txt"), "1\n", "utf8");
    writeFileSync(join(dir, "two.txt"), "2\n", "utf8");

    tui.start(dir);
    tui.waitFor("Explore");
    openChanges();

    tui.type("D");
    tui.waitStable();
    tui.press("right");
    tui.press("enter");
    tui.waitStable();
    const after = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[after]).toContain("No changes");
    expect(gitStatus(dir)).toEqual([]);
    // The committed file must survive a discard that only targeted new files.
    expect(readFile(join(dir, "tracked.txt"))).toBe("original\n");
  });

  it("should stage and unstage a single file", () => {
    dir = createTempDir();
    createGitRepo(dir);
    writeFileSync(join(dir, "tracked.txt"), "modified\n", "utf8");
    writeFileSync(join(dir, "untracked.txt"), "new\n", "utf8");

    tui.start(dir);
    tui.waitFor("Explore");
    openChanges();

    tui.press("down");
    tui.exec("Git: Stage File");
    tui.waitStable();
    const staged = tui.snapshot();

    tui.exec("Git: Unstage File");
    tui.waitStable();
    const unstaged = tui.snapshot();

    const { snapshots } = tui.run();

    // Only one of the two files moved.
    expect(snapshots[staged]).toContain("Staged (1)");
    expect(snapshots[staged]).toContain("Changes (1)");
    expect(snapshots[unstaged]).toContain("Changes (2)");
    expect(snapshots[unstaged]).not.toContain("Staged");
  });

  it("should toggle staging with the space key", () => {
    dir = createTempDir();
    createGitRepo(dir);
    writeFileSync(join(dir, "tracked.txt"), "modified\n", "utf8");

    tui.start(dir);
    tui.waitFor("Explore");
    openChanges();

    tui.press("down");
    tui.press("space");
    tui.waitStable();
    const on = tui.snapshot();

    tui.press("space");
    tui.waitStable();
    const off = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[on]).toContain("Staged (1)");
    expect(snapshots[off]).toContain("Changes (1)");
    expect(gitStatus(dir)).toEqual([" M tracked.txt"]);
  });

  it("should discard a single file and leave the others alone", () => {
    dir = createTempDir();
    createGitRepo(dir);
    writeFileSync(join(dir, "tracked.txt"), "modified\n", "utf8");
    writeFileSync(join(dir, "untracked.txt"), "new\n", "utf8");

    tui.start(dir);
    tui.waitFor("Explore");
    openChanges();

    tui.press("down");
    tui.type("d");
    tui.waitStable();
    tui.press("right");
    tui.press("enter");
    tui.waitStable();

    const { snapshots } = tui.run();
    void snapshots;

    // One change discarded, the other untouched.
    expect(gitStatus(dir)).toEqual(["?? untracked.txt"]);
    expect(readFile(join(dir, "tracked.txt"))).toBe("original\n");
  });

  it("should clear the commit message after committing", () => {
    dir = createTempDir();
    createGitRepo(dir);
    writeFileSync(join(dir, "tracked.txt"), "modified\n", "utf8");

    tui.start(dir);
    tui.waitFor("Explore");
    openChanges();

    tui.press("a");
    tui.waitStable();
    tui.press("tab");
    tui.type("first message");
    tui.press("enter");
    tui.waitStable();
    const after = tui.snapshot();

    const { snapshots } = tui.run();
    // The placeholder only renders when the input is empty. Asserting the
    // message is absent from the screen would not work: the new commit shows
    // up in the log list below the input under its own subject.
    expect(snapshots[after]).toContain("❯ Commit to");
    expect(snapshots[after]).toContain("● first message");
  });

  it("should report a failing push instead of silently doing nothing", () => {
    dir = createTempDir();
    createGitRepo(dir);

    tui.start(dir);
    tui.waitFor("Explore");

    // The repo has no remote, so the push fails inside the background task.
    tui.exec("Git: Push");
    tui.waitStable();
    tui.waitStable();
    const after = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[after]).toContain("Pushing failed");
  });
});
