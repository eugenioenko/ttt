import assert from "node:assert/strict";
import {mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync} from "node:fs";
import {tmpdir} from "node:os";
import {join} from "node:path";
import {test} from "node:test";
import {fileURLToPath} from "node:url";
import * as tui from "../../functional/tui.js";

for (const request of ["launch", "attach"]) {
  for (const childRequest of [undefined, request === "launch" ? "attach" : "launch"]) {
    const target = childRequest ?? request;
    test(`[dap] integrated terminal argv, PID, and cleanup on ${request}${childRequest ? " with " + childRequest + " child" : ""}`, () => {
      const dir = mkdtempSync(join(tmpdir(), "ttt-dap-terminal-"));
      try {
        const config = join(dir, "settings");
        const cwd = join(dir, "target dir");
        mkdirSync(cwd);
        mkdirSync(config);
        mkdirSync(join(dir, ".ttt"));
        const command = [process.execPath, fileURLToPath(new URL("../fixtures/terminal-adapter.cjs", import.meta.url))];
        writeFileSync(join(config, "settings.json"), JSON.stringify({debug: {adapters: {fixture: {command}}}}));
        writeFileSync(join(dir, ".ttt", "launch.json"), JSON.stringify({configurations: [
          {name: "Terminal", type: "fixture", request, childRequest, cwd: "${workspaceFolder}/target dir"}
        ]}));
        tui.start(dir);
        tui.setEnv({TTT_CONFIG_DIR: config, DAP_REMOVED: "inherited"});
        tui.exec("Start Debugging");
        tui.waitFor("TERMINAL_READY", 10000);
        tui.exec(request === "launch" ? "Stop Debugging" : "Detach Debugger");
        tui.waitFor("Debug session ended");
        const state = join(dir, "ended.json");
        tui.dumpState(state);
        tui.run(20000);
        const terminals = JSON.parse(readFileSync(state, "utf8")).terminals ?? [];
        assert.deepEqual(terminals.map(t => t.label), target === "launch" ? [] : ["Debug target"]);
        const pid = Number(readFileSync(join(cwd, "adapter-pid"), "utf8"));
        assert.ok(pid > 0);
        assert.deepEqual(JSON.parse(readFileSync(join(cwd, "target.json"), "utf8")), [pid, cwd, "one arg $HOME", "set", null, target === "launch" ? "dumb" : null]);
        assert.equal(JSON.parse(readFileSync(join(cwd, "target-disconnect"), "utf8")), target === "launch");
        if (childRequest) assert.equal(JSON.parse(readFileSync(join(cwd, "root-disconnect"), "utf8")), request === "launch");
        if (target === "launch") assert.throws(() => process.kill(pid, 0), {code: "ESRCH"});
      } finally {
        rmSync(dir, {recursive: true, force: true});
      }
    });
  }
}
