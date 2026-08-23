import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("outline panel", () => {
  it("shows markdown headings and jumps on activate", () => {
    dir = createTempDir();
    const file = createTempFile(
      dir,
      "doc.md",
      "# Intro\n\ntext\n\n## Setup\n\nmore\n\n## Usage\n",
    );

    tui.start(dir, file);
    tui.waitFor("Explore");

    tui.exec("Show Outline");
    tui.waitFor("§ Intro");
    const s0 = tui.snapshot();

    // Navigate Intro -> Setup -> Usage; enter jumps and focuses the editor.
    // Typing a marker proves both the jump target and the focus handoff.
    tui.press("down");
    tui.press("down");
    tui.press("enter");
    tui.type("JUMPED");
    const s1 = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("Outline");
    expect(snapshots[s0]).toContain("§ Intro");
    expect(snapshots[s0]).toContain("§ Setup");
    expect(snapshots[s0]).toContain("§ Usage");
    expect(snapshots[s1]).toContain("JUMPED## Usage");
  });

  it("nests subsections under their parent heading", () => {
    dir = createTempDir();
    const file = createTempFile(
      dir,
      "nested.md",
      "# Top\n\n## Child\n\n### Grandchild\n",
    );

    tui.start(dir, file);
    tui.waitFor("Explore");
    tui.exec("Show Outline");
    tui.waitFor("§ Top");
    const s0 = tui.snapshot();

    const { snapshots } = tui.run();
    const screen = snapshots[s0];
    const topCol = screen.split("\n").find((l) => l.includes("§ Top"))?.indexOf("§");
    const childCol = screen.split("\n").find((l) => l.includes("§ Child"))?.indexOf("§");
    const grandCol = screen.split("\n").find((l) => l.includes("§ Grandchild"))?.indexOf("§");
    expect(topCol).toBeLessThan(childCol);
    expect(childCol).toBeLessThan(grandCol);
  });

  it("shows empty state for files without symbols", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "plain.txt", "just text\n");

    tui.start(dir, file);
    tui.waitFor("Explore");
    tui.exec("Show Outline");
    const s0 = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[s0]).toContain("No symbols");
  });
});
