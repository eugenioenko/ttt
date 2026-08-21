import { describe, it, expect, afterEach } from "vitest";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("plugin menu checked entries", () => {
  it("renders checked entries without shifting menus that omit checked", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello\n");
    const plugin = join(dir, "menu-indicators.lua");
    writeFileSync(
      plugin,
      `local ttt = require("ttt")

ttt.register({
  bottom = {
    title = "Menu Indicators",
    render = function(panel)
      panel:dropdown({
        label = "Checked menu",
        entries = {
          { label = "Checked mode", command = "checked", checked = true },
        },
      })
      panel:dropdown({
        label = "Legacy menu",
        entries = {
          { label = "Legacy mode", command = "legacy" },
        },
      })
    end,
  },
})
`,
      "utf8",
    );

    tui.start("--plugin", plugin, file);
    tui.setSize(80, 20);
    tui.waitStable(300);
    tui.panel("plugin.menu-indicators");
    tui.waitStable();

    // The two dropdowns occupy the first two content rows of the bottom panel.
    tui.click(35, 12);
    const checked = tui.snapshot();
    tui.press("esc");
    tui.click(35, 13);
    const omitted = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[checked]).toContain("│✓ Checked mode");
    expect(snapshots[omitted]).not.toContain("✓");
    expect(snapshots[omitted]).toContain("│ Legacy mode   │");
  });
});
