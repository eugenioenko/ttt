import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("home and end keys", () => {
  it("should jump to end of line with End key", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "end.txt", "hello world");

    tui.start(file);
    tui.waitFor("hello world");

    tui.press("end");
    tui.type("X");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("hello worldX\n");
  });

  it("should jump to start of line with Home key", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "home.txt", "hello world");

    tui.start(file);
    tui.waitFor("hello world");

    tui.press("end");
    tui.press("home");
    tui.type("X");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("Xhello world\n");
  });

  it("should work with Home/End on second line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "line2.txt", "first\nsecond line");

    tui.start(file);
    tui.waitFor("second line");

    tui.press("arrow_down");
    tui.press("end");
    tui.type("X");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("first\nsecond lineX\n");
  });

  it("should handle Home on indented line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "indent.txt", "    indented");

    tui.start(file);
    tui.waitFor("indented");

    tui.press("end");
    tui.press("home");
    tui.type("X");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    // Smart home goes to first non-whitespace; plain home goes to column 0
    const smartHome = "    Xindented\n";
    const plainHome = "X    indented\n";
    expect([smartHome, plainHome]).toContain(content);
  });

  it("should handle End then type on empty line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "empty.txt", "hello\n\nworld");

    tui.start(file);
    tui.waitFor("hello");

    tui.press("arrow_down");
    tui.press("end");
    tui.type("MIDDLE");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("hello\nMIDDLE\nworld\n");
  });
});
