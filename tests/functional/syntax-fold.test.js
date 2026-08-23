import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
  dir = null;
});

const goContent = `package main

import "fmt"

func main() {
\tx := "hello"
\tfmt.Println(x)
}
`;

describe("syntax highlighting consistency after fold/unfold", () => {
  it("should preserve content after fold and unfold cycle", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "main.go", goContent);

    tui.start(file);
    tui.waitFor("hello");

    // Verify initial content is visible
    const s0 = tui.snapshot();

    // Go to the func main() line and fold it
    tui.press("ctrl+g");
    tui.type("5");
    tui.press("enter");

    tui.pressChord("ctrl+k", "[");

    // Content inside the fold should be hidden
    const s1 = tui.snapshot();

    // Unfold using toggle
    tui.pressChord("ctrl+k", "[");

    // Content should be restored exactly
    const s2 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).toContain("hello");
    expect(snapshots[s0]).toContain("Println");

    expect(snapshots[s1]).not.toContain("hello");

    expect(snapshots[s2]).toContain("hello");
    expect(snapshots[s2]).toContain("Println");
    expect(snapshots[s2]).toContain("package main");
    expect(snapshots[s2]).toContain('import "fmt"');
  });

  it("should survive multiple fold/unfold cycles without content corruption", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "main.go", goContent);

    tui.start(file);
    tui.waitFor("hello");

    // Perform 3 fold-all / unfold-all cycles
    const foldedSnaps = [];
    const unfoldedSnaps = [];
    for (let i = 0; i < 3; i++) {
      tui.pressChord("ctrl+k", "0");
      foldedSnaps.push(tui.snapshot());

      tui.pressChord("ctrl+k", "9");
      unfoldedSnaps.push(tui.snapshot());
    }

    // Final verification snapshot
    const sFinal = tui.snapshot();
    const { snapshots } = tui.run();

    for (const idx of foldedSnaps) {
      expect(snapshots[idx]).not.toContain("hello");
    }
    for (const idx of unfoldedSnaps) {
      expect(snapshots[idx]).toContain("hello");
    }

    // Final verification that all content is intact
    expect(snapshots[sFinal]).toContain("package main");
    expect(snapshots[sFinal]).toContain('import "fmt"');
    expect(snapshots[sFinal]).toContain("func main()");
    expect(snapshots[sFinal]).toContain("hello");
    expect(snapshots[sFinal]).toContain("Println");
  });

  it("should preserve edits made after unfolding", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "main.go", goContent);

    tui.start(file);
    tui.waitFor("hello");

    // Go to the func main() line and fold
    tui.press("ctrl+g");
    tui.type("5");
    tui.press("enter");

    tui.pressChord("ctrl+k", "[");

    const s0 = tui.snapshot();

    // Unfold to edit inside
    tui.pressChord("ctrl+k", "[");

    const s1 = tui.snapshot();

    // Navigate to the "hello" line and add text
    tui.press("ctrl+g");
    tui.type("6");
    tui.press("enter");

    // Go to end of line and add a comment
    tui.press("end");
    tui.type(" // edited");

    const s2 = tui.snapshot();

    // Fold again
    tui.press("ctrl+g");
    tui.type("5");
    tui.press("enter");

    tui.pressChord("ctrl+k", "[");

    const s3 = tui.snapshot();

    // Unfold and verify the edit persisted
    tui.pressChord("ctrl+k", "[");

    const s4 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).not.toContain("hello");

    expect(snapshots[s1]).toContain("hello");

    expect(snapshots[s2]).toContain("edited");

    expect(snapshots[s3]).not.toContain("hello");
    expect(snapshots[s3]).not.toContain("edited");

    expect(snapshots[s4]).toContain("hello");
    expect(snapshots[s4]).toContain("edited");
  });

  it("should show fold indicator when folded and hide it when unfolded", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "main.go", goContent);

    tui.start(file);
    tui.waitFor("hello");

    // Initially no fold indicator
    const s0 = tui.snapshot();

    // Go to func main() line and fold
    tui.press("ctrl+g");
    tui.type("5");
    tui.press("enter");

    tui.pressChord("ctrl+k", "[");

    // Fold indicator should appear
    const s1 = tui.snapshot();

    // Unfold
    tui.pressChord("ctrl+k", "[");

    // Fold indicator should disappear
    const s2 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).not.toContain("⋯");
    expect(snapshots[s1]).toContain("⋯");
    expect(snapshots[s2]).not.toContain("⋯");
  });
});
