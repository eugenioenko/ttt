import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { it, expect } from "vitest";
import * as tui from "../../functional/tui.js";
import { cases, copyTarget, executionMarker, startEditor } from "./harness.js";

for (const spec of cases) {
  for (const action of ["stop", "quit"]) {
    const breakpoint = spec.service.stopAtBreakpoint;
    it(`[${spec.name.replace("-", "/")}] ${breakpoint ? "hits a breakpoint" : "pauses a launched program"} and terminates it on ${action}`, async () => {
      const dir = mkdtempSync(join(tmpdir(), "ttt-debug-cleanup-"));
      const pidfile = join(dir, "pid");
      let exited = false;
      try {
        const file = copyTarget(dir, spec.service);
        startEditor(dir, file, spec, {name: "Lifecycle", request: "launch", ...spec.lifecycle});
        if (breakpoint) {
          for (let line = 1; line < spec.service.line; line++) tui.press("down");
          tui.exec("Toggle Breakpoint");
        }
        tui.exec("Start Debugging");
        tui.panel("output");
        tui.waitFor("[debug:Lifecycle] TARGET_READY");
        if (breakpoint) {
          tui.waitFor(executionMarker(dir, file)(spec.service.line));
        } else {
          tui.exec("Debug: Pause");
        }
        tui.waitFor("⏵   ∎");
        if (action === "stop") {
          tui.exec("Stop Debugging");
          tui.waitFor("Debug session ended");
        }
        tui.run(35000);
        const pid = Number(readFileSync(pidfile, "utf8"));
        const deadline = Date.now() + 5000;
        while (Date.now() < deadline) {
          try { process.kill(pid, 0); } catch (err) { if (err.code === "ESRCH") { exited = true; return; } throw err; }
          await delay(25);
        }
        expect.fail(`program ${pid} was still present after ${action}`);
      } finally {
        if (!exited) {
          try {
            const pid = Number(readFileSync(pidfile, "utf8"));
            if (Number.isSafeInteger(pid) && pid > 0) process.kill(pid, "SIGKILL");
          } catch {}
        }
        rmSync(dir, { recursive: true, force: true });
      }
    }, 45000);
  }
}
