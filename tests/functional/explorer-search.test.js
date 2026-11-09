import { afterEach, expect, it } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;
afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

it("delivers debounced explorer results through the running event loop", () => {
  dir = createTempDir();
  createTempFile(dir, "alpha.txt", "alpha contents");
  createTempFile(dir, "beta.txt", "beta contents");
  tui.start(dir);
  tui.exec("Show Explorer");
  tui.press("/");
  tui.type("betaX");
  tui.waitFor("No results");
  tui.press("backspace");
  tui.waitFor("beta.txt");
  const filtered = tui.snapshot();
  tui.press("enter");
  tui.waitFor("beta contents");
  const opened = tui.snapshot();
  tui.exec("Show Explorer");
  tui.press("esc");
  const cleared = tui.snapshot();
  const { snapshots } = tui.run();
  expect(snapshots[filtered]).toContain("❯ beta");
  expect(snapshots[filtered]).not.toContain("alpha.txt");
  expect(snapshots[opened]).toContain("beta contents");
  expect(snapshots[cleared]).toContain("alpha.txt");
  expect(snapshots[cleared]).not.toContain("❯ beta");
});
