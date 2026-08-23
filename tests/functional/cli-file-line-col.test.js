import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createMultiLineFile, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("file:line:col command line arguments", () => {
  it("should open a file with the cursor on the given line", () => {
    dir = createTempDir();
    const file = createMultiLineFile(dir, "lines.txt", 80);

    tui.start(`${file}:42`);

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Ln 42");
    expect(snapshots[s0]).toContain("Line 42");
  });

  it("should honour a column as well as a line", () => {
    dir = createTempDir();
    const file = createMultiLineFile(dir, "lines.txt", 80);

    tui.start(`${file}:42:6`);

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Ln 42, Col 6");
  });

  // Output pasted from `grep -n` often carries a trailing colon.
  it("should tolerate a trailing colon", () => {
    dir = createTempDir();
    const file = createMultiLineFile(dir, "lines.txt", 80);

    tui.start(`${file}:42:`);

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Ln 42");
  });

  // A file that exists is opened as itself, so a name ending in a colon and
  // digits is never mistaken for a position.
  it("should prefer a real file whose name ends in :digits", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "report:12", "alpha\nbeta\n");

    tui.start(file);

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Ln 1");
    expect(snapshots[s0]).toContain("alpha");
  });

  // Only the file left active is framed; the others keep a placed cursor.
  // This asserts on the active file rather than switching tabs and reading the
  // status bar: in this harness the rendered status bar lagged a tab switch,
  // which reproduces with Ctrl+G plus View: Previous Tab and no CLI positions.
  it("should position the file left active when several are given", () => {
    dir = createTempDir();
    const first = createMultiLineFile(dir, "first.txt", 80);
    const second = createMultiLineFile(dir, "second.txt", 80);

    tui.start(`${first}:10`, `${second}:20`);

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Ln 20");
    expect(snapshots[s0]).toContain("Line 20");
  });

  it("should leave a plain path untouched", () => {
    dir = createTempDir();
    const file = createMultiLineFile(dir, "lines.txt", 80);

    tui.start(file);

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Ln 1, Col 1");
  });
});
