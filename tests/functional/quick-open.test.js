import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("quick open (Go to File)", () => {
  it("should open quick open dialog with ctrl+k p", () => {
    dir = createTempDir();
    createTempFile(dir, "alpha.txt", "alpha content");
    createTempFile(dir, "beta.txt", "beta content");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.pressChord("ctrl+k", "p");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("alpha.txt");
    expect(snapshots[s0]).toContain("beta.txt");
  });

  it("should filter files by typing", () => {
    dir = createTempDir();
    createTempFile(dir, "apple.txt", "apple content");
    createTempFile(dir, "banana.txt", "banana content");
    createTempFile(dir, "cherry.txt", "cherry content");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.pressChord("ctrl+k", "p");

    tui.type("ban");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("banana");
  });

  it("should open the selected file on enter", () => {
    dir = createTempDir();
    createTempFile(dir, "target.txt", "TARGET CONTENT");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.pressChord("ctrl+k", "p");

    tui.type("target");

    tui.press("enter");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("TARGET CONTENT");
  });

  it("should dismiss quick open with escape", () => {
    dir = createTempDir();
    createTempFile(dir, "file.txt", "file content");

    tui.start(dir);
    tui.waitFor("Explore");

    tui.pressChord("ctrl+k", "p");

    const s0 = tui.snapshot();

    tui.press("escape");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();

    // Dialog should be visible in the first snapshot
    expect(snapshots[s0]).toContain("file.txt");
    // After escape, the overlay should be gone
    expect(snapshots[s1]).not.toContain("Go to File");
  });
});
