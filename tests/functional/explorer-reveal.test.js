import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir } from "./helpers.js";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

function setup(settings) {
  dir = createTempDir();
  const project = join(dir, "proj");
  mkdirSync(join(project, "src", "deep"), { recursive: true });
  mkdirSync(join(project, "docs"), { recursive: true });
  const file = join(project, "src", "deep", "main.go");
  writeFileSync(file, "package main\n");
  const configFile = join(dir, "settings.json");
  writeFileSync(configFile, JSON.stringify(settings));
  return { project, file, configFile };
}

describe("explorer reveal", () => {
  it("reveals the active file when explorer.autoReveal is on", () => {
    const { project, file, configFile } = setup({ explorer: { autoReveal: true } });

    tui.start("--config", configFile, project, file);
    tui.waitFor("package main");
    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("▼ deep");
    expect(snapshots[s0]).toMatch(/│\s+main\.go/);
  });

  it("leaves the tree alone when explorer.autoReveal is off", () => {
    const { project, file, configFile } = setup({});

    tui.start("--config", configFile, project, file);
    tui.waitFor("package main");
    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("▶ src");
    expect(snapshots[s0]).not.toContain("deep");
  });

  it("reveals the active file from the command palette", () => {
    const { project, file, configFile } = setup({});

    tui.start("--config", configFile, project, file);
    tui.waitFor("package main");
    tui.exec("Explorer: Reveal Active File");
    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("▼ deep");
    expect(snapshots[s0]).toMatch(/│\s+main\.go/);
  });
});
