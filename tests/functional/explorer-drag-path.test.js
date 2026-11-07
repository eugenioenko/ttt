import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("dragging an Explorer entry onto the terminal", () => {
  it("inserts the path, quoted when the shell needs it", () => {
    dir = createTempDir();
    createTempFile(dir, "my file.txt", "a");
    createTempFile(dir, "plain.go", "b");

    tui.start(dir);
    tui.setSize(200, 40);
    tui.setEnv({ SHELL: "/bin/sh", PS1: "$ " });
    tui.waitFor("plain.go");
    tui.press("ctrl+t");
    tui.elapse(500);

    // Rows under the workspace root, sorted: "my file.txt" then "plain.go".
    tui.drag(5, 5, 120, 30);
    tui.elapse(200);
    tui.drag(5, 6, 120, 30);
    tui.elapse(300);
    const s0 = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[s0]).toMatch(/'[^']*\/my file\.txt' /);
    expect(snapshots[s0]).toMatch(/ \/\S*\/plain\.go /);
  });

  it("does nothing when dropped outside the terminal", () => {
    dir = createTempDir();
    createTempFile(dir, "stay.txt", "a");

    tui.start(dir);
    tui.setSize(200, 40);
    tui.setEnv({ SHELL: "/bin/sh", PS1: "$ " });
    tui.waitFor("stay.txt");
    tui.press("ctrl+t");
    tui.elapse(500);

    tui.drag(5, 5, 120, 10); // the editor area
    tui.elapse(300);
    const s0 = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[s0]).not.toMatch(/\/stay\.txt /);
  });
});
