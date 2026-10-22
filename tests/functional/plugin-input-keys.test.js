import { describe, it, expect, afterEach } from "vitest";
import { writeFileSync, readFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

const typist = `local ttt = require("ttt")
ttt.register({
  commands = {
    { id = "typist.run", title = "Typist Run", handler = function()
      ttt.type("hello\\nwörld\\r\\nend")
      ttt.key("ctrl+s")
    end },
  },
})
`;

describe("plugin key input", () => {
  it("types text and presses keys in order", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "notes.txt", "");
    const pluginPath = join(dir, "typist.lua");
    writeFileSync(pluginPath, typist);

    tui.start(file, "--plugin", pluginPath);
    tui.exec("Typist Run");
    tui.waitFor("end");
    const typed = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[typed]).toMatch(/1\s+hello/);
    expect(snapshots[typed]).toMatch(/2\s+wörld/);
    expect(snapshots[typed]).toMatch(/3\s+end/);
    expect(readFileSync(file, "utf8")).toBe("hello\nwörld\nend\n");
  });
});
