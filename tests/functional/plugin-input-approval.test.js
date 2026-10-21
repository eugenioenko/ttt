import { describe, it, expect, afterEach } from "vitest";
import { mkdirSync, writeFileSync, readFileSync, existsSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

function clickerSource(screenPath) {
  return `local ttt = require("ttt")
local fs = require("ttt.fs")
local function find_allow()
  ttt.screenshot(${JSON.stringify(screenPath)})
  local screen = fs.read(${JSON.stringify(screenPath)})
  local at = screen:find("Allow", 1, true)
  local y, line_start = 0, 1
  while true do
    local nl = screen:find("\\n", line_start, true)
    if not nl or nl > at then break end
    y = y + 1
    line_start = nl + 1
  end
  local prefix = screen:sub(line_start, at - 1)
  local _, continuation = prefix:gsub("[\\128-\\191]", "")
  return #prefix - continuation, y
end
ttt.register({
  commands = {
    { id = "clicker.allow", title = "Click Allow", handler = function()
      local x, y = find_allow()
      ttt.click(x, y)
    end },
  },
})
`;
}

function setup() {
  dir = createTempDir();
  const ws = join(dir, "ws");
  const configDir = join(dir, "config");
  const victimDir = join(ws, "plugins", "victim");
  mkdirSync(victimDir, { recursive: true });
  mkdirSync(configDir, { recursive: true });
  writeFileSync(
    join(victimDir, "plugin.ttt.json"),
    JSON.stringify({
      name: "victim",
      version: "0.1.0",
      entry: "init.lua",
      permissions: { "system.env": true },
    }),
  );
  writeFileSync(join(victimDir, "init.lua"), 'local ttt = require("ttt")\n');
  const clickerPath = join(dir, "clicker.lua");
  writeFileSync(clickerPath, clickerSource(join(dir, "screen.txt")));
  return { ws, configDir, clickerPath };
}

describe("plugin input during plugin approval", () => {
  it("cannot approve another plugin's permissions", () => {
    const { ws, configDir, clickerPath } = setup();

    tui.start(ws, "--plugin", clickerPath);
    tui.setEnv({ TTT_CONFIG_DIR: configDir });
    tui.waitFor("victim requests");
    tui.exec("Click Allow");
    tui.elapse(300);
    const afterPluginClick = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[afterPluginClick]).toContain("victim requests");
    const registry = join(configDir, "plugins.ttt.json");
    const granted = existsSync(registry) ? readFileSync(registry, "utf8") : "";
    expect(granted).not.toContain("victim");
  });
});
