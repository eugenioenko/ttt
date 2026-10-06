import { describe, it, expect, afterEach } from "vitest";
import { mkdirSync, writeFileSync, readFileSync, existsSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

const grammar = {
  scopeName: "source.foo",
  name: "Foo",
  fileTypes: ["foo"],
  patterns: [{ match: "\\bfoo\\b", name: "keyword.control.foo" }],
};

function setup() {
  dir = createTempDir();
  const ws = join(dir, "ws");
  const pluginDir = join(ws, "plugins", "lang-foo");
  mkdirSync(join(pluginDir, "syntaxes"), { recursive: true });
  writeFileSync(
    join(pluginDir, "plugin.ttt.json"),
    JSON.stringify({
      name: "lang-foo",
      version: "0.1.0",
      grammars: [{ path: "syntaxes/foo.tmLanguage.json", language: "Foo" }],
    }),
  );
  writeFileSync(join(pluginDir, "syntaxes", "foo.tmLanguage.json"), JSON.stringify(grammar));
  const file = createTempFile(ws, "main.foo", "foo bar\n");
  const configDir = join(dir, "config");
  return { ws, file, configDir };
}

describe("grammar plugins", () => {
  it("installs a grammar-only plugin and removes it on uninstall", () => {
    const { ws, file, configDir } = setup();
    const copied = join(configDir, "grammars", "lang-foo", "syntaxes", "foo.tmLanguage.json");

    tui.start(ws, file);
    tui.setEnv({ TTT_CONFIG_DIR: configDir });
    tui.waitFor("Grammar");
    tui.press("a");
    tui.waitFor("LF   Foo");
    tui.run();

    expect(existsSync(copied)).toBe(true);
    const settings = JSON.parse(readFileSync(join(configDir, "settings.json"), "utf8"));
    expect(settings.editor.grammars).toEqual([
      { path: "grammars/lang-foo/syntaxes/foo.tmLanguage.json", language: "Foo", plugin: "lang-foo" },
    ]);

    tui.start(file);
    tui.setEnv({ TTT_CONFIG_DIR: configDir });
    tui.waitFor("LF   Foo");
    tui.exec("Plugins: Uninstall");
    tui.waitFor("lang-foo");
    tui.press("enter");
    tui.waitFor("Remove plugin");
    tui.press("tab");
    tui.press("enter");
    const after = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[after]).not.toContain("LF   Foo");
    expect(existsSync(copied)).toBe(false);
    const remaining = JSON.parse(readFileSync(join(configDir, "settings.json"), "utf8"));
    expect(remaining.editor.grammars).toBeUndefined();
  });
});
