import assert from "node:assert/strict";
import {mkdtempSync, readFileSync, rmSync} from "node:fs";
import {tmpdir} from "node:os";
import {join} from "node:path";
import {test} from "node:test";
import {setTimeout as delay} from "node:timers/promises";
import * as tui from "../../functional/tui.js";
import {cases, copyTarget, executionMarker, startEditor} from "./harness.js";

for (const spec of cases.filter(spec => spec.lifecycle)) {
  for (const action of ["stop", "quit"]) {
    test(`[${spec.name}] ${action} terminates the launched target`, async () => {
      const dir = mkdtempSync(join(tmpdir(), "ttt-dap-lifecycle-"));
      const pidFile = join(dir, "pid");
      let exited = false;
      try {
        const file = copyTarget(dir, spec.service);
        startEditor(dir, file, spec, {request: "launch", name: "Lifecycle", ...spec.lifecycle});
        for (let i = 1; i < spec.service.line; i++) tui.press("down");
        tui.exec("Toggle Breakpoint");
        tui.exec("Start Debugging");
        tui.waitFor(executionMarker(dir, file)(spec.service.line), 90000);
        if (action === "stop") {
          tui.exec("Stop Debugging");
          tui.waitFor("Debug session ended");
        }
        tui.run(120000);
        const pid = Number(readFileSync(pidFile, "utf8"));
        assert.ok(Number.isSafeInteger(pid) && pid > 0);
        const deadline = Date.now() + 5000;
        while (Date.now() < deadline) {
          try { process.kill(pid, 0); }
          catch (error) { if (error.code === "ESRCH") { exited = true; return; } throw error; }
          await delay(25);
        }
        assert.fail(`target ${pid} survived ${action}`);
      } finally {
        if (!exited) {
          try {
            const pid = Number(readFileSync(pidFile, "utf8"));
            if (Number.isSafeInteger(pid) && pid > 0) process.kill(pid, "SIGKILL");
          } catch (error) {
            if (!["ENOENT", "ESRCH"].includes(error.code)) throw error;
          }
        }
        rmSync(dir, {recursive: true, force: true});
      }
    });
  }
}
