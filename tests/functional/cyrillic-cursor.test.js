import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("cursor position with multi-byte UTF-8 characters", () => {
  it("cursor column is correct after typing Cyrillic", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "cyrillic.txt", "");

    tui.start(file);

    // Type 6 Cyrillic characters: привет
    tui.type("привет");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // 6 runes typed; cursor should be at column 7 (1-indexed, after last char).
    // If using byte length, it would show Col 13 (6 × 2 bytes + 1).
    expect(snapshots[s0]).toContain("Col 7");
  });

  it("cursor column advances correctly mixing ASCII and Cyrillic", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "mixed2.txt", "");

    tui.start(file);

    // Type: hiпривет (2 ASCII + 6 Cyrillic = 8 runes)
    tui.type("hiпривет");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // 8 runes typed; cursor should be at column 9.
    expect(snapshots[s0]).toContain("Col 9");
  });

  it("cursor stays after last Cyrillic character when navigating", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "nav.txt", "привет\n");

    tui.start(file);

    // Move to end of line
    tui.press("end");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // End of "привет" (6 runes) should be Col 7.
    expect(snapshots[s0]).toContain("Col 7");
  });

  it("home/end work correctly with Cyrillic line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "cyrline.txt", "привет\n");

    tui.start(file);

    // Start at Col 1
    const s0 = tui.snapshot();

    tui.press("end");
    const s1 = tui.snapshot();

    tui.press("home");
    const s2 = tui.snapshot();

    const { snapshots } = tui.run();

    // Home should be Col 1
    expect(snapshots[s0]).toContain("Col 1");
    // End should be Col 7 (after 6 runes)
    expect(snapshots[s1]).toContain("Col 7");
    // Home again should be Col 1
    expect(snapshots[s2]).toContain("Col 1");
  });
});
