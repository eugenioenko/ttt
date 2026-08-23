import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir, fileExists } from "./helpers.js";
import { join } from "node:path";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("dialog paste", () => {
  it("should paste bracketed text into Save As dialog input", () => {
    dir = createTempDir();
    const filePath = join(dir, "pasted.txt");

    tui.start();

    // Open Save As dialog (new file has no path)
    tui.press("ctrl+s");

    // Simulate bracketed paste into the dialog
    tui.paste(filePath);

    const s0 = tui.snapshot();

    // Confirm with Enter
    tui.press("enter");

    const { snapshots } = tui.run();

    // The dialog should show the pasted path
    expect(snapshots[s0]).toContain("pasted.txt");

    // The file should have been created at the pasted path
    expect(fileExists(filePath)).toBe(true);
  });
});
