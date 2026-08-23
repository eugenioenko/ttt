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

func alpha() {
\tsecretAlpha := "hidden"
\tfmt.Println(secretAlpha)
}

func beta() {
\tsecretBeta := "also hidden"
\treturn
}
`;

describe("search interaction with code folding", () => {
  it("should expand the second collapsed fold when search matches inside it", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "main.go", goContent);

    tui.start(file);
    tui.waitFor("secretAlpha");

    // Fold all functions
    tui.pressChord("ctrl+k", "0");

    // Verify both folds are collapsed
    const s0 = tui.snapshot();

    // Search for text in the second fold
    tui.press("ctrl+f");
    tui.type("secretBeta");
    tui.press("enter");
    tui.press("escape");

    // The second fold should have expanded to reveal the match
    const s1 = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[s0]).not.toContain("secretAlpha");
    expect(snapshots[s0]).not.toContain("secretBeta");
    expect(snapshots[s1]).toContain("secretBeta");
  });
});
