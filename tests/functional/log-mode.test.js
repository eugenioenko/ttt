import { it, expect, afterEach } from "vitest";
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;
afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
  dir = null;
});

it("opens a large log at startup, searches past the viewport, and returns to editing", () => {
  dir = createTempDir();
  const text = "ordinary log entry with some padding\n".repeat(40000) + "unique tail marker\n";
  const file = createTempFile(dir, "server.log", text);
  const config = join(dir, "config");
  mkdirSync(config);
  writeFileSync(join(config, "settings.json"), JSON.stringify({
    editor: { largeFiles: { logModeMinMB: 1 } },
  }));
  tui.start(file);
  tui.setEnv({ TTT_CONFIG_DIR: config });
  tui.waitFor("server.log (log)");
  tui.click(116, 39);
  tui.press("ctrl+end");
  tui.waitFor("unique tail marker");
  const tail = tui.snapshot();
  tui.press("ctrl+home");
  tui.type("SHOULD_NOT_EDIT");
  tui.press("ctrl+f");
  tui.type("unique tail marker");
  tui.waitFor("1/1");
  const found = tui.snapshot();
  tui.press("escape");
  tui.exec("Open in Editor");
  tui.press("enter");
  tui.press("ctrl+home");
  tui.type("edited ");
  tui.exec("Save File");
  const edited = tui.snapshot();
  const { snapshots } = tui.run();
  expect(snapshots[tail]).toContain("following");
  expect(snapshots[found]).toContain("unique tail marker");
  expect(snapshots[found]).toContain("1/1");
  expect(snapshots[edited]).not.toContain("LOG ");
  expect(readFileSync(file, "utf8")).toBe("edited " + text);
});
