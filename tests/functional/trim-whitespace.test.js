import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir, readFile } from "./helpers.js";
import { writeFileSync } from "node:fs";
import { join } from "node:path";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("trim trailing whitespace on save", () => {
  it("trims trailing whitespace when editorconfig enables it", () => {
    dir = createTempDir();
    writeFileSync(join(dir, ".editorconfig"), [
      "root = true",
      "",
      "[*]",
      "trim_trailing_whitespace = true",
    ].join("\n"));
    const file = join(dir, "code.txt");
    writeFileSync(file, "hello   \nworld\t\t\nclean\n");

    tui.start(file);
    tui.waitFor("hello");
    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("hello\nworld\nclean\n");
  });

  it("trims trailing whitespace when --config enables it", () => {
    dir = createTempDir();
    const configFile = join(dir, "settings.json");
    writeFileSync(configFile, JSON.stringify({ editor: { trimTrailingWhitespace: true } }));
    const file = join(dir, "code.txt");
    writeFileSync(file, "foo   \nbar\t\nclean\n");

    tui.start("--config", configFile, file);
    tui.waitFor("foo");
    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("foo\nbar\nclean\n");
  });

  it("preserves trailing whitespace without the setting", () => {
    dir = createTempDir();
    const file = join(dir, "code.txt");
    writeFileSync(file, "hello   \nworld\t\t\n");

    tui.start(file);
    tui.waitFor("hello");
    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toBe("hello   \nworld\t\t\n");
  });
});
