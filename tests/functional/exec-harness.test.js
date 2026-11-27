import { spawnSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { resolve, join } from "node:path";
import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

const BINARY = resolve(import.meta.dirname, "../../bin/ttt");
let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
  dir = undefined;
});

describe("exec harness reliability", () => {
  it("waitFor blocks until delayed text is actually visible", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "wait.txt", "waiting\n");
    const pluginFile = join(dir, "delayed.lua");
    writeFileSync(
      pluginFile,
      `local ttt = require("ttt")
ttt.set_timeout(350, function()
  ttt.set_status_item("left", "ready", "ASYNC READY", { priority = 10 })
end)
`
    );

    tui.start("--plugin", pluginFile, file);
    tui.waitFor("ASYNC READY");
    const screen = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[screen]).toContain("ASYNC READY");
  });

  it("opens a relative file and writes a state dump under the run directory", () => {
    dir = createTempDir();
    createTempFile(dir, "sample.txt", "per-run workspace");
    tui.start("sample.txt");
    tui.setCwd(dir);
    tui.waitFor("per-run workspace", 2000);
    tui.dumpState("sample state.json");
    tui.run();
    const state = JSON.parse(readFileSync(join(dir, "sample state.json"), "utf8"));
    expect(state.buffer.path).toBe(join(dir, "sample.txt"));
  });

  it("returns a nonzero exit and stderr for an invalid action", () => {
    dir = createTempDir();
    const result = spawnSync(BINARY, ["--size", "80x24", "--exec", "not-an-action"], {
      encoding: "utf8",
      timeout: 15000,
      env: { ...process.env, TTT_CONFIG_DIR: join(dir, "config") },
    });

    expect(result.status).not.toBe(0);
    expect(result.stderr).toContain("--exec failed");
    expect(result.stderr).toContain('unknown action "not-an-action"');
  });

  it("returns a nonzero exit when wait-for reaches its bound", () => {
    dir = createTempDir();
    const result = spawnSync(
      BINARY,
      ["--size", "80x24", "--exec", `wait-for "NEVER VISIBLE" timeout=40`],
      {
        encoding: "utf8",
        timeout: 15000,
        env: { ...process.env, TTT_CONFIG_DIR: join(dir, "config") },
      }
    );

    expect(result.status).not.toBe(0);
    expect(result.stderr).toContain('screen text "NEVER VISIBLE" not visible');
  });

  it("rejects an overflowing wait-for timeout as invalid input", () => {
    dir = createTempDir();
    const result = spawnSync(
      BINARY,
      ["--size", "40x12", "--exec", "wait-for x timeout=9223372036854775807"],
      {
        encoding: "utf8",
        timeout: 15000,
        env: { ...process.env, TTT_CONFIG_DIR: join(dir, "config") },
      }
    );

    expect(result.status).not.toBe(0);
    expect(result.stderr).toContain('invalid wait-for timeout "9223372036854775807"');
    expect(result.stderr).not.toContain("not visible after -");
  });
});
