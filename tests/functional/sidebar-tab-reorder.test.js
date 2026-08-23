import { describe, it, expect, afterEach } from "vitest";
import { mkdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("sidebar tab order", () => {
  it("drags Changes before Explore and persists the order", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "file.txt", "hello\n");
    const configDir = join(dir, "config");
    mkdirSync(configDir, { recursive: true });

    tui.start(file);
    tui.setEnv({ TTT_CONFIG_DIR: configDir });
    tui.pressChord("ctrl+k", "c");
    const before = tui.snapshot();
    tui.drag(16, 2, 1, 2);
    const after = tui.snapshot();

    const { snapshots } = tui.run();
    const beforeHeader = snapshots[before].split("\n")[2];
    const afterHeader = snapshots[after].split("\n")[2];
    expect(beforeHeader.indexOf("Changes")).toBeGreaterThan(beforeHeader.indexOf("Explore"));
    expect(afterHeader.indexOf("Changes")).toBeLessThan(afterHeader.indexOf("Explore"));

    const saved = JSON.parse(readFileSync(join(configDir, "settings.json"), "utf8"));
    expect(saved.sidebar.panelOrder.slice(0, 2)).toEqual(["changes", "explorer"]);
  });
});
