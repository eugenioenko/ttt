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
  it("renders optional checked states and keeps callbacks interactive", () => {
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
          { label = "Unchecked mode", command = "unchecked", checked = false },
          { separator = true },
        },
        on_menu = function(command)
          ttt.notify("selected:" .. command)
        end,
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

    tui.click(35, 12);
    const checkedStates = tui.snapshot();
    tui.press("arrow_down");
    tui.press("enter");
    const callback = tui.snapshot();
    tui.click(35, 13);
    const omitted = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[checkedStates]).toContain("│✓ Checked mode");
    expect(snapshots[checkedStates]).toContain("│  Unchecked mode");
    expect(snapshots[callback]).toContain("selected:unchecked");
    expect(snapshots[omitted]).not.toContain("✓");
    expect(snapshots[omitted]).toContain("│ Legacy mode   │");
  });
});
