import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("markdown preview", () => {
  it("should open preview for a .md file", () => {
    dir = createTempDir();
    const file = createTempFile(
      dir,
      "readme.md",
      "# Hello World\n\nSome text here.\n\n- item one\n- item two\n"
    );

    tui.start(file);
    tui.waitFor("Hello World");

    tui.exec("Open Preview");
    tui.waitStable();

    const snap = tui.snapshot();
    expect(snap).toContain("Preview: readme.md");
    expect(snap).toContain("Hello World");
  });

  it("should render list bullets in preview", () => {
    dir = createTempDir();
    const file = createTempFile(
      dir,
      "list.md",
      "- alpha\n- beta\n- gamma\n"
    );

    tui.start(file);
    tui.waitFor("alpha");

    tui.exec("Open Preview");
    tui.waitStable();

    const snap = tui.snapshot();
    expect(snap).toContain("alpha");
    expect(snap).toContain("beta");
    expect(snap).toContain("gamma");
  });

  it("should not open preview for non-md files", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "code.go", "package main\n");

    tui.start(file);
    tui.waitFor("package");

    tui.exec("Open Preview");
    tui.waitStable();

    const snap = tui.snapshot();
    expect(snap).not.toContain("Preview:");
  });
});
