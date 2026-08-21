import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";
import { writeFileSync } from "node:fs";
import { join } from "node:path";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

function writePlugin(dir, name, lua) {
  const path = join(dir, name);
  writeFileSync(path, lua, "utf8");
  return path;
}

describe("ttt.open_diff plugin API", () => {
  it("opens a diff tab with old and new lines", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      ttt.register({
        commands = {
          { id = "test.diff", title = "Test Diff", handler = function()
              ttt.open_diff("myfile.go", {"line one", "line two"}, {"line one", "line TWO", "line three"}, "myfile.go")
            end
          }
        },
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.waitStable(300);
    tui.exec("Test Diff");
    tui.waitStable(300);
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s]).toContain("myfile.go (diff)");
  });

  it("wraps long diff lines through the palette command", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      ttt.register({
        commands = {
          { id = "test.diff", title = "Test Diff", handler = function()
              ttt.open_diff("wrapped.go", {"left-prefix-LEFT-SUFFIX"}, {"right-prefix-RIGHT-SUFFIX"}, "wrapped.go")
            end
          }
        },
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.setSize(42, 12);
    tui.waitStable(300);
    tui.exec("Test Diff");
    const before = tui.snapshot();
    tui.exec("Git: Toggle Diff Wrap");
    const after = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[before]).not.toContain("FFIX");
    expect(snapshots[after]).toContain("FFIX");
  });

  it("turns diff wrapping off with a second toggle", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      ttt.register({
        commands = {
          { id = "test.diff", title = "Test Diff", handler = function()
              ttt.open_diff("wrapped.go", {"left-prefix-LEFT-SUFFIX"}, {"right-prefix-RIGHT-SUFFIX"}, "wrapped.go")
            end
          }
        },
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.setSize(42, 12);
    tui.waitStable(300);
    tui.exec("Test Diff");
    tui.exec("Git: Toggle Diff Wrap");
    const wrapped = tui.snapshot();
    tui.exec("Git: Toggle Diff Wrap");
    const afterSecondToggle = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[wrapped]).toContain("FFIX");
    expect(snapshots[afterSecondToggle]).not.toContain("FFIX");
  });

  it("stacks removed lines before added lines in unified mode", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      ttt.register({
        commands = {
          { id = "test.diff", title = "Test Diff", handler = function()
              ttt.open_diff("unified.go", {"old one", "old two"}, {"new one", "new two"}, "unified.go")
            end
          }
        },
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.setSize(80, 14);
    tui.waitStable(300);
    tui.exec("Test Diff");
    tui.exec("Git: Toggle Unified Diff");
    const unified = tui.snapshot();
    const { snapshots } = tui.run();

    const screen = snapshots[unified];
    expect(screen.indexOf("old one")).toBeLessThan(screen.indexOf("old two"));
    expect(screen.indexOf("old two")).toBeLessThan(screen.indexOf("new one"));
    expect(screen.indexOf("new one")).toBeLessThan(screen.indexOf("new two"));
  });

  it("restores the side-by-side diff after a second unified toggle", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      ttt.register({
        commands = {
          { id = "test.diff", title = "Test Diff", handler = function()
              ttt.open_diff("unified.go", {"old one"}, {"new one"}, "unified.go")
            end
          }
        },
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.setSize(80, 14);
    tui.waitStable(300);
    tui.exec("Test Diff");
    tui.exec("Git: Toggle Unified Diff");
    const unified = tui.snapshot();
    tui.exec("Git: Toggle Unified Diff");
    const split = tui.snapshot();
    const { snapshots } = tui.run();

    const unifiedLines = snapshots[unified].split("\n");
    const oldRow = unifiedLines.findIndex((line) => line.includes("old one"));
    const newRow = unifiedLines.findIndex((line) => line.includes("new one"));
    expect(oldRow).toBeGreaterThanOrEqual(0);
    expect(newRow).toBeGreaterThan(oldRow);
    expect(unifiedLines[oldRow]).not.toContain("new one");
    expect(snapshots[split]).toMatch(/old one.*│.*new one/);
  });

  it("switches modes from the visible tab control and shows the current value", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      ttt.register({
        commands = {
          { id = "test.diff", title = "Test Diff", handler = function()
              ttt.open_diff("control.go", {"old one", "old two"}, {"new one", "new two"}, "control.go")
            end
          }
        },
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.setSize(80, 14);
    tui.waitStable(300);
    tui.exec("Test Diff");
    const split = tui.snapshot();
    // The segmented control is right-aligned in the editor tab bar. At this
    // deterministic width, Unified occupies columns 65..75 beside More.
    tui.click(70, 2);
    const unified = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[split]).toContain("● Split");
    expect(snapshots[split]).toContain("○ Unified");
    expect(snapshots[unified]).toContain("○ Split");
    expect(snapshots[unified]).toContain("● Unified");
    const rows = snapshots[unified].split("\n");
    expect(rows.findIndex((row) => row.includes("old two"))).toBeLessThan(
      rows.findIndex((row) => row.includes("new one")),
    );
  });

  it("shows the distance across a large collapsed region", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      ttt.register({
        commands = {
          { id = "test.diff", title = "Test Diff", handler = function()
              ttt.open_diff("distance.go", {}, {}, "distance.go", false, [[--- a/distance.go
+++ b/distance.go
@@ -22,3 +22,3 @@
 line 22
 line 23
 line 24
@@ -356,2 +356,2 @@
 line 356
 line 357
]])
            end
          }
        },
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.setSize(100, 14);
    tui.waitStable(300);
    tui.exec("Test Diff");
    const compact = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[compact]).toContain("⋯ 331 lines ⋯");
    expect(snapshots[compact]).not.toContain("@@ -356,2 +356,2 @@");
  });

  it("uses singular wording for exactly one hidden line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      ttt.register({
        commands = {
          { id = "test.diff", title = "Test Diff", handler = function()
              ttt.open_diff("distance.go", {}, {}, "distance.go", false, [[--- a/distance.go
+++ b/distance.go
@@ -24,1 +24,1 @@
 line 24
@@ -26,1 +26,1 @@
 line 26
]])
            end
          }
        },
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.setSize(100, 14);
    tui.waitStable(300);
    tui.exec("Test Diff");
    const compact = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[compact]).toContain("⋯ 1 line ⋯");
    expect(snapshots[compact]).not.toContain("⋯ 1 lines ⋯");
  });

  it("omits the separator when adjacent lines hide nothing", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      ttt.register({
        commands = {
          { id = "test.diff", title = "Test Diff", handler = function()
              ttt.open_diff("distance.go", {}, {}, "distance.go", false, [[--- a/distance.go
+++ b/distance.go
@@ -24,1 +24,1 @@
 line 24
@@ -25,1 +25,1 @@
 line 25
]])
            end
          }
        },
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.setSize(100, 14);
    tui.waitStable(300);
    tui.exec("Test Diff");
    const compact = tui.snapshot();
    const { snapshots } = tui.run();

    const rows = snapshots[compact].split("\n");
    const line24Row = rows.findIndex((row) => row.includes("line 24"));
    const line25Row = rows.findIndex((row) => row.includes("line 25"));
    expect(line24Row).toBeGreaterThanOrEqual(0);
    expect(line25Row).toBe(line24Row + 1);
    expect(snapshots[compact]).not.toContain("⋯");
  });
});

