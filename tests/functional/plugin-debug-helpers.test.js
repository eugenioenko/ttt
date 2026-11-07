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

const probe = `local ttt = require("ttt")
local json = require("ttt.json")
ttt.register({
  commands = {
    { id = "probe.capture", title = "Probe Capture", handler = function()
      local screen = ttt.screenshot()
      local state = json.decode(ttt.debug())
      ttt.log("info", "capture screen=" .. tostring(screen:find("probe.txt", 1, true) ~= nil) .. " buffer=" .. tostring(state.buffer ~= nil))
    end },
  },
})
`;

describe("plugin debug helpers", () => {
  it("return the screen and state to a plugin loaded with --plugin", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "probe.txt", "hello\n");
    const pluginPath = join(dir, "probe.lua");
    writeFileSync(pluginPath, probe);

    tui.start(file, "--plugin", pluginPath);
    tui.exec("Probe Capture");
    tui.panel("output");
    tui.waitFor("capture screen=");
    const output = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[output]).toContain("capture screen=true buffer=true");
  });
});
