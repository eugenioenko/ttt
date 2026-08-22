import { afterEach, describe, expect, it } from "vitest";
import { execFileSync } from "node:child_process";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { cleanupDir, createGitRepo, createTempDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("current changes document", () => {
  it("shows both mixed index boundaries in one stable shared diff tab", () => {
    dir = createGitRepo(createTempDir());
    const tracked = join(dir, "tracked.txt");
    writeFileSync(tracked, "staged version\n", "utf8");
    execFileSync("git", ["-C", dir, "add", "tracked.txt"]);
    writeFileSync(tracked, "final working version 界\n", "utf8");

    tui.start(dir);
    tui.exec("Git: Open Current Changes");
    tui.waitStable(500);
    const first = tui.snapshot();
    tui.exec("Git: Open Current Changes");
    tui.waitStable(300);
    const reopened = tui.snapshot();
    const { snapshots } = tui.run();

    for (const screen of [snapshots[first], snapshots[reopened]]) {
      expect(screen).toContain("Current changes");
      expect(screen).toContain("M  tracked.txt · staged");
      expect(screen).toContain("M  tracked.txt · unstaged");
      expect(screen).toContain("staged version");
      expect(screen).toContain("final working version 界");
      expect(screen.match(/Current Changes x/g)).toHaveLength(1);
    }
  });

  it("shows a UD merge conflict as one explicit two-way resolution section", () => {
    dir = createGitRepo(createTempDir());
    const tracked = join(dir, "tracked.txt");
    execFileSync("git", ["-C", dir, "checkout", "-qb", "other"]);
    execFileSync("git", ["-C", dir, "rm", "--", "tracked.txt"]);
    execFileSync("git", ["-C", dir, "commit", "-m", "other deletes"]);
    execFileSync("git", ["-C", dir, "checkout", "-q", "main"]);
    writeFileSync(tracked, "main content\n", "utf8");
    execFileSync("git", ["-C", dir, "commit", "-am", "main modifies"]);
    try {
      execFileSync("git", ["-C", dir, "merge", "other"], { stdio: "pipe" });
    } catch {}

    tui.start(dir);
    tui.exec("Git: Open Current Changes");
    tui.waitStable(500);
    const conflict = tui.snapshot();
    const { snapshots } = tui.run();
    const screen = snapshots[conflict];

    expect(screen).toContain("U  tracked.txt - conflict (UD)");
    expect(screen).toContain("No line changes");
    expect(screen).not.toContain("tracked.txt · staged");
    expect(screen).not.toContain("tracked.txt · unstaged");
  });
});