describe("ttt.open_readonly plugin API", () => {
  it("opens a readonly buffer tab", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      ttt.register({
        commands = {
          { id = "test.readonly", title = "Test ReadOnly", handler = function()
              ttt.open_readonly("file (abc123)", {"readonly content", "second line"}, "file.go")
            end
          }
        },
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.waitStable(300);
    tui.exec("Test ReadOnly");
    tui.waitStable(300);
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s]).toContain("file (abc123)");
    expect(snapshots[s]).toContain("readonly content");
  });

  it("blocks typing in readonly tab", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      ttt.register({
        commands = {
          { id = "test.readonly", title = "Test ReadOnly", handler = function()
              ttt.open_readonly("file (v1)", {"original line"}, "file.go")
            end
          }
        },
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.waitStable(300);
    tui.exec("Test ReadOnly");
    tui.waitStable(300);
    tui.type("should not appear");
    tui.waitStable();
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s]).toContain("original line");
    expect(snapshots[s]).not.toContain("should not appear");
  });
});

describe("ttt.open_file readonly mode", () => {
  it("opens a file in readonly mode with third argument", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello\n");
    const target = createTempFile(dir, "target.txt", "readonly file content\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      ttt.register({
        commands = {
          { id = "test.openro", title = "Test OpenRO", handler = function()
              ttt.open_file("${target.replace(/\\/g, "\\\\")}", 0, true)
            end
          }
        }
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.waitStable(300);
    tui.exec("Test OpenRO");
    tui.waitStable(300);
    tui.type("nope");
    tui.waitStable();
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s]).toContain("readonly file content");
    expect(snapshots[s]).not.toContain("nope");
  });
});
