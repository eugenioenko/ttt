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

function installPlugin(configDir, name, fields) {
  const pluginDir = join(configDir, "plugins", name);
  mkdirSync(pluginDir, { recursive: true });
  writeFileSync(join(pluginDir, "plugin.ttt.json"), JSON.stringify({
    name,
    version: "1.0.0",
    entry: "main.ttt.lua",
    permissions: { "panel.sidebar": true },
  }));
  writeFileSync(join(pluginDir, "main.ttt.lua"),
    `require("ttt").register({ sidebar = { ${fields}, render = function() end } })`);
  return { name, version: "1.0.0", enabled: true, permissions: { "panel.sidebar": true } };
}

function tabRow(screen) {
  return screen.split("\n").find((line) => line.includes("⌂"));
}

it("shows an installed plugin's icon from startup and follows the icon mode", () => {
  dir = createTempDir();
  const configDir = join(dir, "config");
  const ws = join(dir, "ws");
  mkdirSync(ws, { recursive: true });
  mkdirSync(configDir, { recursive: true });
  const registry = [
    installPlugin(configDir, "both", `title = "Todo", icon = { plain = "T", nerd = "\\238\\171\\176" }`),
    installPlugin(configDir, "nerdonly", `title = "Nerdy", icon = { nerd = "\\238\\171\\176" }`),
    installPlugin(configDir, "none", `title = "Notes"`),
  ];
  writeFileSync(join(configDir, "plugins.ttt.json"), JSON.stringify(registry));
  writeFileSync(join(configDir, "settings.json"), JSON.stringify({
    sidebar: { tabStyle: "icon", width: 70 },
    appearance: { icons: "none" },
  }));

  tui.start(ws);
  tui.setEnv({ TTT_CONFIG_DIR: configDir });
  const plain = tui.snapshot();
  tui.exec("Toggle Font Icons");
  const nerd = tui.snapshot();
  const { snapshots } = tui.run();

  const plainTabs = tabRow(snapshots[plain]);
  expect(plainTabs).toContain(" T ");
  expect(plainTabs).not.toContain("Todo");
  expect(plainTabs).toContain("Nerdy");
  expect(plainTabs).toContain("Notes");

  const nerdTabs = snapshots[nerd].split("\n").find((line) => line.includes("Notes"));
  expect(nerdTabs).toContain("");
  expect(nerdTabs).not.toContain("Nerdy");
  expect(nerdTabs).not.toContain(" T ");
});
