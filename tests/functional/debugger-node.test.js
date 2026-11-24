import { execFileSync } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { it, expect } from "vitest";
import * as tui from "./tui.js";

it.skipIf(!process.env.TTT_JS_DEBUG_SERVER)("steps through original TypeScript source in a js-debug child session", () => {
  const dir = mkdtempSync(join(tmpdir(), "ttt-ts-debug-"));
  try {
    const file = join(dir, "app.ts");
    writeFileSync(file, [
      "function report(arg: number) {", "  const local = arg + 1;",
      '  console.log("TS_RESULT", local);', '  console.log("TS_DONE", local + 1);',
      "}", "report(41);", "",
    ].join("\n"));
    execFileSync("tsc", ["--target", "es2020", "--sourceMap", "--outDir", "dist", "app.ts"], { cwd: dir });
    mkdirSync(join(dir, ".ttt"));
    writeFileSync(join(dir, ".ttt/launch.json"), JSON.stringify({
      configurations: [{ name: "TS", type: "pwa-node", request: "launch",
        program: "${workspaceFolder}/dist/app.js", outFiles: ["${workspaceFolder}/dist/**/*.js"] }],
    }));
    tui.start(dir, file);
    tui.click(37, 6);
    tui.exec("Start Debugging");
    tui.waitFor("local = 42");
    const stopped = tui.snapshot();
    tui.exec("Debug: Step Over");
    tui.waitFor("app.ts:4");
    const stepped = tui.snapshot();
    tui.waitFor("this = undefined");
    tui.click(9, 13);
    tui.waitFor("Ln 6, Col 1");
    const caller = tui.snapshot();
    tui.exec("Debug: Continue");
    tui.panel("output");
    tui.waitFor("Debug session ended");
    const output = tui.snapshot();
    const { snapshots } = tui.run(60000);
    expect(snapshots[stopped]).toContain("app.ts:3");
    expect(snapshots[stepped]).toContain("Ln 4, Col 1");
    expect(snapshots[caller]).toContain("app.ts");
    expect(snapshots[output]).toContain("[debug:TS] TS_DONE 43");
  } finally { rmSync(dir, { recursive: true, force: true }); }
}, 70000);
