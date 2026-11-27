import assert from "node:assert/strict";
import {spawn} from "node:child_process";
import {once} from "node:events";
import {mkdtempSync, readFileSync, rmSync} from "node:fs";
import {tmpdir} from "node:os";
import {join} from "node:path";
import {test} from "node:test";
import {setTimeout as delay} from "node:timers/promises";
import * as tui from "../../functional/tui.js";
import {cases, copyTarget, executionMarker, startEditor} from "./harness.js";

async function waitUntil(read, description) {
  const deadline = Date.now() + 10000;
  do {
    const value = read();
    if (value) return value;
    await delay(25);
  } while (Date.now() < deadline);
  throw new Error(`Timed out waiting for ${description}`);
}

function substitute(args, vars) {
  return Object.fromEntries(Object.entries(args).map(([key, value]) => {
    if (Object.hasOwn(vars, value)) return [key, vars[value]];
    if (typeof value !== "string") return [key, value];
    return [key, value.replaceAll("${port}", String(vars["${port}"]))];
  }));
}

for (const spec of cases) {
  for (const mode of ["PID", "picker", "address"]) {
    const cfg = mode === "address" ? spec.addressAttach : spec.pidAttach;
    if (!cfg) continue;
    test(`[${spec.name}] attach by ${mode}, inspect, step, detach`, async () => {
      const dir = mkdtempSync(join(tmpdir(), "ttt-attach-"));
      const children = [];
      const start = argv => {
        const child = spawn(argv[0], argv.slice(1), {cwd: dir});
        children.push(child);
        return child;
      };
      const pulse = () => {
        try { return Number(readFileSync(join(dir, "heartbeat"), "utf8")); }
        catch (error) { if (error.code === "ENOENT") return 0; throw error; }
      };
      try {
        const file = copyTarget(dir, spec.service);
        const target = start(spec.service.command);
        await waitUntil(pulse, "target heartbeat");
        const vars = {"${pid}": mode === "picker" ? "${command:pickProcess}" : target.pid};
        if (mode === "address") {
          const server = start(cfg.command.map(arg => String(vars[arg] ?? arg)));
          let output = "";
          for (const stream of [server.stdout, server.stderr]) stream.on("data", data => {output += data;});
          const pattern = new RegExp(cfg.portPattern);
          vars["${port}"] = await waitUntil(() => pattern.exec(output)?.[1], "debug server address");
        }
        startEditor(dir, file, spec, {name: "Attach", request: "attach", ...substitute(cfg.arguments, vars)});
        for (let i = 1; i < spec.service.line; i++) tui.press("down");
        tui.exec("Toggle Breakpoint");
        tui.exec("Start Debugging");
        if (mode === "picker") {
          tui.waitFor("Select a Process to Debug");
          tui.type(String(target.pid));
          tui.press("enter");
        }
        if (cfg.continueOnAttach) {
          tui.waitFor("⏵   ⏏");
          tui.exec("Debug: Continue");
        }
        for (const [key, value] of Object.entries(spec.service.locals)) tui.waitFor(`│    ${key} = ${value} `, 10000);
        tui.press("f10");
        tui.waitFor(executionMarker(dir, file)(spec.service.nextLine));
        tui.exec("Detach Debugger");
        tui.waitFor("Debug session ended");
        tui.run(45000);
        const first = await waitUntil(pulse, "heartbeat after detach");
        await waitUntil(() => pulse() > first, "target progress after detach");
        assert.equal(target.exitCode, null);
      } finally {
        for (const child of children.reverse()) {
          if (child.exitCode === null && child.signalCode === null) {
            const done = once(child, "exit");
            child.kill("SIGKILL");
            await done;
          }
        }
        rmSync(dir, {recursive: true, force: true});
      }
    });
  }
}
