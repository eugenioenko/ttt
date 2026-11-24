import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { it, expect } from "vitest";
import * as tui from "./tui.js";
import { configureDebugAdapters } from "./debug-adapters.js";

it.skipIf(!process.env.TTT_TEST_DEBUGPY)("uses the selected workspace folder with another folder's dirty editor focused", () => {
  const dir = mkdtempSync(join(tmpdir(), "ttt-debug-folders-"));
  try {
    const names = ["left", "right"];
    for (const name of names) {
      const root = join(dir, name);
      mkdirSync(join(root, ".ttt"), { recursive: true });
      writeFileSync(join(root, "main.py"), `print("running ${name}")\n`);
      writeFileSync(join(root, ".ttt/launch.json"), JSON.stringify({configurations: [{
        type:"debugpy", request:"launch", name, program:"main.py", python:process.env.TTT_TEST_DEBUGPY,
      }]}));
    }
    const workspace = join(dir, "project.ttt");
    writeFileSync(workspace, JSON.stringify({folders:names.map(path => ({path:join(dir,path)}))}));
    const file = join(dir, "left/main.py");
    tui.start("--workspace", workspace, file);
    configureDebugAdapters(dir);
    tui.type("# unsaved");
    tui.exec("Show Debug");
    tui.waitFor(" Run & Debug ");
    tui.press("enter");
    tui.type("right");
    tui.press("enter");
    tui.panel("output");
    tui.waitFor("[debug:right] running right");
    tui.waitFor("Debug session ended");
    tui.run(30000);
    expect(readFileSync(file,"utf8")).toBe('print("running left")\n');
  } finally {
    rmSync(dir,{recursive:true,force:true});
  }
}, 35000);
