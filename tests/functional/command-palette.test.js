import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

function paletteWidth(snapshot) {
  const borderRow = snapshot.split("\n")[2] || "";
  const matches = borderRow.matchAll(/(?:╭─+╮|╔═+╗|┌─+┐)/g);
  return Math.max(0, ...[...matches].map((match) => [...match[0]].length));
}

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("command palette", () => {
  it("should open command palette with ctrl+p", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "palette.txt", "Palette test");

    tui.start(file);
    tui.waitFor("palette.txt");

    tui.press("ctrl+p");
    tui.waitStable();

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain(">");
  });

  it("should execute a command from the palette", () => {
    dir = createTempDir();
    createTempFile(dir, "exec.txt", "Exec test");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.exec("View: Toggle Sidebar");
    tui.waitStable();

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).not.toContain("Explore");
  });

  it("should dismiss palette with escape", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "dismiss.txt", "Dismiss test");

    tui.start(file);
    tui.waitFor("dismiss.txt");

    tui.press("ctrl+p");
    tui.waitStable();

    tui.press("escape");
    tui.waitStable();

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Dismiss test");
  });

  it.each([
    { name: "narrow", width: 30, height: 24, expected: 26 },
    { name: "typical", width: 80, height: 24, expected: 48 },
    { name: "wide", width: 200, height: 50, expected: 90 },
  ])("should use the responsive width at $name terminal sizes", ({ width, height, expected }) => {
    dir = createTempDir();
    createTempFile(dir, "responsive.txt", "Responsive palette test");

    tui.start(dir);
    tui.setSize(width, height);
    tui.waitFor("responsive.txt");
    tui.press("ctrl+p");
    const palette = tui.snapshot();

    const { snapshots } = tui.run();
    expect(paletteWidth(snapshots[palette])).toBe(expected);
  });

  it("should keep one responsive width across modes and transitions", () => {
    dir = createTempDir();
    createTempFile(dir, "wide.txt", "Wide palette test");

    tui.start(dir);
    tui.setSize(200, 50);
    tui.waitFor("wide.txt");
    tui.press("ctrl+p");
    const command = tui.snapshot();

    tui.type("?");
    const help = tui.snapshot();

    tui.press("backspace");
    tui.type(">");
    const commandAgain = tui.snapshot();

    tui.press("backspace");
    const files = tui.snapshot();

    tui.type(":");
    const goToLine = tui.snapshot();

    tui.press("backspace");
    tui.click(54, 3);
    const dismissed = tui.snapshot();

    tui.press("ctrl+p");
    const reopened = tui.snapshot();
    tui.press("escape");
    tui.pressChord("ctrl+k", "p");
    const fileBinding = tui.snapshot();

    const { snapshots } = tui.run();
    expect(paletteWidth(snapshots[command])).toBe(90);
    expect(paletteWidth(snapshots[help])).toBe(90);
    expect(paletteWidth(snapshots[commandAgain])).toBe(90);
    expect(paletteWidth(snapshots[files])).toBe(90);
    expect(paletteWidth(snapshots[goToLine])).toBe(90);
    expect(paletteWidth(snapshots[dismissed])).toBe(0);
    expect(paletteWidth(snapshots[reopened])).toBe(90);
    expect(paletteWidth(snapshots[fileBinding])).toBe(90);
    expect(snapshots[command]).toContain(">");
    expect(snapshots[files]).toContain("wide.txt");
    expect(snapshots[fileBinding]).toContain("wide.txt");
  });

  it("should orient a new user before listing commands in help mode", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "help.txt", "Help mode test");

    tui.start(file);
    tui.setSize(80, 24);
    tui.waitFor("help.txt");
    tui.press("ctrl+p");
    tui.type("?");
    tui.waitStable();

    const topics = tui.snapshot();
    tui.press("arrow_down");
    const nextTopic = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[topics]).toContain("Workspace map");
    expect(snapshots[topics]).toContain("folders, tabs, and editor groups");
    expect(snapshots[topics]).not.toContain("Open Folder");
    expect(snapshots[nextTopic]).toContain("Explorer, Search, Changes, and Output");
  });

  it("should search commands outside the curated help topics", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "help-search.txt", "Help search test");

    tui.start(file);
    tui.waitFor("help-search.txt");
    tui.press("ctrl+p");
    tui.type("?saved tabs");
    tui.waitStable();

    const result = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[result]).toContain("Close All Saved Tabs");
  });

  it("should prefer precise help matches and explain an empty result", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "help-precision.txt", "Help precision test");

    tui.start(file);
    tui.setSize(80, 24);
    tui.waitFor("help-precision.txt");
    tui.press("ctrl+p");
    tui.type("?undo");
    tui.waitStable();
    const precise = tui.snapshot();

    for (let i = 0; i < 4; i++) tui.press("backspace");
    tui.type("qxzvjk");
    tui.waitStable();
    const empty = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[precise]).toContain("Undo");
    expect(snapshots[precise]).toContain("Undo Last Cursor");
    expect(snapshots[precise]).not.toContain("Git: Discard Changes");
    expect(snapshots[precise]).not.toContain("Add Next Occurrence");
    expect(snapshots[empty]).toContain('No help entries match "qxzvjk"');
    expect(snapshots[empty]).toContain("Try > for all commands");
  });
});
