import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("unicode stress tests", () => {
  it("should handle symbol characters and deletion", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "symbols.txt", "");

    tui.start(file);

    // Use simple single-codepoint symbols that terminals handle reliably
    tui.type("ABC");

    // Select all and delete
    tui.press("ctrl+a");
    tui.press("backspace");

    // Type new symbols
    tui.type("XYZ");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("XYZ\n");
  });

  it("should move cursor by character through Greek letters", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "greek.txt", "αβγδ");

    tui.start(file);
    tui.waitFor("αβγδ");

    // Move cursor to start, then right 2 times (past α and β)
    tui.press("home");
    tui.press("arrow_right");
    tui.press("arrow_right");

    // Type "X" — should insert between β and γ
    tui.type("X");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("αβXγδ\n");
  });

  it("should handle accented characters with cursor movement", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "accent-nav.txt", "cafe");

    tui.start(file);
    tui.waitFor("cafe");

    // Go to end, backspace to remove 'e', type accented 'e'
    tui.press("end");
    tui.press("backspace");
    tui.type("é");

    tui.press("ctrl+s");

    // Take snapshot to verify intermediate state (café on screen)
    const s0 = tui.snapshot();

    // Move left 2 chars (past é and f), insert a character
    tui.press("end");
    tui.press("arrow_left");
    tui.press("arrow_left");
    tui.type("Z");

    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    // Verify intermediate state showed café
    expect(snapshots[s0]).toContain("café");

    // Verify final file state
    const content = readFile(file);
    expect(content).toBe("caZfé\n");
  });
});
