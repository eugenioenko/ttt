import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("indent detection", () => {
  it("should detect 2-space indentation in mjs file", () => {
    dir = createTempDir();
    const content = [
      "import { defineConfig } from 'astro/config';",
      "",
      "export default defineConfig({",
      "  site: 'https://example.com',",
      "  integrations: [",
      "    starlight({",
      "      title: 'TTT',",
      "    }),",
      "  ],",
      "});",
    ].join("\n");
    const file = createTempFile(dir, "astro.config.mjs", content);
    const configFile = createTempFile(dir, "settings.json", JSON.stringify({
      lsp: { notifyAvailability: false },
    }));

    tui.start("--config", configFile, file);
    tui.waitFor("astro.config.mjs");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Spaces: 2");
  });

  it("should detect tab indentation in go file", () => {
    dir = createTempDir();
    const content = [
      "package main",
      "",
      "func main() {",
      "\tfmt.Println()",
      "\tif true {",
      "\t\treturn",
      "\t}",
      "}",
    ].join("\n");
    const file = createTempFile(dir, "main.go", content);
    const configFile = createTempFile(dir, "settings.json", JSON.stringify({
      lsp: { notifyAvailability: false },
    }));

    tui.start("--config", configFile, file);
    tui.waitFor("main.go");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Tab Size:");
  });

  it("should respect editorconfig over detection", () => {
    dir = createTempDir();
    createTempFile(dir, ".editorconfig", "root = true\n\n[*]\nindent_style = tab\nindent_size = 4\n");
    const content = [
      "export default {",
      "  foo: 'bar',",
      "  baz: 'qux',",
      "};",
    ].join("\n");
    const file = createTempFile(dir, "config.mjs", content);
    const configFile = createTempFile(dir, "settings.json", JSON.stringify({
      lsp: { notifyAvailability: false },
    }));

    tui.start("--config", configFile, file);
    tui.waitFor("config.mjs");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Tab Size: 4");
  });

  it("should detect 4-space indentation in python file", () => {
    dir = createTempDir();
    const content = [
      "def main():",
      "    print('hello')",
      "    if True:",
      "        return",
    ].join("\n");
    const file = createTempFile(dir, "app.py", content);
    const configFile = createTempFile(dir, "settings.json", JSON.stringify({
      lsp: { notifyAvailability: false },
    }));

    tui.start("--config", configFile, file);
    tui.waitFor("app.py");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Spaces: 4");
  });
});
