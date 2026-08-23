import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

// Fullwidth East Asian characters occupy two terminal columns. Everything that
// maps text to columns — drawing, the cursor, clicks, truncation — has to agree
// with that, or the terminal swallows the character after each wide one.
// See https://github.com/eugenioenko/ttt/issues/434
describe("fullwidth CJK rendering", () => {
  it("should render all Korean syllables, not every other one", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "hangul.txt", "가나다라마바사\n");

    tui.start(file);
    tui.waitFor("hangul.txt");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // The issue: fullwidth characters cause the next character to be swallowed.
    // 가나다라마바사 (7 syllables) renders as 가다마사 (only indices 0,2,4,6).
    // All 7 should be visible.
    expect(snapshots[s0]).toContain("가나다라마바사");
  });

  it("should render all Chinese characters, not every other one", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "chinese.txt", "你好世界\n");

    tui.start(file);
    tui.waitFor("chinese.txt");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // 你好世界 (4 chars) should all be visible, not just 你世.
    expect(snapshots[s0]).toContain("你好世界");
  });

  it("should render all Japanese kana, not every other one", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "japanese.txt", "こんにちは\n");

    tui.start(file);
    tui.waitFor("japanese.txt");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // こんにちは (5 chars) should all be visible, not just こには.
    expect(snapshots[s0]).toContain("こんにちは");
  });

  it("should render mixed fullwidth and ASCII without swallowing", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "mixed.txt", "가a나b다\n");

    tui.start(file);
    tui.waitFor("mixed.txt");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // ASCII chars following fullwidth chars should not be swallowed.
    // 가a나b다 should show all 5 runes, not just 가나다.
    expect(snapshots[s0]).toContain("가a나b다");
  });

  it("fullwidth last on line should render correctly (trailing padding ok)", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "trailing.txt", "hello가\n");

    tui.start(file);
    tui.waitFor("trailing.txt");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // A line ending with a fullwidth char — the swallowed cell is trailing
    // padding, so it looks fine. But verify the whole line renders.
    expect(snapshots[s0]).toContain("hello가");
  });

  it("should type fullwidth text into the buffer", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "typing.txt", "\n");

    tui.start(file);
    tui.waitFor("typing.txt");
    tui.type("가나다");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("가나다");
  });
});

describe("fullwidth CJK cursor and columns", () => {
  it("should report a character column, not a screen column", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "hangul.txt", "가나다라\n");

    tui.start(file);
    tui.waitFor("hangul.txt");
    tui.press("end");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // Four runes: the cursor sits at character 5, not column 9.
    expect(snapshots[s0]).toContain("Ln 1, Col 5");
  });

  it("should move one rune per arrow key", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "hangul.txt", "가나다라\n");

    tui.start(file);
    tui.waitFor("hangul.txt");
    tui.press("right");
    tui.press("right");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("Ln 1, Col 3");
  });

  it("should keep the line intact when scrolled horizontally", () => {
    dir = createTempDir();
    // Wider than the editor pane, forcing horizontal scroll.
    const file = createTempFile(dir, "wide.txt", "가".repeat(120) + "END\n");

    tui.start(file);
    tui.setSize(80, 24);
    tui.waitFor("wide.txt");
    tui.press("end");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // Scrolled to the end of the line: the tail must render whole.
    expect(snapshots[s0]).toContain("가가가가");
    expect(snapshots[s0]).toContain("END");
  });

  it("should not let a fullwidth rune bleed over the pane border", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "edge.txt", "가".repeat(200) + "\n");

    tui.start(file);
    tui.setSize(80, 24);
    tui.waitFor("edge.txt");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // Every row of the editor frame keeps its right border: a fullwidth rune
    // drawn in the last column would paint over it.
    const rows = snapshots[s0].split("\n").filter((r) => r.includes("가"));
    expect(rows.length).toBeGreaterThan(0);
    for (const row of rows) {
      expect(row.trimEnd().endsWith("│")).toBe(true);
    }
  });
});

describe("fullwidth CJK outside the editor", () => {
  it("should render a CJK filename in the explorer and tab bar", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "한국어파일.txt", "hello\n");

    tui.start(dir, file);
    tui.waitFor("한국어파일.txt");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // Once in the explorer tree, once in the tab bar — neither may drop runes.
    const matches = snapshots[s0].split("한국어파일.txt").length - 1;
    expect(matches).toBeGreaterThanOrEqual(2);
  });

  it("should render fullwidth text typed into the find bar", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "search.txt", "가나다라\n");

    tui.start(file);
    tui.waitFor("search.txt");
    tui.press("ctrl+f");
    tui.type("가나다");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    // The query renders in full, and matches the buffer text.
    expect(snapshots[s0]).toContain("가나다");
    expect(snapshots[s0]).toContain("1/1");
  });

  it("should render fullwidth text typed into the command palette", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "palette.txt", "hello\n");

    tui.start(file);
    tui.waitFor("palette.txt");
    tui.press("ctrl+p");
    tui.type("한국");

    const s0 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("한국");
  });
});
