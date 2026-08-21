import { describe, it, expect, afterEach } from "vitest";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
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

describe("diff reading preferences", () => {
  it("marks removed and added line numbers in the shared diff gutter", () => {
    const fixture = diffFixture();
    startWithConfig(fixture);
    tui.exec("Test Preference Diff");
    tui.waitStable();
    const diff = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[diff]).toMatch(/1 -.*1 \+/);
  });

  it("shows persistent diff defaults in Options even without an open diff", () => {
    const fixture = diffFixture();
    startWithConfig(fixture);

    tui.exec("Menu: Options");
    tui.waitStable();
    const options = tui.snapshot();
    tui.press("down");
    tui.press("down");
    tui.press("right");
    tui.waitStable();
    const diffOptions = tui.snapshot();
    tui.press("escape");
    enableUnifiedWrappedDefaults();

    const { snapshots } = tui.run();
    expect(snapshots[options]).toContain("Diff View");
    expect(snapshots[options]).toContain("Git Files");
    expect(snapshots[diffOptions]).toContain("Split");
    expect(snapshots[diffOptions]).toContain("Unified");
    expect(snapshots[diffOptions]).toContain("Wrap Lines");
    expect(snapshots[diffOptions]).toContain("High Contrast");

    const saved = savedSettings(fixture.configDir);
    expect(saved.editor.diffMode).toBe("unified");
    expect(saved.editor.diffWordWrap).toBe(true);
  });

  it("updates an already-open diff when its Options defaults change", () => {
    const fixture = diffFixture();
    startWithConfig(fixture);
    tui.exec("Test Preference Diff");
    tui.waitStable();
    const before = tui.snapshot();

    enableUnifiedWrappedDefaults();
    const after = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[before]).toContain("● Split");
    expect(snapshots[before]).not.toContain("SUFFIX");
    expect(snapshots[after]).toContain("● Unified");
    expect(snapshots[after]).toContain("SUFFIX");
  });

  it("keeps View overrides when Options defaults later change", () => {
    const fixture = diffFixture();
    startWithConfig(fixture);
    tui.exec("Test Preference Diff");
    tui.waitStable();

    tui.exec("Git: Unified Diff");
    tui.exec("Git: Toggle Diff Wrap");
    enableUnifiedWrappedDefaults();
    tui.exec("Use Split Diff by Default");
    tui.exec("Toggle Diff Word Wrap Default");
    tui.waitStable();
    const overridden = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[overridden]).toContain("○ Split");
    expect(snapshots[overridden]).toContain("● Unified");
    expect(snapshots[overridden]).toContain("SUFFIX");

    const saved = savedSettings(fixture.configDir);
    expect(saved.editor.diffMode).toBe("split");
    expect(saved.editor.diffWordWrap).toBe(false);
  });

  it("updates only the untouched diff when one of two open diffs is overridden", () => {
    const fixture = diffFixture();
    startWithConfig(fixture);
    tui.exec("Test First Preference Diff");
    tui.exec("Test Second Preference Diff");
    tui.exec("Git: Unified Diff");
    tui.exec("Git: Toggle Diff Wrap");
    tui.waitStable();
    const overriddenBefore = tui.snapshot();

    enableUnifiedWrappedDefaults();
    const overriddenAfter = tui.snapshot();
    tui.exec("View: Previous Tab");
    const untouchedAfter = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[overriddenBefore]).toContain("● Unified");
    expect(snapshots[overriddenBefore]).toContain("SUFFIX");
    expect(snapshots[overriddenAfter]).toContain("● Unified");
    expect(snapshots[overriddenAfter]).toContain("SUFFIX");
    expect(snapshots[untouchedAfter]).toContain("● Unified");
    expect(snapshots[untouchedAfter]).toContain("SUFFIX");
  });

  it("opens a new diff with defaults loaded from the prior run", () => {
    const fixture = diffFixture();
    startWithConfig(fixture);
    enableUnifiedWrappedDefaults();
    tui.run();

    startWithConfig(fixture);
    tui.exec("Test Preference Diff");
    tui.waitStable();
    const reopened = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[reopened]).toContain("○ Split");
    expect(snapshots[reopened]).toContain("● Unified");
    expect(snapshots[reopened]).toContain("SUFFIX");
  });
});
