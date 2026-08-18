import { mkdtempSync, writeFileSync, readFileSync, rmSync, existsSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { join } from "node:path";
import { tmpdir } from "node:os";

// git identity and signing are set on the repo itself: CI runners have no
// global config, and a developer's global commit.gpgsign would hang the commit
// waiting for a passphrase.
const GIT_CONFIG = [
  ["user.email", "test@test.com"],
  ["user.name", "Test User"],
  ["commit.gpgsign", "false"],
  ["tag.gpgsign", "false"],
];

export function git(dir, ...args) {
  return execFileSync("git", ["-C", dir, ...args], { encoding: "utf8" });
}

// createGitRepo initialises a repo with one commit and returns its path.
export function createGitRepo(dir) {
  git(dir, "init", "-q", "-b", "main");
  for (const [key, value] of GIT_CONFIG) {
    git(dir, "config", key, value);
  }
  writeFileSync(join(dir, "tracked.txt"), "original\n", "utf8");
  git(dir, "add", "tracked.txt");
  git(dir, "commit", "-qm", "initial commit");
  return dir;
}

// gitStatus returns porcelain status lines, e.g. ["M  tracked.txt"].
export function gitStatus(dir) {
  return git(dir, "status", "--porcelain")
    .split("\n")
    .filter((line) => line.length > 0);
}

export function gitLog(dir) {
  return git(dir, "log", "--format=%s")
    .split("\n")
    .filter((line) => line.length > 0);
}

export function createTempDir() {
  return mkdtempSync(join(tmpdir(), "ttt-test-"));
}

export function createTempFile(dir, name, content) {
  const filePath = join(dir, name);
  writeFileSync(filePath, content, "utf8");
  return filePath;
}

export function createMultiLineFile(dir, name, lines) {
  const content = Array.from({ length: lines }, (_, i) => `Line ${i + 1}`).join("\n");
  return createTempFile(dir, name, content);
}

export function readFile(path) {
  return readFileSync(path, "utf8");
}

export function fileExists(path) {
  return existsSync(path);
}

export function cleanupDir(dir) {
  rmSync(dir, { recursive: true, force: true });
}
