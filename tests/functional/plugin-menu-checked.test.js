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

  it("preserves mixed checked states in sidebar actions and action indexes", () => {
    dir = createTempDir();
    createTempFile(dir, "test.txt", "hello\n");
    const plugin = join(dir, "sidebar-menu-indicators.lua");
    writeFileSync(
      plugin,
      `local ttt = require("ttt")

ttt.register({
  sidebar = {
    title = "State Probe",
    actions = {
      { label = "Omitted", command = "omitted" },
      { label = "Unchecked", command = "unchecked", checked = false },
      { separator = true },
      { label = "Checked", command = "checked", checked = true },
    },
    on_action = function(command)
      ttt.notify("action:" .. command)
    end,
    render = function(panel)
      panel:label("probe")
    end,
  },
})
`,
      "utf8",
    );

    tui.start("--plugin", plugin, dir);
    tui.setSize(80, 20);
    tui.waitStable(300);
    tui.click(27, 2);
    tui.waitStable();
    tui.click(29, 5);
    tui.waitStable();
    tui.click(29, 2);
    tui.click(29, 2);
    tui.waitStable();
    const mixed = tui.snapshot();
    tui.press("arrow_down");
    tui.press("enter");
    const uncheckedCallback = tui.snapshot();
    tui.click(29, 2);
    tui.click(29, 2);
    tui.press("arrow_down");
    tui.press("arrow_down");
    tui.press("enter");
    const checkedCallback = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[mixed]).toContain("│ Omitted");
    expect(snapshots[mixed]).not.toContain("│  Omitted");
    expect(snapshots[mixed]).toContain("│  Unchecked");
    expect(snapshots[mixed]).toContain("│✓ Checked");
    expect(snapshots[mixed]).toContain("───────────────");
    expect(snapshots[uncheckedCallback]).toContain("action:unchecked");
    expect(snapshots[checkedCallback]).toContain("action:checked");
  });

  it("reconciles dropdown checked state across redraw and reopen", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello\n");
    const plugin = join(dir, "dynamic-menu-indicators.lua");
    writeFileSync(
      plugin,
      `local ttt = require("ttt")

local state = "checked"

ttt.register({
  bottom = {
    title = "Dynamic Indicators",
    render = function(panel)
      local entry = { label = "Dynamic mode", command = "cycle" }
      if state ~= "omitted" then
        entry.checked = state == "checked"
      end
      panel:dropdown({
        label = "State " .. state,
        entries = {
          entry,
          { separator = true },
          { label = "Tail action", command = "tail" },
        },
        on_menu = function(command)
          if command == "cycle" then
            if state == "checked" then
              state = "unchecked"
            elseif state == "unchecked" then
              state = "omitted"
            else
              state = "checked"
            end
            panel:redraw()
          end
          ttt.notify("selected:" .. command .. ":" .. state)
        end,
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
    tui.panel("plugin.dynamic-menu-indicators");
    tui.waitStable();

    tui.click(35, 12);
    const checked = tui.snapshot();
    tui.press("enter");
    tui.waitStable();
    tui.click(35, 12);
    const unchecked = tui.snapshot();
    tui.press("enter");
    tui.waitStable();
    tui.click(35, 12);
    const omitted = tui.snapshot();
    tui.press("enter");
    tui.waitStable();
    tui.click(35, 12);
    const checkedAgain = tui.snapshot();
    tui.press("arrow_down");
    tui.press("enter");
    const tailCallback = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[checked]).toContain("State checked");
    expect(snapshots[checked]).toContain("│✓ Dynamic mode");
    expect(snapshots[unchecked]).toContain("State unchecked");
    expect(snapshots[unchecked]).toContain("│  Dynamic mode");
    expect(snapshots[omitted]).toContain("State omitted");
    expect(snapshots[omitted]).toContain("│ Dynamic mode");
    expect(snapshots[omitted]).not.toContain("│  Dynamic mode");
    expect(snapshots[checkedAgain]).toContain("State checked");
    expect(snapshots[checkedAgain]).toContain("│✓ Dynamic mode");
    expect(snapshots[tailCallback]).toContain("selected:tail:checked");
  });
});
