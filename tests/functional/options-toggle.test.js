import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("options toggle", () => {
  it("should toggle line numbers on and off", () => {
    dir = createTempDir();
    const file = createTempFile(
      dir,
      "lines.txt",
      "first line\nsecond line\nthird line\nfourth line\nfifth line\n"
    );

    tui.start(file);
    tui.waitFor("first line");

    const s0 = tui.snapshot();

    tui.exec("Toggle Line Numbers");

    const s1 = tui.snapshot();

    // Toggle back to restore original state
    tui.exec("Toggle Line Numbers");

    const s2 = tui.snapshot();
    const { snapshots } = tui.run();

    // The toggle should change something
    expect(snapshots[s0]).not.toBe(snapshots[s1]);

    // One snapshot should have gutter line numbers (e.g., "  1  first"),
    // the other should not. The gutter pattern is digits followed by
    // two spaces then the line content.
    const gutterPattern = /\d\s{2}first line/;
    const hasGutter0 = gutterPattern.test(snapshots[s0]);
    const hasGutter1 = gutterPattern.test(snapshots[s1]);
    expect(hasGutter0).not.toBe(hasGutter1);

    // Toggling back should restore the original state
    expect(snapshots[s2]).toBe(snapshots[s0]);
  });

  it("should toggle syntax highlight without crashing", () => {
    dir = createTempDir();
    const file = createTempFile(
      dir,
      "test.go",
      'package main\n\nimport "fmt"\n\nfunc main() {\n\tfmt.Println("hello")\n}\n'
    );

    tui.start(file);
    tui.waitFor("package main");

    const s0 = tui.snapshot();

    tui.exec("Toggle Syntax Highlight");

    const s1 = tui.snapshot();

    // Toggle back to restore
    tui.exec("Toggle Syntax Highlight");

    const s2 = tui.snapshot();
    const { snapshots } = tui.run();

    // Both snapshots should still contain the file content
    expect(snapshots[s0]).toContain("package main");
    expect(snapshots[s1]).toContain("package main");
    expect(snapshots[s2]).toContain("package main");
  });

  it("should toggle bracket pair colorization without crashing", () => {
    dir = createTempDir();
    const file = createTempFile(
      dir,
      "brackets.go",
      "package main\n\nfunc main() {\n\tif (true) {\n\t\tx := ((1 + 2) * 3)\n\t}\n}\n"
    );

    tui.start(file);
    tui.waitFor("package main");

    const s0 = tui.snapshot();

    tui.exec("Toggle Bracket Pair Colorization");

    const s1 = tui.snapshot();

    // Toggle back to restore
    tui.exec("Toggle Bracket Pair Colorization");

    const s2 = tui.snapshot();
    const { snapshots } = tui.run();

    // Editor should remain functional and show content in all states
    expect(snapshots[s0]).toContain("package main");
    expect(snapshots[s1]).toContain("package main");
    expect(snapshots[s2]).toContain("package main");
  });
});
