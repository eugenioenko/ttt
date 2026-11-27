import { mkdirSync } from "node:fs";
import { join } from "node:path";
import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("BUG-041: theme picker cancel leaves border charset stuck on the preview", () => {
  it("Escape reverts border glyphs to the pre-picker theme", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "th.txt", "sample content\nline two\n");
    const configDir = join(dir, "config");
    const themesDir = join(configDir, "themes");
    mkdirSync(themesDir, { recursive: true });
    createTempFile(configDir, "settings.json", JSON.stringify({ theme: "", editor: { borderStyle: "default" } }));
    createTempFile(themesDir, "audit-double.json", JSON.stringify({
      borders: {
        horizontal: "═", vertical: "║",
        topLeft: "╔", topRight: "╗", bottomLeft: "╚", bottomRight: "╝",
        topTee: "╦", bottomTee: "╩", leftTee: "╠", rightTee: "╣",
      },
    }));

    tui.start(file);
    tui.setEnv({ TTT_CONFIG_DIR: configDir });
    tui.waitFor("sample");
    const before = tui.snapshot();

    tui.exec("Switch Theme");
    tui.type("audit-double");
    const preview = tui.snapshot();
    tui.press("escape");
    const after = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[before]).toContain("╭");
    expect(snapshots[before]).not.toContain("╔");
    expect(snapshots[preview]).toContain("Select Theme");
    expect(snapshots[preview]).toContain("╔");
    expect(snapshots[after]).not.toContain("Select Theme");
    expect(snapshots[after]).toContain("╭");
    expect(snapshots[after]).not.toContain("╔");
  });
});
