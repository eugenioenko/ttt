import { describe, it, expect, afterEach } from "vitest";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createGitRepo, createTempDir, createTempFile, cleanupDir, git } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("status bar segments", () => {
  it("shows branch, position, indent, encoding, eol, and language", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "hello.go", 'package main\n\nfunc main() {\n}\n');

    tui.start(file);
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    const lastLine = snapshots[s].split("\n").pop();
    expect(lastLine).toContain("Ln 1, Col 1");
    expect(lastLine).toContain("UTF-8");
    expect(lastLine).toContain("LF");
    expect(lastLine).toContain("Go");
  });

  it("updates position when cursor moves", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "nav.txt", "hello world\nline two\nline three\n");

    tui.start(file);
    tui.press("arrow_down");
    tui.press("arrow_down");
    tui.press("arrow_right");
    tui.press("arrow_right");
    tui.press("arrow_right");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    const lastLine = snapshots[s].split("\n").pop();
    expect(lastLine).toContain("Ln 3, Col 4");
  });

  it("shows indent style for spaces", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "spaces.js", "const x = 1;\n");

    tui.start(file);
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    const lastLine = snapshots[s].split("\n").pop();
    expect(lastLine).toContain("Spaces:");
  });

  it("shows a notification from a delayed debug action", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "note.txt", "hello\n");

    tui.start(file);
    tui.waitFor("note.txt");
    tui.exec("Debug: Screenshot");
    tui.waitFor("Screenshot saved: screenshot.txt");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    const lastLine = snapshots[s].split("\n").pop();
    expect(lastLine).toContain("Screenshot");
  });

  it("blames the line under the cursor after unsaved edits shift it", () => {
    dir = createTempDir();
    createGitRepo(dir);
    const file = join(dir, "authors.txt");
    writeFileSync(file, "first line\n");
    git(dir, "add", "authors.txt");
    git(dir, "-c", "user.name=Alice", "commit", "-qm", "first");
    writeFileSync(file, "first line\nsecond line\n");
    git(dir, "-c", "user.name=Bob", "commit", "-qam", "second");

    tui.start(file);
    tui.waitFor("Alice");
    tui.press("enter");
    tui.waitFor("Alice");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    const lastLine = snapshots[s].split("\n").pop();
    expect(lastLine).toContain("Ln 2, Col 1");
    expect(lastLine).toContain("Alice");
    expect(lastLine).not.toContain("Bob");
  });

  it("clears blame on a same-line edit and restores it after undo", () => {
    dir = createTempDir();
    createGitRepo(dir);
    const file = join(dir, "authors.txt");
    writeFileSync(file, "committed line\n");
    git(dir, "add", "authors.txt");
    git(dir, "-c", "user.name=Alice", "commit", "-qm", "committed");

    tui.start(file);
    tui.waitFor("Alice");
    tui.type("X");
    const edited = tui.snapshot();
    tui.exec("Undo");
    tui.waitFor("Alice");
    const restored = tui.snapshot();
    const { snapshots } = tui.run();

    const editedStatus = snapshots[edited].split("\n").pop();
    expect(editedStatus).toContain("Ln 1, Col 2");
    expect(editedStatus).not.toContain("Alice");
    expect(snapshots[restored].split("\n").pop()).toContain("Alice");
  });
});
