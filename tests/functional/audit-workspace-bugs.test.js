import { describe, it, expect, afterEach } from "vitest";
import { execFileSync } from "node:child_process";
import { chmodSync, readFileSync, writeFileSync, mkdirSync, symlinkSync } from "node:fs";
import { delimiter, join } from "node:path";
import * as tui from "./tui.js";
import { createGitRepo, createLinkedWorktree, createTempDir, cleanupDir, git } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

function createExternalFileSymlinkFixture() {
  dir = createTempDir();
  const repo = join(dir, "repo");
  mkdirSync(repo);
  createGitRepo(repo);
  git(repo, "branch", "-m", "filesymlinkbranch");

  const nested = join(repo, "nested");
  mkdirSync(nested);
  const target = join(nested, "file.txt");
  writeFileSync(target, "symlink target\n");
  git(repo, "add", "nested/file.txt");
  git(repo, "commit", "-qm", "add symlink target");

  const plain = join(dir, "plain");
  mkdirSync(plain);
  const link = join(plain, "file-link.txt");
  symlinkSync(target, link);
  return { target, link };
}

describe("BUG-044: git branch indicator missing when the opened file is below the repo root", () => {
  it("status bar shows the branch for a file in a repo subdirectory", () => {
    dir = createTempDir();
    // Make `dir` a git repo with a subdirectory and a distinctively-named
    // branch so the assertion can't match incidental "main" text elsewhere.
    const git = (...args) =>
      execFileSync("git", ["-C", dir, ...args], { stdio: "ignore" });
    git("init", "-q", "-b", "auditbranch");
    git("config", "user.email", "a@a.com");
    git("config", "user.name", "a");
    mkdirSync(join(dir, "sub"));
    const subfile = join(dir, "sub", "f.txt");
    writeFileSync(subfile, "content\n");
    git("add", "-A");
    git("commit", "-qm", "init");

    // Open the subdir file by absolute path → workspace folder becomes the
    // file's parent (dir/sub), which has no .git of its own.
    tui.start(subfile);
    tui.waitFor("content");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s]).toContain("auditbranch");
  });

  it("uses linked-worktree identity and clears it on a non-repository tab", () => {
    dir = createTempDir();
    const { worktree } = createLinkedWorktree(dir, "linkedbranch");
    expect(readFileSync(join(worktree, ".git"), "utf8")).toMatch(/^gitdir: /);

    const nested = join(worktree, "nested");
    mkdirSync(nested);
    const linkedFile = join(nested, "linked.txt");
    writeFileSync(linkedFile, "linked content\n");

    const plainDir = join(dir, "plain");
    mkdirSync(plainDir);
    const plainFile = join(plainDir, "plain.txt");
    writeFileSync(plainFile, "plain content\n");

    tui.start(plainFile, linkedFile);
    tui.waitFor("linkedbranch");
    const linked = tui.snapshot();
    tui.exec("View: Previous Tab");
    tui.waitStable();
    const plain = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[linked]).toContain("linkedbranch");
    expect(snapshots[plain]).not.toContain("linkedbranch");
  });

  it("shows target repository branch and blame without false gutter markers through an external file symlink", () => {
    const { link } = createExternalFileSymlinkFixture();

    tui.start(link);
    tui.waitFor("filesymlinkbranch");
    tui.waitFor("Test User");
    tui.waitStable();
    const screen = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[screen]).toContain("filesymlinkbranch");
    expect(snapshots[screen]).toContain("Test User");
    expect(snapshots[screen]).not.toContain("│▎");
  });

  it("shows a modified tracked target gutter marker through an external file symlink", () => {
    const { target, link } = createExternalFileSymlinkFixture();
    writeFileSync(target, "modified target\n");

    tui.start(link);
    tui.waitFor("filesymlinkbranch");
    tui.waitFor("│▎");
    const screen = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[screen]).toContain("modified target");
    expect(snapshots[screen]).toContain("│▎");
  });

  it("drops delayed external-symlink blame after switching to a non-repository tab", () => {
    const { link } = createExternalFileSymlinkFixture();
    const plainFile = join(dir, "plain", "plain.txt");
    writeFileSync(plainFile, "plain content\n");

    const realGit = execFileSync("which", ["git"], { encoding: "utf8" }).trim();
    const binDir = join(dir, "bin");
    mkdirSync(binDir);
    const gitWrapper = join(binDir, "git");
    writeFileSync(
      gitWrapper,
      '#!/bin/sh\nif [ "$3" = "blame" ]; then sleep 1; fi\nexec "$TTT_REAL_GIT" "$@"\n',
    );
    chmodSync(gitWrapper, 0o755);

    tui.start(plainFile, link);
    tui.setEnv({ PATH: `${binDir}${delimiter}${process.env.PATH}`, TTT_REAL_GIT: realGit });
    tui.waitFor("filesymlinkbranch");
    tui.exec("View: Previous Tab");
    tui.wait(1300);
    const screen = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[screen]).toContain("plain content");
    expect(snapshots[screen]).not.toContain("filesymlinkbranch");
    expect(snapshots[screen]).not.toContain("Test User");
  });
});
