import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("menu bar toggle", () => {
  it("should hide and restore the menu bar with alt+shift+m", () => {
    dir = createTempDir();
    createTempFile(dir, "menu.txt", "Menu bar test");

    tui.start(dir);
    tui.waitFor("Options");
    const s0 = tui.snapshot();

    tui.press("alt+shift+m");
    tui.waitStable();
    const s1 = tui.snapshot();

    tui.press("alt+shift+m");
    tui.waitStable();
    const s2 = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Selection");
    expect(snapshots[s1]).not.toContain("Selection");
    expect(snapshots[s2]).toContain("Selection");
  });

  it("should hide the menu bar from the command palette and say how to restore it", () => {
    dir = createTempDir();
    createTempFile(dir, "menu.txt", "Menu bar test");

    tui.start(dir);
    tui.waitFor("Options");

    tui.exec("View: Toggle Menu Bar");
    tui.waitStable();
    const s0 = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[s0]).not.toContain("Selection");
    expect(snapshots[s0]).toContain("Menu bar hidden");
    expect(snapshots[s0]).toContain("Alt+Shift+M");
  });

  it("should open menu dropdowns while the menu bar stays hidden", () => {
    dir = createTempDir();
    createTempFile(dir, "menu.txt", "Menu bar test");

    tui.start(dir);
    tui.waitFor("Options");

    tui.press("alt+shift+m");
    tui.waitStable();

    tui.exec("Menu: View");
    tui.waitStable();
    const s0 = tui.snapshot();

    tui.press("escape");
    tui.waitStable();
    const s1 = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Command Palette");
    expect(snapshots[s0]).not.toContain("Selection");
    expect(snapshots[s1]).not.toContain("Command Palette");
    expect(snapshots[s1]).not.toContain("Selection");
  });
});
