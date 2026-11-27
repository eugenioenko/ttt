import assert from "node:assert/strict";
import {execFileSync, spawn} from "node:child_process";
import {once} from "node:events";
import {mkdtempSync, readFileSync, rmSync} from "node:fs";
import {tmpdir} from "node:os";
import {join} from "node:path";
import {test} from "node:test";
import {fileURLToPath} from "node:url";
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

function substitute(value, vars) {
  if (Array.isArray(value)) return value.map(item => substitute(item, vars));
  if (value && typeof value === "object") return Object.fromEntries(
    Object.entries(value).map(([key, item]) => [key, substitute(item, vars)]));
  if (typeof value !== "string") return value;
  if (Object.hasOwn(vars, value)) return vars[value];
  return value.replaceAll("${port}", String(vars["${port}"]));
}

for (const spec of cases) {
  for (const mode of ["PID", "picker", "address", ...(spec.addressAttach?.externalAdapter ? ["external adapter"] : [])]) {
    const byAddress = mode === "address" || mode === "external adapter";
    const cfg = byAddress ? spec.addressAttach : spec.pidAttach;
    if (!cfg) continue;
    test(`[${spec.name}] attach by ${mode}, inspect, step, detach`, {skip: cfg.skip}, async () => {
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
        if (spec.service.build) execFileSync(spec.service.build[0], spec.service.build.slice(1), {cwd: dir});
        const vars = {};
        let target;
        if (!byAddress || cfg.command.includes("${pid}")) {
          target = start(spec.service.command);
          await waitUntil(pulse, "target heartbeat");
          vars["${pid}"] = mode === "picker" ? "${command:pickProcess}" : target.pid;
        }
        async function portFrom(child, pattern) {
          let output = "";
          for (const stream of [child.stdout, child.stderr]) stream.on("data", data => {output += data;});
          return Number(await waitUntil(() => pattern.exec(output)?.[1], "debug server address"));
        }
        if (byAddress) {
          const fixtures = fileURLToPath(new URL("../fixtures/", import.meta.url));
          const server = start(substitute(cfg.command, vars).map(arg => String(arg).replaceAll("${fixtures}/", fixtures)));
          if (cfg.portPattern) vars["${port}"] = await portFrom(server, new RegExp(cfg.portPattern));
        }
        if (cfg.portFile) vars["${port}"] = await waitUntil(() => {
          try { return Number(readFileSync(join(dir, cfg.portFile), "utf8")); }
          catch (e) { if (e.code !== "ENOENT") throw e; }
        }, "target port");
        const args = substitute(cfg.arguments, vars);
        if (mode === "external adapter") args.debugServer = await portFrom(start(["js-debug-adapter", "0", "127.0.0.1"]), /listening at 127\.0\.0\.1:(\d+)/);
        startEditor(dir, file, spec, {name: "Attach", request: "attach", ...args});
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
        for (const [key, value] of Object.entries(spec.service.locals)) tui.waitFor(`│  ${key} = ${value} `, 10000);
        tui.press("f10");
        tui.waitFor(executionMarker(dir, file)(spec.service.nextLine));
        tui.exec("Detach Debugger");
        tui.waitFor("Debug session ended");
        tui.run(45000);
        const first = await waitUntil(pulse, "heartbeat after detach");
        await waitUntil(() => pulse() > first, "target progress after detach");
        if (target) assert.equal(target.exitCode, null);
      } finally {
        for (const child of children.reverse()) {
          if (child.exitCode === null && child.signalCode === null) {
            const done = once(child, "exit");
            child.kill(child.spawnargs[0] === "dlv" ? "SIGINT" : "SIGKILL");
            await done;
          }
        }
        rmSync(dir, {recursive: true, force: true});
      }
    });
  }
}
