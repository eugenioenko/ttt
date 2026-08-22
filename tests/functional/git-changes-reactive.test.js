import { afterEach, describe, expect, it } from "vitest";
import { join } from "node:path";
import * as tui from "./tui.js";
import { cleanupDir, createGitRepo, createTempDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("reactive git changes", () => {
  it("shows a saved editor change without a manual refresh", () => {
    dir = createGitRepo(createTempDir());
    const tracked = join(dir, "tracked.txt");

    tui.start(dir, tracked);
    tui.waitFor("Explore");
    tui.exec("Show Changes");
    tui.waitStable(300);
    const clean = tui.snapshot();

    tui.exec("View: Focus Editor");
    tui.press("end");
    tui.type("changed");
    tui.exec("Save File");
    tui.waitStable(600);
    const updated = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[clean]).toContain("No changes");
    expect(snapshots[updated]).toContain("Changes (1)");
    expect(snapshots[updated]).toContain("tracked.txt");
  });
});
