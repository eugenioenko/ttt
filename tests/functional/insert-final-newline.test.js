import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";
import { writeFileSync } from "node:fs";
import { join } from "node:path";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("insertFinalNewline", () => {
  it("should add trailing newline on save by default", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "default.txt", "no newline at end");

    tui.start(file);
    tui.waitFor("no newline at end");

    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    const content = readFile(file);
    expect(content).toBe("no newline at end\n");
  });

  it("should not add trailing newline when setting is false", () => {
    dir = createTempDir();
    const configFile = join(dir, "settings.json");
    writeFileSync(configFile, JSON.stringify({ editor: { insertFinalNewline: false } }));
    const file = createTempFile(dir, "noeol.txt", "no trailing newline");

    tui.start("--config", configFile, file);
    tui.waitFor("no trailing newline");

    tui.press("end");
    tui.type("!");
    tui.waitFor("no trailing newline!");

    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    const content = readFile(file);
    expect(content).toBe("no trailing newline!");
  });

  it("should not add trailing newline to multiline file when setting is false", () => {
    dir = createTempDir();
    const configFile = join(dir, "settings.json");
    writeFileSync(configFile, JSON.stringify({ editor: { insertFinalNewline: false } }));
    const file = createTempFile(dir, "multi.txt", "AAA\nBBB\nCCC");

    tui.start("--config", configFile, file);
    tui.waitFor("AAA");

    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    const content = readFile(file);
    expect(content).toBe("AAA\nBBB\nCCC");
  });

  it("should add trailing newline to multiline file on save", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "multi.txt", "line one\nline two\nline three");

    tui.start(file);
    tui.waitFor("line one");

    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    const content = readFile(file);
    expect(content).toBe("line one\nline two\nline three\n");
  });
});
