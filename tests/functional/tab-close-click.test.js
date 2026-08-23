import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";

afterEach(() => {
  tui.kill();
});

describe("tab close button hit test (#354)", () => {
  it("closes the active tab when its × is clicked in the MoreButton overflow window", () => {
    tui.start();
    // At 83 cols the strip lands in the #354 overflow window; the active tab's
    // close × renders at row 2, col 72.
    tui.setSize(83, 24);

    tui.press("ctrl+n");
    tui.press("ctrl+n");
    tui.press("ctrl+n");
    const before = tui.snapshot();

    tui.click(72, 2);
    const after = tui.snapshot();

    const { snapshots } = tui.run();

    // Sanity: the × really is under the click (guard against geometry drift).
    expect(snapshots[before].split("\n")[2][72]).toBe("x");
    expect(snapshots[before]).toContain("untitled-4");

    // The click must close the active tab.
    expect(snapshots[after]).not.toContain("untitled-4");
  });

  it("does not over-scroll the strip after closing many tabs", () => {
    tui.start();
    tui.setSize(150, 30);

    for (let i = 0; i < 20; i++) tui.press("ctrl+n");
    for (let i = 0; i < 11; i++) tui.press("ctrl+w");

    const s = tui.snapshot();
    const { snapshots } = tui.run();

    // Before the fix the strip was stranded on the last tab (one label visible);
    // it should now show several tabs.
    const tabRow = snapshots[s].split("\n")[2];
    const visible = (tabRow.match(/untitled-\d+/g) || []).length;
    expect(visible).toBeGreaterThan(1);
  });
});
