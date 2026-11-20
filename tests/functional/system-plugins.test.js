import { it, expect, afterEach } from "vitest";
import { mkdirSync, writeFileSync, readFileSync, readdirSync } from "node:fs";
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

function installCommand(root, name, notification) {
  const plugin = join(root, name);
  mkdirSync(plugin, { recursive: true });
  const permissions = { commands: true };
  writeFileSync(join(plugin, "plugin.ttt.json"), JSON.stringify({
    name, version: "1.0.0", entry: "main.ttt.lua", permissions,
  }));
  writeFileSync(join(plugin, "main.ttt.lua"), `
    local ttt = require("ttt")
    ttt.register({commands = {{id = "${name}.check", title = "${name}: Check",
      handler = function() ttt.notify("${notification}") end}}})
  `);
  return { name, version: "1.0.0", enabled: true, grantsVersion: 2, permissions };
}

function filesUnder(root) {
  return Object.fromEntries(readdirSync(root, { recursive: true, withFileTypes: true })
    .filter((entry) => entry.isFile())
    .map((entry) => {
      const path = join(entry.parentPath, entry.name);
      return [path, readFileSync(path, "utf8")];
    }));
}

function startUser(config) {
  tui.start(join(dir, "workspace"));
  tui.setEnv({ TTT_CONFIG_DIR: config });
}

function pluginMenu(action) {
  tui.exec("Plugins: Show Panel");
  tui.press("shift+tab");
  tui.press("shift+enter");
  tui.waitFor(action);
  return tui.snapshot();
}

function expectNoApproval(snapshots, name) {
  for (const screen of snapshots) {
    expect(screen).not.toContain(`${name} requests`);
  }
}

it("persists a system plugin's UI disable for one user without changing shared files", () => {
  const system = setup();
  const userA = join(dir, "config");
  const userB = join(dir, "other-config");
  mkdirSync(userB);
  writeFileSync(join(userB, "settings.json"), readFileSync(join(userA, "settings.json")));
  const entry = installCommand(system, "shared", "Shared fixture command ran");
  writeFileSync(join(system, "plugins.ttt.json"), JSON.stringify([entry]));
  const sharedFiles = filesUnder(system);

  tui.exec("shared: Check");
  const before = tui.snapshot();
  pluginMenu("Disable");
  tui.press("enter");
  const disabled = tui.snapshot();
  let result = tui.run();
  expect(result.snapshots[before]).toContain("Shared fixture command ran");
  expect(result.snapshots[disabled]).toMatch(/○\s+shared/);
  expectNoApproval(result.snapshots, "shared");
  expect(filesUnder(system)).toEqual(sharedFiles);

  startUser(userA);
  const enableMenu = pluginMenu("Enable");
  tui.press("escape");
  const restarted = tui.snapshot();
  tui.exec("Command Palette");
  tui.type("shared: Check");
  const unavailable = tui.snapshot();
  result = tui.run();
  expect(result.snapshots[enableMenu]).toContain("Enable");
  expect(result.snapshots[restarted]).toMatch(/○\s+shared/);
  // Only the palette query remains; there is no command result.
  expect(result.snapshots[unavailable].match(/shared: Check/g)).toHaveLength(1);
  expectNoApproval(result.snapshots, "shared");
  expect(filesUnder(system)).toEqual(sharedFiles);

  startUser(userB);
  tui.exec("shared: Check");
  const otherUser = tui.snapshot();
  result = tui.run();
  expect(result.snapshots[otherUser]).toContain("Shared fixture command ran");
  expectNoApproval(result.snapshots, "shared");
  expect(filesUnder(system)).toEqual(sharedFiles);

  startUser(userA);
  pluginMenu("Enable");
  tui.press("enter");
  const enabled = tui.snapshot();
  tui.exec("shared: Check");
  const ran = tui.snapshot();
  result = tui.run();
  expect(result.snapshots[enabled]).toMatch(/●\s+shared/);
  expect(result.snapshots[ran]).toContain("Shared fixture command ran");
  expectNoApproval(result.snapshots, "shared");
  expect(filesUnder(system)).toEqual(sharedFiles);
});

it("disables and re-enables a user plugin through the Plugins UI", () => {
  const system = setup();
  mkdirSync(system);
  const config = join(dir, "config");
  const plugins = join(config, "plugins");
  const entry = installCommand(plugins, "personal", "Personal fixture command ran");
  writeFileSync(join(config, "plugins.ttt.json"), JSON.stringify([entry]));
  const pluginFiles = filesUnder(plugins);

  tui.exec("personal: Check");
  const before = tui.snapshot();
  pluginMenu("Disable");
  tui.press("enter");
  const disabled = tui.snapshot();
  let result = tui.run();
  expect(result.snapshots[before]).toContain("Personal fixture command ran");
  expect(result.snapshots[disabled]).toMatch(/○\s+personal/);
  expectNoApproval(result.snapshots, "personal");
  expect(filesUnder(plugins)).toEqual(pluginFiles);

  startUser(config);
  pluginMenu("Enable");
  tui.press("enter");
  const enabled = tui.snapshot();
  tui.exec("personal: Check");
  const ran = tui.snapshot();
  result = tui.run();
  expect(result.snapshots[enabled]).toMatch(/●\s+personal/);
  expect(result.snapshots[ran]).toContain("Personal fixture command ran");
  expectNoApproval(result.snapshots, "personal");
  expect(filesUnder(plugins)).toEqual(pluginFiles);
});
