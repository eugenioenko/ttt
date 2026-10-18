import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir, readFile } from "./helpers.js";
import { join } from "node:path";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("piped stdin", () => {
  it("opens stdin given as - in an untitled tab and saves it to a chosen path", () => {
    dir = createTempDir();
    const out = join(dir, "saved.txt");

    tui.start("-");
    tui.stdin("alpha line\r\nbeta line\r\n");
    tui.waitFor("beta line");
    const snap = tui.snapshot();
    tui.press("ctrl+s");
    tui.elapse(300);
    tui.type(out);
    tui.press("enter");
    tui.elapse(300);

    const { snapshots } = tui.run();

    expect(snapshots[snap]).toContain("untitled");
    expect(readFile(out)).toBe("alpha line\r\nbeta line\r\n");
  });

  it("leaves stdin unread under --exec without -", () => {
    dir = createTempDir();

    tui.start(dir);
    tui.stdin("should not open\n");
    tui.elapse(300);
    const snap = tui.snapshot();

    const { snapshots } = tui.run();

    expect(snapshots[snap]).not.toContain("should not open");
  });
});
