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

// A plugin that opens the command line on ":" — the Vim-mode shape this API exists for.
const CMDLINE_PLUGIN = `
  local ttt = require("ttt")
  local events = require("ttt.events")
  ttt.register({})
  events.on("key.press", function(ev)
    if ev.key == ":" then
      ttt.command_line.show({
        prefix = ":",
        on_change = function(text)
          ttt.set_status_item("left", "chg", "CHG[" .. text .. "]", { priority = 10 })
        end,
        on_submit = function(text)
          ttt.set_status_item("left", "res", "SUBMIT[" .. text .. "]", { priority = 11 })
        end,
        on_cancel = function()
          ttt.set_status_item("left", "res", "CANCEL", { priority = 11 })
        end,
      })
      return true
    end
    return false
  end)
`;

describe("plugin command line", () => {
  it("opens a framed command line and submits its text", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello");
    const plugin = writePlugin(dir, "cmdline.lua", CMDLINE_PLUGIN);

    tui.start("--plugin", plugin, file);
    tui.type(":");
    tui.type("wq");
    const open = tui.snapshot();
    tui.press("enter");
    const after = tui.snapshot();
    const { snapshots } = tui.run();

    // While open: the prefix + text show inside the box, and OnChange has fired.
    expect(snapshots[open]).toContain(":wq");
    expect(snapshots[open]).toContain("CHG[wq]");
    // The buffer is untouched — the command line overlays, it does not resize.
    expect(snapshots[open]).toContain("hello");

    // After Enter: submitted with the right text and the box is gone.
    expect(snapshots[after]).toContain("SUBMIT[wq]");
    expect(snapshots[after]).not.toContain(":wq");
  });

  it("cancels on escape and returns focus to the editor", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello");
    const plugin = writePlugin(dir, "cmdline.lua", CMDLINE_PLUGIN);

    tui.start("--plugin", plugin, file);
    tui.type(":");
    tui.type("q");
    tui.press("escape");
    tui.press("end");
    tui.type("Z");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s]).toContain("CANCEL");
    expect(snapshots[s]).not.toContain("SUBMIT");
    // Focus went back to the editor, so the keystroke landed in the buffer.
    expect(snapshots[s]).toContain("helloZ");
  });

  it("silences the plugin key interceptor while open", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello");
    const plugin = writePlugin(
      dir,
      "counting.lua",
      `
      local ttt = require("ttt")
      local events = require("ttt.events")
      ttt.register({})
      local seen = 0
      events.on("key.press", function(ev)
        if ev.key == ":" then
          ttt.command_line.show({ prefix = ":" })
          return true
        end
        seen = seen + 1
        ttt.set_status_item("left", "seen", "SEEN=" .. seen, { priority = 10 })
        return true
      end)
    `,
    );

    tui.start("--plugin", plugin, file);
    tui.type("a");
    tui.type(":");
    tui.type("bcd");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    // "a" reached the interceptor; "bcd" went to the command line instead.
    expect(snapshots[s]).toContain("SEEN=1");
    expect(snapshots[s]).toContain(":bcd");
  });
});
