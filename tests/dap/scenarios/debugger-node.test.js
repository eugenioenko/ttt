import { cpSync, mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { it, expect } from "vitest";
import * as tui from "../../functional/tui.js";
import { configureDebugAdapters } from "./debug-adapters.js";

it("steps through original TypeScript source in a js-debug child session", () => {
  const dir = mkdtempSync(join(tmpdir(), "ttt-ts-debug-"));
  try {
    const file = join(dir, "app.ts");
    cpSync(new URL("../fixtures/ts-sourcemap/", import.meta.url), dir, { recursive: true });
    mkdirSync(join(dir, ".ttt"));
    writeFileSync(join(dir, ".ttt/launch.json"), JSON.stringify({
      configurations: [{ name: "TS", type: "pwa-node", request: "launch",
        program: "${workspaceFolder}/dist/app.js", outFiles: ["${workspaceFolder}/dist/**/*.js"] }],
    }));
    tui.start(dir, file);
    configureDebugAdapters(dir);
    tui.exec("Show Debug");
    tui.click(32, 6);
    tui.click(32, 10);
    tui.exec("Start Debugging");
    tui.waitFor("local = 42");
    const stopped = tui.snapshot();
    tui.click(6, 10);
    tui.waitFor("answer = 42");
    const expanded = tui.snapshot();
    for (const [action, expected] of [["Copy Key", "answer"], ["Copy Value", "42"]]) {
      tui.rclick(9, 11);
      tui.waitFor(action);
      if (action === "Copy Value") tui.press("down");
      tui.press("enter");
      tui.press("ctrl+p");
      tui.type("copied-");
      tui.press("ctrl+v");
      tui.waitFor("copied-" + expected);
      tui.press("esc");
    }
    tui.exec("Debug: Step Over");
    tui.waitFor("app.ts:4");
    const stepped = tui.snapshot();
    tui.waitFor("this = undefined");
    tui.click(9, 21);
    tui.waitFor("Ln 6, Col 1");
    const caller = tui.snapshot();
    tui.click(29, 30);
    tui.waitFor("Clear All Breakpoints");
    tui.press("enter");
    const cleared = tui.snapshot();
    tui.exec("Debug: Continue");
    tui.panel("output");
    tui.waitFor("Debug session ended");
    const output = tui.snapshot();
    const { snapshots } = tui.run(60000);
    expect(snapshots[stopped]).toContain("app.ts:3");
    expect(snapshots[expanded]).toContain("answer = 42");
    expect(snapshots[stepped]).toContain("Ln 4, Col 1");
    expect(snapshots[caller]).toContain("app.ts");
    expect(snapshots[cleared]).not.toMatch(/⦁ app.ts:[37]/);
    expect(snapshots[output]).toContain("[debug:TS] AFTER_REPORT");
    expect(snapshots[output]).toContain("[debug:TS] TS_DONE 43");
  } finally { rmSync(dir, { recursive: true, force: true }); }
}, 70000);

it("Debug File runs the active JavaScript file with or without a saved Go target", () => {
  const dir = mkdtempSync(join(tmpdir(), "ttt-debug-file-"));
  try {
    const file = join(dir, "test.js");
    writeFileSync(file, 'console.log("JS_FILE_RAN");\n');
    for (const hasTarget of [false, true]) {
      if (hasTarget) {
        mkdirSync(join(dir, ".ttt"));
        writeFileSync(join(dir, ".ttt", "launch.json"), JSON.stringify({
          configurations: [{ name: "Launch Go", type: "go", request: "launch", program: "." }],
        }));
      }
      tui.start(dir, file);
      configureDebugAdapters(dir);
      tui.exec("Show Debug");
      const idle = tui.snapshot();
      tui.exec("Debug File");
      tui.panel("output");
      tui.waitFor("[debug:test.js] JS_FILE_RAN");
      const { snapshots } = tui.run(30000);
      const row = snapshots[idle].split("\n").find(line => line.includes("Debug File"));
      expect(row).toContain(hasTarget ? "Run & Debug" : "Configure");
    }
  } finally { rmSync(dir, { recursive: true, force: true }); }
}, 70000);
