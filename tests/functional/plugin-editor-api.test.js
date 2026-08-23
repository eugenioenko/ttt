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

describe("editor.get_line plugin API", () => {
  it("returns the text of a specific line (1-based)", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "alpha\nbeta\ngamma\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      local editor = require("ttt.editor")
      ttt.register({
        commands = {
          { id = "test.getline", title = "Test GetLine", handler = function()
              local line2 = editor.get_line(2)
              ttt.set_status_item("left", "result", "L2:" .. line2, { priority = 10 })
            end
          }
        }
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.exec("Test GetLine");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    const lastLine = snapshots[s].split("\n").pop();
    expect(lastLine).toContain("L2:beta");
  });
});

describe("editor.line_count plugin API", () => {
  it("returns the total number of lines", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "one\ntwo\nthree\nfour\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      local editor = require("ttt.editor")
      ttt.register({
        commands = {
          { id = "test.count", title = "Test Count", handler = function()
              local n = editor.line_count()
              ttt.set_status_item("left", "result", "LINES:" .. n, { priority = 10 })
            end
          }
        }
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.exec("Test Count");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    const lastLine = snapshots[s].split("\n").pop();
    expect(lastLine).toContain("LINES:");
  });
});

describe("editor.set_line plugin API", () => {
  it("replaces a line's content", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "aaa\nbbb\nccc\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      local editor = require("ttt.editor")
      ttt.register({
        commands = {
          { id = "test.setline", title = "Test SetLine", handler = function()
              editor.set_line(2, "REPLACED")
            end
          }
        }
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.exec("Test SetLine");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s]).toContain("REPLACED");
    expect(snapshots[s]).not.toContain("bbb");
  });
});

describe("editor.viewport plugin API", () => {
  it("returns top_line, bottom_line, and height", () => {
    dir = createTempDir();
    const lines = Array.from({ length: 50 }, (_, i) => `line ${i + 1}`).join("\n") + "\n";
    const file = createTempFile(dir, "test.txt", lines);
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      local editor = require("ttt.editor")
      ttt.register({
        commands = {
          { id = "test.vp", title = "Test Viewport", handler = function()
              local vp = editor.viewport()
              ttt.set_status_item("left", "result",
                "T:" .. vp.top_line .. ",H:" .. vp.height, { priority = 10 })
            end
          }
        }
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.exec("Test Viewport");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    const lastLine = snapshots[s].split("\n").pop();
    expect(lastLine).toContain("T:1,H:");
  });

  it("scroll_to changes the top line", () => {
    dir = createTempDir();
    const lines = Array.from({ length: 50 }, (_, i) => `line ${i + 1}`).join("\n") + "\n";
    const file = createTempFile(dir, "test.txt", lines);
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      local editor = require("ttt.editor")
      ttt.register({
        commands = {
          { id = "test.scroll", title = "Test Scroll", handler = function()
              editor.scroll_to(20)
              local vp = editor.viewport()
              ttt.set_status_item("left", "result",
                "T:" .. vp.top_line, { priority = 10 })
            end
          }
        }
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.exec("Test Scroll");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    const lastLine = snapshots[s].split("\n").pop();
    expect(lastLine).toContain("T:20");
  });
});

describe("editor.begin_undo_group / end_undo_group plugin API", () => {
  it("groups multiple edits into a single undo step", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "aaa\nbbb\nccc\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      local editor = require("ttt.editor")
      ttt.register({
        commands = {
          { id = "test.group", title = "Test Group", handler = function()
              editor.begin_undo_group()
              editor.set_line(1, "AAA")
              editor.set_line(2, "BBB")
              editor.set_line(3, "CCC")
              editor.end_undo_group()
            end
          }
        }
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.exec("Test Group");
    const s1 = tui.snapshot();
    // Single undo should revert all three changes
    tui.exec("Undo");
    const s2 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s1]).toContain("AAA");
    expect(snapshots[s1]).toContain("BBB");
    expect(snapshots[s1]).toContain("CCC");
    // After single undo, original content restored
    expect(snapshots[s2]).toContain("aaa");
    expect(snapshots[s2]).toContain("bbb");
    expect(snapshots[s2]).toContain("ccc");
  });
});

describe("editor.add_cursor / get_cursors / clear_cursors plugin API", () => {
  it("adds cursors and returns all cursor positions", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "aaa\nbbb\nccc\nddd\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      local editor = require("ttt.editor")
      ttt.register({
        commands = {
          { id = "test.addcursors", title = "Test AddCursors", handler = function()
              editor.add_cursor(2, 1)
              editor.add_cursor(3, 1)
              local cursors = editor.get_cursors()
              ttt.set_status_item("left", "result",
                "N:" .. #cursors, { priority = 10 })
            end
          }
        }
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.exec("Test AddCursors");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    const lastLine = snapshots[s].split("\n").pop();
    expect(lastLine).toContain("N:3");
  });

  it("clear_cursors collapses back to single cursor", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "aaa\nbbb\nccc\n");
    const plugin = writePlugin(dir, "test.lua", `
      local ttt = require("ttt")
      local editor = require("ttt.editor")
      ttt.register({
        commands = {
          { id = "test.clearcursors", title = "Test ClearCursors", handler = function()
              editor.add_cursor(2, 1)
              editor.add_cursor(3, 1)
              editor.clear_cursors()
              local cursors = editor.get_cursors()
              ttt.set_status_item("left", "result",
                "N:" .. #cursors, { priority = 10 })
            end
          }
        }
      })
    `);

    tui.start("--plugin", plugin, file);
    tui.exec("Test ClearCursors");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    const lastLine = snapshots[s].split("\n").pop();
    expect(lastLine).toContain("N:1");
  });
});
