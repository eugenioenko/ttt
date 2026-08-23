import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

function openEditor() {
  dir = createTempDir();
  const file = createTempFile(dir, "sample.txt", "alpha\nbeta\ngamma\n");
  tui.start(file);
  tui.waitFor("alpha");
  return file;
}

describe("settings editor", () => {
  it("opens as an editor tab with every category", () => {
    openEditor();

    tui.exec("Settings: Open Editor Settings");
    const s0 = tui.snapshot();

    const { snapshots } = tui.run();
    for (const want of [
      "Settings",
      "Editor",
      "Appearance",
      "Completion",
      "Advanced",
      "Tab size",
      "Word wrap",
      "Apply",
    ]) {
      expect(snapshots[s0]).toContain(want);
    }
  });

  it("shows current values", () => {
    openEditor();

    tui.exec("Settings: Open Editor Settings");
    const s0 = tui.snapshot();

    const { snapshots } = tui.run();
    // One row per setting: label left, control right.
    expect(snapshots[s0]).toMatch(/Insert spaces\s+\[x\]/);
    expect(snapshots[s0]).toMatch(/Word wrap\s+\[ \]/);
    expect(snapshots[s0]).toContain("Tab size");
    expect(snapshots[s0]).toMatch(/Tab size\s+❯ 4/);
  });

  // Deferral itself is an app-state assertion and lives in the e2e suite
  // (TestSettingsEditsDeferUntilApply). What only the real binary can prove is
  // the round trip: that Apply writes settings.json and a freshly built form
  // reads the value back.
  it("persists a toggled setting through settings.json", () => {
    openEditor();

    tui.exec("Settings: Open Editor Settings");

    // Tab onto Word wrap. The assertions name that row, so a traversal that
    // lands elsewhere fails loudly instead of passing on a no-op.
    tui.press("tab");
    tui.press("tab");
    tui.press("tab");
    tui.press("space");
    const toggled = tui.snapshot();

    tui.exec("Settings: Apply Changes");
    const applied = tui.snapshot();

    // Reload from disk, then close and reopen. Without the reload the form is
    // rebuilt from the in-memory settings ApplySettings already updated, and
    // would pass even if nothing ever reached settings.json.
    tui.exec("Reload Settings");
    tui.exec("Settings: Discard Changes");
    tui.exec("Settings: Open Editor Settings");
    const reopened = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[toggled]).toMatch(/Word wrap\s+\[x\]/);
    expect(snapshots[applied]).toContain("Settings applied");
    expect(snapshots[reopened]).toMatch(/Word wrap\s+\[x\]/);
  });

  it("keeps a single settings tab when reopened", () => {
    openEditor();

    tui.exec("Settings: Open Editor Settings");
    tui.exec("Settings: Open Editor Settings");
    const s0 = tui.snapshot();

    const { snapshots } = tui.run();
    const tabBar = snapshots[s0].split("\n").find((l) => l.includes("Settings x"));
    expect(tabBar).toBeTruthy();
    expect(tabBar.match(/Settings x/g)).toHaveLength(1);
  });

  // Regression for issue #414: Ctrl+C (editor.copy) while the settings tab was
  // active crashed with a nil dereference — settings tabs have no buffer,
  // cursor, or selection. The copy must be a safe no-op and the app must keep
  // rendering the settings UI.
  it("survives editor clipboard keys while the settings tab is active", () => {
    openEditor();

    tui.exec("Settings: Open Editor Settings");
    tui.press("ctrl+c");
    tui.press("ctrl+x");
    tui.press("ctrl+v");
    tui.press("ctrl+a");
    const s0 = tui.snapshot();

    const { snapshots } = tui.run();
    // A crash would leave an empty/errored snapshot; the settings UI must
    // still be on screen and intact.
    expect(snapshots[s0]).toContain("Settings");
    expect(snapshots[s0]).toContain("Word wrap");
    expect(snapshots[s0]).toContain("Apply");
  });

  it("still offers the raw JSON path", () => {
    openEditor();

    tui.exec("Settings: Open settings.json");
    const s0 = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("settings.json");
  });
});
