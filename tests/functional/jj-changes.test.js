import { describe, it, expect, afterEach, beforeAll } from "vitest";
import { execFileSync } from "node:child_process";
import { existsSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir, readFile } from "./helpers.js";

function hasJJ() {
  try {
    execFileSync("jj", ["--version"], { stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

const available = hasJJ();
if (!available && process.env.TTT_REQUIRE_JJ) {
  throw new Error("jj is required for jj-changes.test.js");
}

function jj(dir, ...args) {
  return execFileSync("jj", ["--no-pager", "--color=never", "--quiet", ...args], { cwd: dir, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
}

function jjLog(dir) {
  return jj(dir, "log", "--no-graph", "-r", "::@- ~ root()", "-T", 'description.first_line() ++ "\\n"')
    .split("\n")
    .filter((line) => line.length > 0);
}

// One committed file, then a modification and a new file in the working copy.
function createJJRepo(dir, ...initArgs) {
  jj(dir, "git", "init", ...initArgs);
  writeFileSync(join(dir, "tracked.txt"), "original\n", "utf8");
  jj(dir, "commit", "-m", "initial commit");
  writeFileSync(join(dir, "tracked.txt"), "modified\n", "utf8");
  writeFileSync(join(dir, "added.txt"), "new\n", "utf8");
  return dir;
}

// Two commits on one file, then a further edit in the working copy.
function createHistoryRepo(dir) {
  jj(dir, "git", "init");
  writeFileSync(join(dir, "tracked.txt"), "alpha\nbeta\ngamma\n", "utf8");
  jj(dir, "commit", "-m", "initial commit");
  writeFileSync(join(dir, "tracked.txt"), "alpha\nBETA\ngamma\n", "utf8");
  jj(dir, "commit", "-m", "uppercase beta");
  writeFileSync(join(dir, "tracked.txt"), "alpha\nBETA\nGAMMA\n", "utf8");
  return dir;
}

let dir;

beforeAll(() => {
  // An empty config keeps the developer's jj settings out of the tests and
  // proves no user.name or user.email is needed.
  const configDir = createTempDir();
  process.env.JJ_CONFIG = join(configDir, "config.toml");
  writeFileSync(process.env.JJ_CONFIG, "", "utf8");
});

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe.skipIf(!available)("jj changes panel", () => {
  it.each([["native"], ["colocated", "--colocate"]])("lists %s working-copy changes without staging", (_, ...initArgs) => {
    dir = createTempDir();
    createJJRepo(dir, ...initArgs);

    tui.start(dir);
    tui.exec("Show Changes");
    tui.waitFor("Changes (2)");
    tui.waitFor("initial commit");
    const panel = tui.snapshot();
    tui.press("a");
    const afterStageKey = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[panel]).toContain("tracked.txt");
    expect(snapshots[panel]).toContain("added.txt");
    expect(snapshots[panel]).not.toContain("Staged");
    expect(snapshots[afterStageKey]).not.toContain("Staged");
  });

  it("commits every working-copy change from the commit box", () => {
    dir = createTempDir();
    createJJRepo(dir);

    tui.start(dir);
    tui.exec("Show Changes");
    tui.waitFor("Changes (2)");
    tui.press("tab");
    tui.type("a jj commit from the editor");
    tui.press("enter");
    tui.waitFor("No changes");
    const after = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[after]).toContain("a jj commit from the editor");
    expect(jjLog(dir)).toEqual(["a jj commit from the editor", "initial commit"]);
    expect(jj(dir, "diff", "--summary")).toBe("");
  });

  it("discards working-copy changes with jj restore", () => {
    dir = createTempDir();
    createJJRepo(dir);

    tui.start(dir);
    tui.exec("Show Changes");
    tui.waitFor("Changes (2)");
    tui.type("D");
    tui.press("right");
    tui.press("enter");
    tui.waitFor("No changes");

    tui.run();
    expect(readFile(join(dir, "tracked.txt"))).toBe("original\n");
    expect(existsSync(join(dir, "added.txt"))).toBe(false);
  });

  it("opens a working-copy change as a diff against the parent", () => {
    dir = createTempDir();
    createHistoryRepo(dir);

    tui.start(dir);
    tui.exec("Show Changes");
    tui.waitFor("Changes (1)");
    tui.press("down");
    tui.exec("Git: Open Changes");
    tui.waitFor("tracked.txt (diff)");
    const opened = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[opened]).toMatch(/3\s+gamma\s+│\s+3\s+GAMMA/);
    expect(snapshots[opened]).toMatch(/2\s+BETA\s+│\s+2\s+BETA/);
  });

  it("opens a commit from history with its message and diff", () => {
    dir = createTempDir();
    createHistoryRepo(dir);

    tui.start(dir);
    tui.exec("Show Changes");
    tui.waitFor("uppercase beta");
    tui.press("tab");
    tui.press("tab");
    tui.press("down");
    tui.press("enter");
    tui.waitFor("1 file · +1 −1");
    const detail = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[detail]).toMatch(/Commit [k-z]{8}/);
    expect(snapshots[detail]).toContain("uppercase beta");
    expect(snapshots[detail]).toMatch(/Authored [A-Z][a-z]{2} \d{1,2}, \d{4}/);
    expect(snapshots[detail]).toMatch(/2\s+beta\s+│\s+2\s+BETA/);
  });

  it("keeps git-only commands out of a colocated workspace", () => {
    dir = createTempDir();
    createJJRepo(dir, "--colocate");
    const head = execFileSync("git", ["-C", dir, "rev-parse", "HEAD"], { encoding: "utf8" });

    tui.start(dir);
    tui.exec("Show Changes");
    tui.waitFor("Changes (2)");
    tui.exec("Git: Pull");
    tui.waitFor("Not available in jj repositories");
    tui.exec("Git: New Branch");
    tui.waitFor("No Git repositories");

    tui.run();
    expect(execFileSync("git", ["-C", dir, "rev-parse", "HEAD"], { encoding: "utf8" })).toBe(head);
  });
});
