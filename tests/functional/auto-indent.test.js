import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("auto indent", () => {
  it("indents a new line after an open brace", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "");

    tui.start(file);

    tui.type("if x {");
    tui.press("enter");
    tui.type("return");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    // The new line inherits no indent but gains one level from the open brace.
    expect(snapshots[s0]).toContain("    return");
  });

  it("dedents when typing a closing brace on a blank line", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "");

    tui.start(file);

    tui.type("if x {");
    tui.press("enter");
    tui.type("return");
    tui.press("enter");
    tui.type("}");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    // The body stays indented, but the closing brace snaps back to column 0.
    expect(snapshots[s0]).toContain("    return");
    expect(snapshots[s0]).not.toContain("    }");
  });
});
