import { it, expect, afterEach } from "vitest";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir } from "./helpers.js";

let dir;
afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

function setup() {
  dir = createTempDir();
  const config = join(dir, "config");
  const workspace = join(dir, "workspace");
  const system = join(dir, "system");
  mkdirSync(config);
  mkdirSync(workspace);
  writeFileSync(join(config, "settings.json"), JSON.stringify({
    plugins: { systemDir: system }, lsp: { enabled: false },
  }));
  tui.start(workspace);
  tui.setEnv({ TTT_CONFIG_DIR: config });
  return system;
}

it("runs an admin-approved system command without user approval", () => {
  const system = setup();
  const plugin = join(system, "shared");
  mkdirSync(plugin, { recursive: true });
  const permissions = { commands: true };
  writeFileSync(join(plugin, "plugin.ttt.json"), JSON.stringify({
    name: "shared", version: "1.0.0", entry: "main.ttt.lua", permissions,
  }));
  writeFileSync(join(plugin, "main.ttt.lua"), `
    local ttt = require("ttt")
    ttt.register({commands = {{id = "shared.check", title = "Shared: Check",
      handler = function() ttt.notify("Admin-approved command ran") end}}})
  `);
  writeFileSync(join(system, "plugins.ttt.json"), JSON.stringify([
    { name: "shared", version: "1.0.0", enabled: true, grantsVersion: 2, permissions },
  ]));
  tui.exec("Shared: Check");
  const screen = tui.snapshot();
  const { snapshots } = tui.run();
  expect(snapshots[screen]).toContain("Admin-approved command ran");
});

it("reports a missing system directory in status and Output", () => {
  setup();
  const status = tui.snapshot();
  tui.exec("Output: Show Panel");
  const output = tui.snapshot();
  const { snapshots } = tui.run();
  expect(snapshots[status].split("\n").slice(-3).join("\n")).toContain("Could not load plugins");
  expect(snapshots[output].split("\n").slice(0, -3).join("\n")).toContain("Could not load plugins");
});
