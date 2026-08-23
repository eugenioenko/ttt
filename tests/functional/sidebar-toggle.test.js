import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("sidebar toggle", () => {
  it("should toggle sidebar with ctrl+b", () => {
    dir = createTempDir();
    createTempFile(dir, "side.txt", "Sidebar test");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.press("ctrl+b");

    const s0 = tui.snapshot();

    tui.press("ctrl+b");
    tui.waitFor("Explore");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).not.toContain("Explore");
    expect(snapshots[s1]).toContain("Explore");
  });

  it("should reset sidebar width after being narrowed below minimum", () => {
    dir = createTempDir();
    createTempFile(dir, "width.txt", "Width reset test");

    tui.start(dir);
    tui.waitFor("Explore");

    // Narrow sidebar to near-zero width using the command palette
    for (let i = 0; i < 25; i++) {
      tui.exec("View: Decrease Sidebar Width");
    }

    // Toggle sidebar off
    tui.press("ctrl+b");
    const s0 = tui.snapshot();

    // Toggle sidebar back on — should reset to usable default width
    tui.press("ctrl+b");
    tui.waitFor("Explore");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).not.toContain("Explore");
    expect(snapshots[s1]).toContain("Explore");
  });

  it("should switch sidebar panels with chord keys", () => {
    dir = createTempDir();
    createTempFile(dir, "panels.txt", "Panel test");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.pressChord("ctrl+k", "f");
    tui.waitFor("Find");

    const s0 = tui.snapshot();

    tui.pressChord("ctrl+k", "e");
    tui.waitFor("Explore");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Find");
    expect(snapshots[s1]).toContain("Explore");
  });
});
