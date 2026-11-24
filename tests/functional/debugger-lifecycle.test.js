import { spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { it, expect } from "vitest";
import * as tui from "./tui.js";

const python = process.env.TTT_TEST_DEBUGPY;
const fixtures = [
  { type: "go", file: "main.go", available: spawnSync("dlv", ["version"]).status === 0,
    source: 'package main\nimport ("fmt"; "time"; "os"; "strconv")\nfunc main() { os.WriteFile("pid", []byte(strconv.Itoa(os.Getpid())), 0600); fmt.Println("TARGET_READY"); for { time.Sleep(time.Millisecond) } }' },
  { type: "node", file: "app.js", available: process.env.TTT_JS_DEBUG_SERVER,
    source: 'require("fs").writeFileSync("pid", String(process.pid)); console.log("TARGET_READY"); setInterval(()=>{},10)' },
  { type: "debugpy", file: "app.py", available: python,
    source: 'import os, time, pathlib\npathlib.Path("pid").write_text(str(os.getpid()))\nprint("TARGET_READY", flush=True)\nwhile True: time.sleep(0.01)' },
];

for (const fixture of fixtures) {
  for (const action of ["stop", "quit"]) {
    it.skipIf(!fixture.available)(`pauses a launched ${fixture.type} program and terminates it on ${action}`, async () => {
      const dir = mkdtempSync(join(tmpdir(), "ttt-debug-cleanup-"));
      const pidfile = join(dir, "pid");
      let exited = false;
      try {
        writeFileSync(join(dir, fixture.file), fixture.source);
        mkdirSync(join(dir, ".vscode"));
        writeFileSync(join(dir, ".vscode/launch.json"), JSON.stringify({ configurations: [{
          name: "Lifecycle", type: fixture.type, request: "launch", program: fixture.file,
          ...(fixture.type === "debugpy" && { python }),
        }] }));
        tui.start(dir, join(dir, fixture.file));
        tui.exec("Start Debugging");
        tui.panel("output");
        tui.waitFor("[debug:Lifecycle] TARGET_READY");
        tui.exec("Debug: Pause");
        tui.waitFor("[Continue]");
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
