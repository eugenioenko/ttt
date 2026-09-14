// Repro test for confirmed bug from audit/2026-07-12-ux-bug-audit.md (branch audit/bug-hunt).
// BUG-041 was fixed by restoring *a.Borders in the theme picker OnDismiss
// (auto-theme change); the `.fails` marker was removed accordingly.
// Asserts the CORRECT behavior — goes red if the bug regresses.
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
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    // The default theme uses rounded borders (╭). Buggy: OnDismiss reverts
    // the style map and palette but never resets *a.Borders, so the
    // double-line preview glyph (╔) stays until another theme is applied.
    expect(snapshots[s]).toContain("╭");
    expect(snapshots[s]).not.toContain("╔");
  });
});
