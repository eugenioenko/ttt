import { describe, it, expect, afterEach } from "vitest";
import { existsSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("terminal toggle", () => {
  it("should show terminal panel with ctrl+t", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "term.txt", "Editor content");

    tui.start(file);
    tui.waitFor("term.txt");

    tui.press("ctrl+t");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Terminal");
  });

  it("should toggle terminal off and on", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "toggle.txt", "Toggle test");

    tui.start(file);
    tui.waitFor("toggle.txt");

    tui.press("ctrl+t");

    tui.press("ctrl+t");

    const s0 = tui.snapshot();

    tui.press("ctrl+t");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).not.toContain("Terminal  Problems");
    expect(snapshots[s1]).toContain("Terminal");
  });
});

describe("terminal key routing", () => {
  it("should send ctrl+k and the next key to the shell", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "keys.txt", "Editor content");
    const marker = join(dir, "ok");

    tui.setEnv({ SHELL: "/bin/bash" });
    tui.start(file);
    tui.waitFor("keys.txt");

    tui.press("ctrl+t");
    tui.elapse(1000);
    tui.type("echo foo");
    tui.press("ctrl+a");
    tui.press("ctrl+k");
    tui.type(`touch ${marker}`);
    tui.press("enter");
    tui.elapse(1000);
    tui.run();

    // Without ctrl+k reaching the PTY the shell runs `touch <marker>echo foo`.
    expect(existsSync(marker)).toBe(true);
  });
});
