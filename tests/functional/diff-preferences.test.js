import { afterEach, describe, expect, it } from "vitest";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { cleanupDir, createTempDir, createTempFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
  dir = undefined;
});

function diffFixture() {
  dir = createTempDir();
  const file = createTempFile(dir, "test.txt", "hello\n");
  const configDir = join(dir, "config");
  mkdirSync(configDir, { recursive: true });
  const plugin = join(dir, "diff.lua");
  writeFileSync(plugin, `
    local ttt = require("ttt")
    ttt.register({
      commands = {
        { id = "test.diff", title = "Test Preference Diff", handler = function()
            ttt.open_diff("prefs.go", {"left-prefix-LEFT-SUFFIX"}, {"right-prefix-RIGHT-SUFFIX"}, "prefs.go")
          end
        },
        { id = "test.diffFirst", title = "Test First Preference Diff", handler = function()
            ttt.open_diff("first.go", {"left-prefix-LEFT-SUFFIX"}, {"right-prefix-RIGHT-SUFFIX"}, "first.go")
          end
        },
        { id = "test.diffSecond", title = "Test Second Preference Diff", handler = function()
            ttt.open_diff("second.go", {"left-prefix-LEFT-SUFFIX"}, {"right-prefix-RIGHT-SUFFIX"}, "second.go")
          end
        }
      },
    })
  `, "utf8");
  return { file, configDir, plugin };
}

function startWithConfig({ file, configDir, plugin }) {
  tui.start("--plugin", plugin, file);
  tui.setEnv({ TTT_CONFIG_DIR: configDir });
  tui.setSize(42, 14);
  tui.waitStable(300);
}

function enableUnifiedWrappedDefaults() {
  tui.exec("Use Unified Diff by Default");
  tui.exec("Toggle Diff Word Wrap Default");
  tui.waitStable();
}

function savedSettings(configDir) {
  return JSON.parse(readFileSync(join(configDir, "settings.json"), "utf8"));
}

function diffRow(screen, text) {
  return screen.split("\n").findIndex((row) => row.includes(text));
}

describe("diff reading preferences", () => {
  it("marks removed and added line numbers in the shared diff gutter", () => {
    const fixture = diffFixture();
    startWithConfig(fixture);
    tui.exec("Test Preference Diff");
    tui.waitStable();
    const snapshot = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[snapshot]).toMatch(/1 −.*1 \+/u);
  });

  it("shows and persists global diff controls under Options", () => {
    const fixture = diffFixture();
    startWithConfig(fixture);
    tui.exec("Menu: Options");
    tui.waitStable();
    const options = tui.snapshot();
    for (let i = 0; i < 9; i++) tui.press("down");
    tui.press("right");
    const diffViews = tui.snapshot();
    tui.press("escape");
    enableUnifiedWrappedDefaults();
    tui.exec("Show Full File Diff by Default");
    tui.exec("Toggle High Contrast Diffs");

    const { snapshots } = tui.run();
    expect(snapshots[options]).toContain("Diff Views");
    expect(snapshots[options]).toContain("Git Files");
    expect(snapshots[diffViews]).toContain("Split");
    expect(snapshots[diffViews]).toContain("Unified");
    expect(snapshots[diffViews]).toContain("Changes Only");
    expect(snapshots[diffViews]).toContain("Full File");
    expect(snapshots[diffViews]).toContain("Wrap Lines");
    expect(snapshots[diffViews]).toContain("High Contrast");

    const saved = savedSettings(fixture.configDir);
    expect(saved.editor.diffMode).toBe("unified");
    expect(saved.editor.diffContext).toBe("full");
    expect(saved.editor.diffWordWrap).toBe(true);
    expect(saved.editor.diffHighContrast).toBe(true);
  });

  it("updates an inherited open diff when Options defaults change", () => {
    const fixture = diffFixture();
    startWithConfig(fixture);
    tui.exec("Test Preference Diff");
    tui.waitStable();
    const before = tui.snapshot();
    enableUnifiedWrappedDefaults();
    const after = tui.snapshot();

    const { snapshots } = tui.run();
    expect(diffRow(snapshots[before], "left-prefix")).toBe(diffRow(snapshots[before], "right-prefix"));
    expect(snapshots[before]).not.toContain("SUFFIX");
    expect(diffRow(snapshots[after], "left-prefix")).toBeLessThan(diffRow(snapshots[after], "right-prefix"));
    expect(snapshots[after]).toContain("SUFFIX");
  });

  it("preserves explicit surface overrides independently", () => {
    const fixture = diffFixture();
    startWithConfig(fixture);
    tui.exec("Test First Preference Diff");
    tui.exec("Test Second Preference Diff");
    tui.exec("Git: Unified Diff");
    tui.exec("Git: Toggle Diff Wrap");
    enableUnifiedWrappedDefaults();
    tui.exec("Use Split Diff by Default");
    tui.exec("Toggle Diff Word Wrap Default");
    tui.waitStable();
    const overridden = tui.snapshot();
    tui.exec("View: Previous Tab");
    const inherited = tui.snapshot();

    const { snapshots } = tui.run();
    expect(diffRow(snapshots[overridden], "left-prefix")).toBeLessThan(diffRow(snapshots[overridden], "right-prefix"));
    expect(snapshots[overridden]).toContain("SUFFIX");
    expect(diffRow(snapshots[inherited], "left-prefix")).toBe(diffRow(snapshots[inherited], "right-prefix"));
    expect(snapshots[inherited]).not.toContain("SUFFIX");
  });

  it("loads persisted diff defaults on the next run", () => {
    const fixture = diffFixture();
    startWithConfig(fixture);
    enableUnifiedWrappedDefaults();
    tui.run();

    startWithConfig(fixture);
    tui.exec("Test Preference Diff");
    tui.waitStable();
    const reopened = tui.snapshot();
    const { snapshots } = tui.run();
    expect(diffRow(snapshots[reopened], "left-prefix")).toBeLessThan(diffRow(snapshots[reopened], "right-prefix"));
    expect(snapshots[reopened]).toContain("SUFFIX");
  });
});
