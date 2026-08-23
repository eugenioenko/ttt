// Repro tests for confirmed bugs from audit/2026-07-12-ux-bug-audit.md (branch audit/bug-hunt).
// Each test asserts the CORRECT behavior and is declared with `it.fails`,
// so it passes while the bug exists and goes red the moment the bug is
// fixed — at that point remove the `.fails` marker and the audit entry.
//
// Several of these are DATA-LOSS bugs (BUG-028..031). The tests operate
// only inside per-test temp dirs created by createTempDir().
import { describe, it, expect, afterEach } from "vitest";
import { writeFileSync, mkdirSync, existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("BUG-028: command-palette Explorer: Delete lacks the isRoot guard the right-click menu has", () => {
  it.fails("Explorer: Delete with the root selected does not disk-remove the root folder", () => {
    dir = createTempDir();
    mkdirSync(join(dir, "sub"));
    writeFileSync(join(dir, "file.txt"), "hi\n");

    tui.start(dir);
    tui.exec("Show Explorer");
    tui.exec("Explorer: Delete");
    tui.press("right"); // move to "Yes" in confirm dialog
    tui.press("enter");
    tui.run();

    // Buggy: explorerNodePath() falls back to Tree.Selected()==0 (root),
    // and os.RemoveAll wipes the whole workspace.
    expect(existsSync(dir)).toBe(true);
    expect(existsSync(join(dir, "file.txt"))).toBe(true);
  });
});

describe("BUG-029: renaming an open file leaves the tab tracking the old path", () => {
  it.fails("save after rename writes to the renamed file, not the old path", () => {
    dir = createTempDir();
    writeFileSync(join(dir, "root.txt"), "root file");

    tui.start(dir);
    tui.exec("Show Explorer");
    tui.press("arrow_down"); // select root.txt
    tui.press("enter"); // open it
    tui.exec("Explorer: Rename");
    tui.press("ctrl+a");
    tui.type("renamed.txt");
    tui.press("enter");
    tui.press("end");
    tui.type("APPENDED");
    tui.press("ctrl+s");
    tui.run();

    // Correct: the edit lands in renamed.txt and no phantom root.txt is
    // recreated. Buggy: tab keeps path root.txt, so save recreates
    // root.txt with the edit and leaves renamed.txt stale.
    expect(readFileSync(join(dir, "renamed.txt"), "utf8")).toBe(
      "root fileAPPENDED",
    );
    expect(existsSync(join(dir, "root.txt"))).toBe(false);
  });
});

describe("BUG-030: New File / Rename silently clobber an existing file", () => {
  it("New File with a colliding name does not truncate the existing file", () => {
    dir = createTempDir();
    writeFileSync(join(dir, "dup.txt"), "existing content - keep me");

    tui.start(dir);
    tui.exec("Show Explorer");
    tui.exec("Explorer: New File");
    tui.type("dup.txt");
    tui.press("enter");
    tui.run();

    // Fixed: FileOpNewFile now checks for an existing file before writing
    // and shows a status error instead of truncating it.
    expect(readFileSync(join(dir, "dup.txt"), "utf8")).toBe(
      "existing content - keep me",
    );
  });

  it("Rename onto an existing file does not overwrite it", () => {
    dir = createTempDir();
    writeFileSync(join(dir, "aaa.txt"), "aaa content");
    writeFileSync(join(dir, "bbb.txt"), "bbb content - keep me");

    tui.start(dir);
    tui.exec("Show Explorer");
    tui.press("arrow_down"); // select aaa.txt (alphabetically first)
    tui.exec("Explorer: Rename");
    tui.press("ctrl+a");
    tui.type("bbb.txt");
    tui.press("enter");
    tui.run();

    // Fixed: FileOpRename now checks for an existing target before
    // renaming and shows a status error instead of silently replacing it.
    expect(readFileSync(join(dir, "aaa.txt"), "utf8")).toBe("aaa content");
    expect(readFileSync(join(dir, "bbb.txt"), "utf8")).toBe(
      "bbb content - keep me",
    );
  });
});

// BUG-031 (stale tab after external delete) has no functional test: the
// batch harness can't rm mid-session. See audit/2026-07-12-ux-bug-audit.md for the timed
// external-delete --exec repro; an integration (PTY) test is the right
// home for it if one is added.

// BUG-032 (open-from-explorer doesn't focus editor) was REJECTED during
// curation as intentional behavior (FocusOnOpen defaults false — keep
// focus in the explorer for browsing). Test removed; see the audit ledger.
