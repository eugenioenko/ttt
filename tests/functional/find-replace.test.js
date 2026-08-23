import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir, readFile } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
  dir = null;
});

describe("find and replace", () => {
  it("should find text and show match count", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "find.txt", "foo bar foo baz foo");

    tui.start(file);
    tui.waitFor("foo bar foo baz foo");

    tui.press("ctrl+f");

    tui.type("foo");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("1/3");
  });

  it("should navigate between find matches", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "nav.txt", "apple banana apple cherry apple");

    tui.start(file);
    tui.waitFor("apple banana");

    tui.press("ctrl+f");

    tui.type("apple");

    const s0 = tui.snapshot();

    tui.press("enter");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("1/3");
    expect(snapshots[s1]).toContain("2/3");
  });

  it("should open find and replace dialog", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "replace.txt", "hello world hello");

    tui.start(file);
    tui.waitFor("hello world hello");

    tui.press("ctrl+r");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Replace");
  });

  it("should replace single occurrence", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "single.txt", "old value old again");

    tui.start(file);
    tui.waitFor("old value old again");

    tui.press("ctrl+r");

    tui.type("old");

    // Tab to replace input
    tui.press("tab");
    tui.type("new");

    // Enter on replace row replaces current match
    tui.press("enter");

    // Close replace bar and save
    tui.press("escape");
    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    // First occurrence replaced, second still present
    expect(content).toContain("new");
    expect(content).toContain("old");
  });

  it("should replace all occurrences by replacing each match", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "replaceall.txt", "cat dog cat bird cat");

    tui.start(file);
    tui.waitFor("cat dog cat bird cat");

    tui.press("ctrl+r");

    tui.type("cat");

    const s0 = tui.snapshot();

    // Tab to replace input
    tui.press("tab");
    tui.type("fish");

    // Replace each occurrence (3 matches)
    tui.press("enter");
    tui.press("enter");
    tui.press("enter");

    // Close replace bar and save
    tui.press("escape");
    tui.press("ctrl+s");

    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("1/3");

    const content = readFile(file);
    expect(content).not.toContain("cat");
    expect(content).toContain("fish dog fish bird fish");
  });

  it("should replace with empty string to delete matches", () => {
    dir = createTempDir();
    const file = createTempFile(
      dir,
      "delete.txt",
      "ZZ keep ZZ stay ZZ end"
    );

    tui.start(file);
    tui.waitFor("ZZ keep ZZ stay ZZ end");

    tui.press("ctrl+r");

    tui.type("ZZ");

    // Tab to replace field but leave it empty (empty replacement = deletion)
    tui.press("tab");

    // Replace each occurrence (3 matches)
    tui.press("enter");
    tui.press("enter");
    tui.press("enter");

    // Close replace bar and save
    tui.press("escape");
    tui.press("ctrl+s");

    tui.run();

    const content = readFile(file);
    expect(content).not.toContain("ZZ");
    expect(content).toContain("keep");
    expect(content).toContain("stay");
    expect(content).toContain("end");
  });

  it("should do nothing when search has no matches", () => {
    dir = createTempDir();
    const original = "alpha beta gamma delta";
    const file = createTempFile(dir, "nomatch.txt", original);

    tui.start(file);
    tui.waitFor("alpha beta gamma");

    tui.press("ctrl+r");

    tui.type("zzzznotfound");

    const s0 = tui.snapshot();

    // Tab to replace, type replacement, try to replace
    tui.press("tab");
    tui.type("replaced");
    tui.press("enter");

    // Close and save
    tui.press("escape");
    tui.press("ctrl+s");

    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("0/0");

    const content = readFile(file).trimEnd();
    expect(content).toBe(original);
  });

  it("should replace all on multiline content", () => {
    dir = createTempDir();
    const lines = [
      "line one with foo here",
      "line two with foo there",
      "line three no match",
      "line four with foo again",
    ].join("\n");
    const file = createTempFile(dir, "multi.txt", lines);

    tui.start(file);
    tui.waitFor("line one");

    tui.press("ctrl+r");

    tui.type("foo");

    const s0 = tui.snapshot();

    // Tab to replace input
    tui.press("tab");
    tui.type("bar");

    // Replace each occurrence (3 matches across lines)
    tui.press("enter");
    tui.press("enter");
    tui.press("enter");

    // Close replace bar and save
    tui.press("escape");
    tui.press("ctrl+s");

    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("1/3");

    const content = readFile(file);
    expect(content).not.toContain("foo");
    expect(content).toContain("line one with bar here");
    expect(content).toContain("line two with bar there");
    expect(content).toContain("line three no match");
    expect(content).toContain("line four with bar again");
  });

  it("should show zero match count after all replaced", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "count.txt", "aaa bbb aaa ccc aaa");

    tui.start(file);
    tui.waitFor("aaa bbb aaa ccc aaa");

    tui.press("ctrl+r");

    tui.type("aaa");

    const s0 = tui.snapshot();

    // Tab to replace input
    tui.press("tab");
    tui.type("xxx");

    // Replace all three
    tui.press("enter");
    tui.press("enter");
    tui.press("enter");

    // After replacing all, match count should be 0/0
    // Tab back to search row to see match count
    tui.press("tab");

    const s1 = tui.snapshot();
    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("1/3");
    expect(snapshots[s1]).toContain("0/0");
  });
});
