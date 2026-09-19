import { describe, it, expect, afterEach } from "vitest";
import { execFileSync } from "node:child_process";
import { symlinkSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;
let emptyPath;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
  if (emptyPath) cleanupDir(emptyPath);
  dir = emptyPath = undefined;
});

const posix = process.platform === "win32" ? describe.skip : describe;

// Columns 31-92 below the input row hold the palette results and skip the
// sidebar, which lists every entry in the folder regardless of type.
function paletteRegion(snapshot) {
  return snapshot
    .split("\n")
    .slice(5, 14)
    .map((line) => line.slice(31, 92))
    .join("\n");
}

// With rg and git off PATH the palette falls back to walking the directory itself.
function startWithoutRgOrGit(target) {
  emptyPath = createTempDir();
  tui.start(target);
  tui.setEnv({ PATH: emptyPath });
}

posix("opening special files", () => {
  it("should not freeze on a named pipe given on the command line", () => {
    dir = createTempDir();
    const pipe = join(dir, "pipe");
    execFileSync("mkfifo", [pipe]);

    tui.start(pipe);

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("untitled");
  });

  it("should not freeze on a device given on the command line", () => {
    tui.start("/dev/null");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("untitled");
  });

  it("should list ordinary files in the fallback palette", () => {
    dir = createTempDir();
    createTempFile(dir, "notes.txt", "hello");

    startWithoutRgOrGit(dir);
    tui.waitFor("notes.txt");
    tui.exec("Go to File");
    tui.type("notes");
    tui.elapse(500);

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(paletteRegion(snapshots[s0])).toContain("notes.txt");
  });

  it("should not offer a named pipe in the fallback palette", () => {
    dir = createTempDir();
    createTempFile(dir, "notes.txt", "hello");
    execFileSync("mkfifo", [join(dir, "zzpipe")]);

    startWithoutRgOrGit(dir);
    tui.waitFor("notes.txt");
    tui.exec("Go to File");
    tui.type("zz");
    tui.elapse(500);

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(paletteRegion(snapshots[s0])).not.toContain("zzpipe");
  });

  it("should not offer a symlink to a named pipe in the fallback palette", () => {
    dir = createTempDir();
    createTempFile(dir, "notes.txt", "hello");
    execFileSync("mkfifo", [join(dir, "zzpipe")]);
    symlinkSync("zzpipe", join(dir, "zzlink"));

    startWithoutRgOrGit(dir);
    tui.waitFor("notes.txt");
    tui.exec("Go to File");
    tui.type("zzlink");
    tui.elapse(500);

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(paletteRegion(snapshots[s0])).not.toContain("zzlink");
  });

  it("should still offer a symlink to a regular file in the fallback palette", () => {
    dir = createTempDir();
    createTempFile(dir, "notes.txt", "hello");
    symlinkSync("notes.txt", join(dir, "zzlink"));

    startWithoutRgOrGit(dir);
    tui.waitFor("notes.txt");
    tui.exec("Go to File");
    tui.type("zzlink");
    tui.elapse(500);

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(paletteRegion(snapshots[s0])).toContain("zzlink");
  });
});
