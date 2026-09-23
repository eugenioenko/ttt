import { describe, it, expect, afterEach } from "vitest";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("global search", () => {
  it.each([
    ["--exec", "1: run with --exec here"],
    ["-listen", "2: and --listen too"],
  ])("searches for %s as text, not as an rg flag", (query, result) => {
    dir = createTempDir();
    writeFileSync(join(dir, "flags.txt"), "run with --exec here\nand --listen too\n");

    tui.start(dir);
    tui.waitFor("Explore");
    tui.pressChord("ctrl+k", "f");
    tui.type(query);
    tui.waitFor(result);
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s]).not.toContain("search failed");
  });
});
