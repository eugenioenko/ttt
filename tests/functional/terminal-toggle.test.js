import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("terminal toggle", () => {
  it("should show terminal panel with ctrl+t", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "term.txt", "Editor content");

    tui.start(file);
    tui.waitFor("term.txt");

    tui.press("ctrl+t");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Terminal");
  });

  it("should toggle terminal off and on", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "toggle.txt", "Toggle test");

    tui.start(file);
    tui.waitFor("toggle.txt");

    tui.press("ctrl+t");

    tui.press("ctrl+t");

    const s0 = tui.snapshot();

    tui.press("ctrl+t");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).not.toContain("Terminal  Problems");
    expect(snapshots[s1]).toContain("Terminal");
  });
});
