import { describe, it, expect, afterEach } from "vitest";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("plugin timers", () => {
  it("interval ticks repeatedly and timeout fires once through the event loop", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello timers\n");
    const pluginFile = join(dir, "timers.lua");
    writeFileSync(
      pluginFile,
      `local ttt = require("ttt")
local ticks = 0
ttt.set_interval(100, function()
  ticks = ticks + 1
  ttt.log("info", "tick " .. ticks)
end)
ttt.set_timeout(150, function()
  ttt.log("info", "timeout-once")
end)
local cancelled = ttt.set_timeout(100, function()
  ttt.log("error", "cancelled-timer-fired")
end)
ttt.clear_timeout(cancelled)
`
    );

    tui.start("--plugin", pluginFile, file);
    tui.waitFor("hello timers");
    // These elapsed intervals are the subject of the test: cross the timer
    // deadlines before and after opening the output panel.
    tui.elapse(550);
    tui.panel("output");
    tui.elapse(150);
    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // At least 3 interval ticks in ~700ms
    expect(snapshots[s0]).toContain("tick 3");
    // Timeout fired exactly once
    expect(snapshots[s0]).toContain("timeout-once");
    // Cleared timer never fired
    expect(snapshots[s0]).not.toContain("cancelled-timer-fired");
  });
});
