// Repro tests for confirmed bugs from audit/2026-07-12-ux-bug-audit.md (branch audit/bug-hunt).
// Each test asserts the CORRECT behavior and is declared with `it.fails`,
// so it passes while the bug exists and goes red the moment the bug is
// fixed — at that point remove the `.fails` marker and the audit entry.
import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("BUG-036: status bar text invisible at width <= 50", () => {
  it.fails("status bar renders at width 50 (a realistic split-pane width)", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "sb.txt", "hello world\n");

    tui.start(file);
    tui.setSize(50, 20);
    tui.waitFor("hello");
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    // Correct: the status bar shows "Ln 1, Col 1 ..." at 50 cols as it
    // does at 51+. Buggy: at <=50 the editor box's bottom border renders
    // over the status-bar row and the text disappears entirely.
    expect(snapshots[s]).toContain("Ln 1, Col 1");
  });
});

describe("BUG-039: Discard button vanishes from the quit-confirm dialog at narrow widths", () => {
  it.fails("unsaved-changes dialog keeps Discard reachable at 26 cols", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "dsc.txt", "hello world\n");

    tui.start(file);
    tui.setSize(26, 15);
    tui.waitFor("hello");
    tui.type("X"); // make it dirty
    tui.press("ctrl+w"); // close tab -> unsaved-changes confirm dialog
    const s = tui.snapshot();
    const { snapshots } = tui.run();

    // Correct: all three actions (Discard/Cancel/Save) stay visible when
    // the dialog is narrow (grow/wrap/truncate gracefully). Buggy: the
    // Discard label shrinks then disappears entirely at <=26 cols while
    // Save is never truncated — silently removing an action.
    expect(snapshots[s]).toContain("Discard");
  });
});
