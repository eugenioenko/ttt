import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

function numbered(n) {
  return Array.from({ length: n }, (_, i) => `line ${i + 1}`).join("\n") + "\n";
}

describe("file start and end keys", () => {
  it("ctrl+end and ctrl+home jump to the end and start of the file", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "edges.txt", numbered(200));

    tui.start(file);
    tui.waitFor("line 1");
    tui.press("ctrl+end");
    const atEnd = tui.snapshot();
    tui.press("ctrl+home");
    tui.type("Y");
    tui.press("ctrl+s");
    const { snapshots } = tui.run();

    expect(snapshots[atEnd]).toContain("line 200");
    expect(snapshots[atEnd]).not.toMatch(/\bline 1\b/);
    expect(readFile(file).startsWith("Yline 1\n")).toBe(true);
  });

  it("ctrl+shift+home selects from the end of the file to its start", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "select.txt", numbered(200));

    tui.start(file);
    tui.waitFor("line 1");
    tui.press("ctrl+end");
    tui.press("ctrl+shift+home");
    tui.type("Z");
    tui.press("ctrl+s");
    tui.run();

    expect(readFile(file)).toBe("Z\n");
  });
});
