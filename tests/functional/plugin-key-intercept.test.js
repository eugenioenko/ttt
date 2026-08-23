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

describe("key.press event", () => {
  it("intercepts and suppresses a key when returning true", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello");
    const plugin = writePlugin(dir, "intercept.lua", `
      local ttt = require("ttt")
      local events = require("ttt.events")
      ttt.register({})
      events.on("key.press", function(ev)
        if ev.key == "j" then
          ttt.set_status_item("left", "mode", "INTERCEPTED", { priority = 10 })
          return true
        end
        return false
      end)
    `);

    tui.start("--plugin", plugin, file);
    tui.press("end");
    tui.type("j");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s]).not.toContain("helloj");
    expect(snapshots[s]).toContain("INTERCEPTED");
  });

  it("passes key through when returning false", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello");
    const plugin = writePlugin(dir, "passthrough.lua", `
      local ttt = require("ttt")
      local events = require("ttt.events")
      ttt.register({})
      events.on("key.press", function(ev)
        return false
      end)
    `);

    tui.start("--plugin", plugin, file);
    tui.press("end");
    tui.type("x");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s]).toContain("hellox");
  });
});
