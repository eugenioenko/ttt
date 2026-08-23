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
    tui.exec("Test Diff");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s]).toContain("myfile.go (diff)");
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
    tui.exec("Test ReadOnly");
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
    tui.exec("Test ReadOnly");
    tui.type("should not appear");
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
    tui.exec("Test OpenRO");
    tui.type("nope");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s]).toContain("readonly file content");
    expect(snapshots[s]).not.toContain("nope");
  });
});
