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

describe("toggle line comment", () => {
  it("comments a single line via command palette", () => {
    dir = createTempDir();
    const file = join(dir, "test.go");
    writeFileSync(file, "package main\n\nfunc main() {\n}\n");

    tui.start(file);
    tui.waitFor("package main");

    tui.exec("Toggle Line Comment");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toContain("// package main");
  });

  it("uncomments a commented line", () => {
    dir = createTempDir();
    const file = join(dir, "test.go");
    writeFileSync(file, "// package main\n\nfunc main() {\n}\n");

    tui.start(file);
    tui.waitFor("// package main");

    tui.exec("Toggle Line Comment");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    const firstLine = content.split("\n")[0];
    expect(firstLine).toBe("package main");
  });

  it("comments all lines when selecting all", () => {
    dir = createTempDir();
    const file = join(dir, "test.go");
    writeFileSync(file, "line1\nline2\nline3\n");

    tui.start(file);
    tui.waitFor("line1");

    tui.press("ctrl+a");

    tui.exec("Toggle Line Comment");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toContain("// line1");
    expect(content).toContain("// line2");
    expect(content).toContain("// line3");
  });

  it("uncomments all lines when selecting all commented lines", () => {
    dir = createTempDir();
    const file = join(dir, "test.go");
    writeFileSync(file, "// line1\n// line2\n// line3\n");

    tui.start(file);
    tui.waitFor("// line1");

    tui.press("ctrl+a");

    tui.exec("Toggle Line Comment");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    const lines = content.split("\n");
    expect(lines[0]).toBe("line1");
    expect(lines[1]).toBe("line2");
    expect(lines[2]).toBe("line3");
  });

  it("uses # prefix for Python files", () => {
    dir = createTempDir();
    const file = join(dir, "test.py");
    writeFileSync(file, 'print("hello")\nx = 1\n');

    tui.start(file);
    tui.waitFor("print");

    tui.exec("Toggle Line Comment");

    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).toContain('# print("hello")');
  });
});
