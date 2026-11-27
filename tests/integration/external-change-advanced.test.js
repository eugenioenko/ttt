import { describe, it, expect, afterEach } from "vitest";
import { writeFileSync, unlinkSync } from "node:fs";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("advanced external file change scenarios", () => {
  it("reloads a clean buffer when the file is externally changed", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "clean-reload.txt", "ORIGINAL_CLEAN");

    tui.start(file);
    tui.waitFor("ORIGINAL_CLEAN");

    // Externally overwrite the file
    writeFileSync(file, "EXTERNALLY_MODIFIED", "utf8");

    // Wait for the editor to detect the change and reload
    tui.waitFor("EXTERNALLY_MODIFIED");

    const snap = tui.snapshot();
    expect(snap).toContain("EXTERNALLY_MODIFIED");
    expect(snap).not.toContain("ORIGINAL_CLEAN");
  });

  it("does not silently reload when the buffer is dirty", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "dirty-conflict.txt", "BASE_CONTENT");

    tui.start(file);
    tui.waitFor("BASE_CONTENT");

    // Make the buffer dirty with a user edit
    tui.press("end");
    tui.type(" USER_EDIT");
    tui.waitFor("BASE_CONTENT USER_EDIT");

    // Externally overwrite the file
    writeFileSync(file, "EXTERNAL_OVERWRITE", "utf8");
    tui.waitStable(1000);

    // The editor must preserve the user's unsaved work
    const snap = tui.snapshot();
    expect(snap).toContain("USER_EDIT");
    expect(snap).not.toContain("EXTERNAL_OVERWRITE");
  });

  it("warns about external file deletion and preserves the buffer", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "deleteme.txt", "CONTENT_BEFORE_DELETE");

    tui.start(file);
    tui.waitFor("CONTENT_BEFORE_DELETE");

    unlinkSync(file);
    tui.waitFor("deleteme.txt was deleted on disk");

    const snap = tui.snapshot();
    expect(snap).toContain("deleteme.txt was deleted on disk");
    expect(snap).toContain("CONTENT_BEFORE_DELETE");
  });

  it("settles to the final version after rapid external changes", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "rapid.txt", "RAPID_V0");

    tui.start(file);
    tui.waitFor("RAPID_V0");

    // Make three rapid external modifications
    writeFileSync(file, "RAPID_V1", "utf8");
    writeFileSync(file, "RAPID_V2", "utf8");
    writeFileSync(file, "RAPID_V3_FINAL", "utf8");

    // Wait for the editor to settle
    tui.waitFor("RAPID_V3_FINAL");

    const snap = tui.snapshot();
    expect(snap).toContain("RAPID_V3_FINAL");
    // Should not show any intermediate version
    expect(snap).not.toContain("RAPID_V0");
    expect(snap).not.toContain("RAPID_V1");
    expect(snap).not.toContain("RAPID_V2");
  });
});
