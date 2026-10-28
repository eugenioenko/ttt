import { it, expect, afterEach } from "vitest";
import { mkdirSync, writeFileSync, readFileSync, existsSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

it("approves multiple themes without Lua, retains them when disabled, and removes them on uninstall", () => {
  dir = createTempDir();
  const ws = join(dir, "ws");
  const pluginDir = join(ws, "plugins", "theme-pack");
  mkdirSync(pluginDir, { recursive: true });
  writeFileSync(join(pluginDir, "plugin.ttt.json"), JSON.stringify({
    name: "theme-pack",
    themes: [{ path: "dark.json" }, { path: "light.json" }],
  }));
  for (const name of ["dark", "light"]) {
    writeFileSync(join(pluginDir, `${name}.json`), JSON.stringify({
      default: { fg: "#ffffff", bg: name === "dark" ? "#121212" : "#eeeeee" },
    }));
  }
  const configDir = join(dir, "config");
  const installed = join(configDir, "themes", "plugins", "theme-pack");
  tui.start(ws);
  tui.setEnv({ TTT_CONFIG_DIR: configDir });
  tui.waitFor("theme-pack requests");
  const approval = tui.snapshot();
  tui.press("a");
  tui.exec("Switch Theme");
  tui.type("plugins/theme-pack/light");
  const picker = tui.snapshot();
  tui.press("enter");
  const { snapshots } = tui.run();
  expect(snapshots[approval]).toContain("dark");
  expect(snapshots[approval]).toContain("light");
  expect(snapshots[picker]).toContain("plugins/theme-pack/light");
  expect(existsSync(join(installed, "dark.json"))).toBe(true);
  expect(existsSync(join(installed, "light.json"))).toBe(true);
  const settingsPath = join(configDir, "settings.json");
  expect(JSON.parse(readFileSync(settingsPath, "utf8")).theme).toBe("plugins/theme-pack/light");

  const registryPath = join(configDir, "plugins.ttt.json");
  const registry = JSON.parse(readFileSync(registryPath, "utf8"));
  registry[0].enabled = false;
  writeFileSync(registryPath, JSON.stringify(registry));
  tui.start(ws);
  tui.setEnv({ TTT_CONFIG_DIR: configDir });
  tui.exec("Switch Theme");
  tui.type("plugins/theme-pack/light");
  const disabled = tui.snapshot();
  tui.press("esc");
  tui.exec("Plugins: Uninstall");
  tui.press("enter");
  tui.waitFor("Remove plugin");
  tui.press("tab");
  tui.press("enter");
  const restarted = tui.run();
  expect(restarted.snapshots[disabled]).toContain("plugins/theme-pack/light");
  expect(existsSync(installed)).toBe(false);
  expect(JSON.parse(readFileSync(settingsPath, "utf8")).theme).toBeFalsy();
});
