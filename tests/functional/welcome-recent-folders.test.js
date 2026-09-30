import { describe, it, expect, afterEach } from "vitest";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("welcome recent folders", () => {
  it("purges history and does not record folders while disabled", () => {
    dir = createTempDir();
    const configDir = join(dir, "config");
    const privateFolder = join(dir, "private-project");
    const openedFolder = join(dir, "opened-project");
    mkdirSync(configDir, { recursive: true });
    mkdirSync(privateFolder);
    mkdirSync(openedFolder);
    writeFileSync(
      join(configDir, "settings.json"),
      JSON.stringify({ welcome: { recentFolders: false } }),
    );
    writeFileSync(
      join(configDir, "state.json"),
      JSON.stringify({ sidebarWidth: 22, recentFolders: [privateFolder] }),
    );

    tui.start("--welcome");
    tui.setEnv({ TTT_CONFIG_DIR: configDir });
    tui.waitFor("Open Folder…");
    const welcome = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[welcome]).not.toContain("Recent");
    expect(snapshots[welcome]).not.toContain("private-project");

    let state = JSON.parse(readFileSync(join(configDir, "state.json"), "utf8"));
    expect(state.sidebarWidth).toBe(22);
    expect(state.recentFolders).toBeUndefined();

    tui.start(openedFolder);
    tui.setEnv({ TTT_CONFIG_DIR: configDir });
    tui.run();

    state = JSON.parse(readFileSync(join(configDir, "state.json"), "utf8"));
    expect(state.sidebarWidth).toBe(22);
    expect(state.recentFolders).toBeUndefined();
  });
});
