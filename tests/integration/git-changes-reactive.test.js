import { afterEach, describe, expect, it } from "vitest";
import { execFileSync } from "node:child_process";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { cleanupDir, createTempDir } from "./helpers.js";

let dir;

function git(...args) {
  return execFileSync("git", ["-C", dir, ...args], { encoding: "utf8" });
}

function createRepository() {
  dir = createTempDir();
  git("init", "-q", "-b", "main");
  git("config", "user.email", "test@test.com");
  git("config", "user.name", "Test User");
  git("config", "commit.gpgsign", "false");
  writeFileSync(join(dir, "tracked.txt"), "original\n", "utf8");
  git("add", "tracked.txt");
  git("commit", "-qm", "initial commit");
}

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("reactive git polling", () => {
  it("detects an external change to an unopened file while Changes is visible", () => {
    createRepository();

    tui.start(dir);
    tui.waitFor("Explore");
    tui.exec("Show Changes");
    tui.waitFor("No changes");

    writeFileSync(join(dir, "tracked.txt"), "external\n", "utf8");
    tui.waitFor("tracked.txt");

    const screen = tui.snapshot();
    expect(screen).toContain("Changes (1)");
    expect(screen).toContain("tracked.txt");
  });

  it("keeps Current Changes live across same-status edits and close/reopen", () => {
    createRepository();

    tui.start(dir);
    tui.waitFor("Explore");
    tui.exec("Git: Open Current Changes");
    tui.waitFor("Working tree clean");

    writeFileSync(join(dir, "tracked.txt"), "external-one\n", "utf8");
    tui.waitFor("external-one");
    writeFileSync(join(dir, "tracked.txt"), "external-two\n", "utf8");
    tui.waitFor("external-two");

    tui.press("ctrl+w");
    tui.waitFor("untitled");
    writeFileSync(join(dir, "tracked.txt"), "while-closed\n", "utf8");
    tui.exec("Git: Open Current Changes");
    tui.waitFor("while-closed");

    const screen = tui.snapshot();
    expect(screen).toContain("Current changes");
    expect(screen).toContain("M  tracked.txt · unstaged");
    expect(screen).toContain("while-closed");
  });
});
