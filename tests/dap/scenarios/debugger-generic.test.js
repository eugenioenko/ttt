import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, it } from "vitest";
import * as tui from "../../functional/tui.js";

it("starts a stdio child and honors a literal argv, cwd, and environment in an integrated terminal", () => {
  const dir = mkdtempSync(join(tmpdir(), "ttt-dap-reverse-"));
  try {
    const config = join(dir, "user-config");
    mkdirSync(config);
    mkdirSync(join(dir, ".ttt"));
    const command = [process.execPath, fileURLToPath(new URL("../fixtures/reverse-adapter.cjs", import.meta.url))];
    writeFileSync(join(config, "settings.json"), JSON.stringify({debug:{adapters:{go:{command,languages:["c"]}}}}));
    writeFileSync(join(dir, ".ttt/launch.json"), JSON.stringify({inputs:[], compounds:[], configurations:[
      {name:"Reverse",type:"go",request:"launch",paths:["${workspaceFolder}"]}
    ]}));
    tui.start(dir);
    tui.setEnv({TTT_CONFIG_DIR:config,DAP_REMOVED:"inherited"});
    tui.exec("Start Debugging");
    tui.waitFor("TERMINAL_COMPLETED");
    tui.panel("output");
    tui.waitFor("CHILD_CONFIGURED");
    tui.exec("Stop Debugging");
    tui.waitFor("Debug session ended");
    tui.run();
    expect(JSON.parse(readFileSync(join(dir,"terminal.json"),"utf8"))).toEqual([dir,"one arg $HOME","set",null]);
  } finally { rmSync(dir,{recursive:true,force:true}); }
});
