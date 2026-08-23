import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("go to matching bracket", () => {
  it("should jump to matching bracket with ctrl+k m", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.go", "func main() {\n\tx()\n}\n");
    const configFile = createTempFile(dir, "settings.json", JSON.stringify({
      lsp: { notifyAvailability: false },
    }));

    tui.start("--config", configFile, file);
    tui.waitFor("main");

    // Go to end of line 1 where '{' is the last character
    tui.press("end");

    // Move one left to land on '{'
    tui.press("arrow_left");

    // Now jump to matching bracket
    tui.pressChord("ctrl+k", "m");

    // The status bar should show we're on line 3 (the } line)
    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Ln 3");
  });
});
