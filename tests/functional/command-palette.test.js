import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

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
