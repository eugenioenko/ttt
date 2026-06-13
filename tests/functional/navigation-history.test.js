import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createMultiLineFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("navigation history", () => {
  it("should navigate back after go-to-line with alt+left", () => {
    dir = createTempDir();
    const file = createMultiLineFile(dir, "nav.txt", 50);

    tui.start(file);
    tui.waitFor("Ln 1");

    // Go to line 25
    tui.press("ctrl+g");
    tui.waitStable();
    tui.type("25");
    tui.press("enter");
    tui.waitStable();

    let snap = tui.snapshot();
    expect(snap).toContain("Ln 25");

    // Navigate back
    tui.press("alt+left");
    tui.waitStable();

    snap = tui.snapshot();
    expect(snap).toContain("Ln 1");
  });

  it("should navigate forward after going back with alt+right", () => {
    dir = createTempDir();
    const file = createMultiLineFile(dir, "nav2.txt", 50);

    tui.start(file);
    tui.waitFor("Ln 1");

    // Go to line 30
    tui.press("ctrl+g");
    tui.waitStable();
    tui.type("30");
    tui.press("enter");
    tui.waitStable();

    let snap = tui.snapshot();
    expect(snap).toContain("Ln 30");

    // Navigate back
    tui.press("alt+left");
    tui.waitStable();
    snap = tui.snapshot();
    expect(snap).toContain("Ln 1");

    // Navigate forward
    tui.press("alt+right");
    tui.waitStable();
    snap = tui.snapshot();
    expect(snap).toContain("Ln 30");
  });

  it("should do nothing when there is no history", () => {
    dir = createTempDir();
    const file = createMultiLineFile(dir, "nav3.txt", 10);

    tui.start(file);
    tui.waitFor("Ln 1");

    // Press alt+left with no history - should be a no-op
    tui.press("alt+left");
    tui.waitStable();

    const snap = tui.snapshot();
    expect(snap).toContain("Ln 1");
  });
});
