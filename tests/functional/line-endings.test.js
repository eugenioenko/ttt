import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir } from "./helpers.js";
import { writeFileSync, readFileSync } from "node:fs";
import { join } from "node:path";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("line ending detection and preservation", () => {
  it("shows LF in status bar for LF files", () => {
    dir = createTempDir();
    const file = join(dir, "lf.txt");
    writeFileSync(file, "line1\nline2\nline3\n");

    tui.start(file);
    tui.waitFor("line1");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("LF");
    expect(snapshots[s0]).not.toContain("CRLF");
  });

  it("shows CRLF in status bar for CRLF files", () => {
    dir = createTempDir();
    const file = join(dir, "crlf.txt");
    writeFileSync(file, "line1\r\nline2\r\nline3\r\n");

    tui.start(file);
    tui.waitFor("line1");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("CRLF");
  });

  it("preserves LF endings on save", () => {
    dir = createTempDir();
    const file = join(dir, "lf.txt");
    writeFileSync(file, "hello\nworld");

    tui.start(file);
    tui.waitFor("hello");
    tui.press("end");
    tui.type(" edited");
    tui.waitFor("hello edited");
    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    const raw = readFileSync(file);
    expect(raw.includes(Buffer.from("\r\n"))).toBe(false);
    const content = raw.toString();
    expect(content).toContain("hello edited\nworld");
    expect(content.includes("\r\n")).toBe(false);
  });

  it("switches from LF to CRLF via command palette", () => {
    dir = createTempDir();
    const file = join(dir, "lf.txt");
    writeFileSync(file, "aaa\nbbb");

    tui.start(file);
    tui.waitFor("aaa");

    const s0 = tui.snapshot();

    tui.exec("Change Line Ending");
    tui.type("CRLF");
    tui.press("enter");

    const s1 = tui.snapshot();

    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    expect(snapshots[s0]).not.toContain("CRLF");
    expect(snapshots[s1]).toContain("CRLF");

    const raw = readFileSync(file);
    expect(raw.toString()).toContain("aaa\r\nbbb");
  });

  it("switches from CRLF to LF via command palette", () => {
    dir = createTempDir();
    const file = join(dir, "crlf.txt");
    writeFileSync(file, "aaa\r\nbbb");

    tui.start(file);
    tui.waitFor("aaa");
    tui.waitFor("CRLF");

    tui.exec("Change Line Ending");
    tui.type("LF");
    tui.press("enter");

    const s0 = tui.snapshot();

    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("LF");
    expect(snapshots[s0]).not.toContain("CRLF");

    const raw = readFileSync(file);
    const content = raw.toString();
    expect(content).toContain("aaa\nbbb");
    expect(content.includes("\r\n")).toBe(false);
  });

  it("preserves CRLF endings on save", () => {
    dir = createTempDir();
    const file = join(dir, "crlf.txt");
    writeFileSync(file, "hello\r\nworld");

    tui.start(file);
    tui.waitFor("hello");
    tui.press("end");
    tui.type(" edited");
    tui.waitFor("hello edited");
    tui.press("ctrl+s");

    const { snapshots } = tui.run();

    const raw = readFileSync(file);
    const content = raw.toString();
    expect(content).toContain("hello edited\r\nworld");
    // Verify no mixed line endings — only CRLF, no bare LF
    const withoutCRLF = content.replace(/\r\n/g, "");
    expect(withoutCRLF.includes("\n")).toBe(false);
  });
});
