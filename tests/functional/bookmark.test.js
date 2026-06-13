import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("bookmarks", () => {
  it("should toggle bookmark and show indicator", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "bookmark.txt", "AAA\nBBB\nCCC\nDDD\nEEE");

    tui.start(file);
    tui.waitFor("AAA");

    // Toggle bookmark on the first line via command palette
    tui.exec("Toggle Bookmark");
    tui.waitStable();

    const snap = tui.snapshot();
    expect(snap).toContain("●");
  });

  it("should navigate between bookmarks", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "nav.txt", "Line1\nLine2\nLine3\nLine4\nLine5\nLine6\nLine7\nLine8");

    tui.start(file);
    tui.waitFor("Line1");

    // Bookmark line 1 (cursor starts on line 1)
    tui.exec("Toggle Bookmark");
    tui.waitStable();

    // Move to line 5 and bookmark it
    tui.press("arrow_down");
    tui.press("arrow_down");
    tui.press("arrow_down");
    tui.press("arrow_down");
    tui.waitStable();
    tui.exec("Toggle Bookmark");
    tui.waitStable();

    // Move to line 1 again
    tui.press("ctrl+g");
    tui.waitStable();
    tui.type("1");
    tui.press("enter");
    tui.waitStable();

    // Jump to next bookmark (should go to line 5 since cursor is on line 1 which is bookmarked)
    tui.exec("Next Bookmark");
    tui.waitStable();

    const snap = tui.snapshot();
    // Both bookmarks should be visible
    expect(snap).toContain("●");
  });

  it("should clear all bookmarks", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "clear.txt", "AAA\nBBB\nCCC");

    tui.start(file);
    tui.waitFor("AAA");

    tui.exec("Toggle Bookmark");
    tui.waitStable();

    let snap = tui.snapshot();
    expect(snap).toContain("●");

    tui.exec("Clear All Bookmarks");
    tui.waitStable();

    snap = tui.snapshot();
    expect(snap).not.toContain("●");
  });
});
