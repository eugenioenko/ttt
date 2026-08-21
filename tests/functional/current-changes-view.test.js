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
  it("shows the final working tree in one combined scrollable view", () => {
    dir = createGitRepo(createTempDir());
    const tracked = join(dir, "tracked.txt");
    writeFileSync(tracked, "staged version\n", "utf8");
    execFileSync("git", ["-C", dir, "add", "tracked.txt"]);
    writeFileSync(tracked, "final working version\n", "utf8");

    tui.start(dir);
    tui.waitFor("Explore");
    tui.exec("Git: View All Current Changes");
    tui.waitStable(600);
    const screen = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[screen]).toContain("Current changes");
    expect(snapshots[screen]).toContain("M  tracked.txt · mixed");
    expect(snapshots[screen]).toContain("final working version");
  });
});
