// Regression for BUG-041 from audit/2026-07-12-ux-bug-audit.md.
import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("BUG-041: theme picker cancel leaves border charset stuck on the preview", () => {
  it("Escape reverts border glyphs to the pre-picker theme", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "th.txt", "sample content\nline two\n");

    tui.start(file);
    tui.waitFor("sample");

    tui.exec("Switch Theme");
    tui.type("turbo"); // preview turbo-vision (double-line borders)
    tui.press("escape"); // cancel — should fully revert
    tui.waitStable();
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    // The default theme uses rounded borders (╭); the cancelled Turbo Vision
    // preview must not leave its double-line glyph (╔) behind.
    expect(snapshots[s]).toContain("╭");
    expect(snapshots[s]).not.toContain("╔");
  });
});
