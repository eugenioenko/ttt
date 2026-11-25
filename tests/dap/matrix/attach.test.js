import { spawn, execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { once } from "node:events";
import { createConnection } from "node:net";
import { it, expect } from "vitest";
import * as tui from "../../functional/tui.js";
import { cases, copyTarget, executionMarker, startEditor } from "./harness.js";

function readNumber(dir, file) {
  try { return Number(readFileSync(join(dir, file), "utf8")); }
  catch (error) { if (error.code === "ENOENT") return 0; throw error; }
}

function substitute(value, vars) {
  if (Array.isArray(value)) return value.map(item => substitute(item, vars));
  if (value && typeof value === "object") return Object.fromEntries(
    Object.entries(value).map(([key, item]) => [key, substitute(item, vars)]));
  if (typeof value !== "string") return value;
  if (Object.hasOwn(vars, value)) return vars[value];
  return value.replace(/\$\{port\}/g, () => String(vars["${port}"]));
}

async function checkAttach(spec, mode, externalAdapter) {
  const dir = mkdtempSync(join(tmpdir(), "ttt-attach-"));
  const children = [];
  const start = (argv, signal = "SIGTERM") => {
    const child = spawn(argv[0], argv.slice(1), {cwd: dir});
    children.push({child, signal});
    return child;
  };
  const portFromLog = async (child, pattern) => {
    let log = "";
    for (const stream of [child.stdout, child.stderr]) stream.on("data", chunk => { log += chunk; });
    const port = () => Number(log.match(pattern)?.[1]);
    await expect.poll(port, {timeout: 10000}).toBeGreaterThan(0);
    return port();
  };
  const pulse = () => readNumber(dir, "heartbeat");
  try {
    const service = spec.service;
    const file = copyTarget(dir, service);
    if (service.build) execFileSync(service.build[0], service.build.slice(1), {cwd: dir});
    const byAddress = mode === "address";
    const cfg = byAddress ? spec.addressAttach : spec.pidAttach;
    const vars = {};
    let pid;
    if (!byAddress || cfg.command.includes("${pid}")) {
      pid = start(service.command).pid;
      await expect.poll(pulse, {timeout: 10000}).toBeGreaterThan(0);
      vars["${pid}"] = mode === "picker" ? "${command:pickProcess}" : pid;
    }
    if (byAddress) {
      const server = start(substitute(cfg.command, vars).map(String), "SIGINT");
      if (cfg.portPattern) vars["${port}"] = await portFromLog(server, new RegExp(cfg.portPattern));
    }
    if (cfg.portFile) {
      const readPort = () => readNumber(dir, cfg.portFile);
      await expect.poll(readPort, {timeout: 10000}).toBeGreaterThan(0);
      vars["${port}"] = readPort();
    }
    const args = substitute(cfg.arguments, vars);
    if (externalAdapter) {
      const adapter = start([process.execPath, process.env.TTT_JS_DEBUG_SERVER, "0", "127.0.0.1"]);
      args.debugServer = await portFromLog(adapter, /listening at 127\.0\.0\.1:(\d+)/);
    }
    startEditor(dir, file, spec, {name: "Attach", request: "attach", ...args});
    for (let line = 1; line < service.line; line++) tui.press("down");
    tui.exec("Toggle Breakpoint");
    tui.exec("Start Debugging");
    if (mode === "picker") {
      tui.waitFor("Select a Process to Debug");
      tui.type(String(pid));
      tui.press("enter");
    }
    if (cfg.continueOnAttach) {
      tui.waitFor("⏵   ⏏");
      tui.exec("Debug: Continue");
    }
    for (const [key, value] of Object.entries(service.locals)) tui.waitFor(`│    ${key} = ${value} `);
    tui.exec("Debug: Step Over");
    tui.waitFor(executionMarker(dir, file)(service.nextLine));
    tui.exec("Detach Debugger");
    tui.waitFor("Debug session ended");
    tui.run(60000);
    if (externalAdapter) {
      const probe = createConnection(args.debugServer, "127.0.0.1");
      try { await once(probe, "connect"); } finally { probe.destroy(); }
    }
    const first = pulse();
    await expect.poll(pulse, {timeout: 5000}).toBeGreaterThan(first);
  } finally {
    for (const {child, signal} of children.reverse()) {
      if (child.exitCode === null && child.signalCode === null) {
        const done = once(child, "exit");
        child.kill(signal);
        await done;
      }
    }
    rmSync(dir, {recursive: true, force: true});
  }
}

for (const spec of cases) {
  const label = `[${spec.name.replace("-", "/")}]`;
  for (const mode of ["PID", "picker", "address"]) {
    const cfg = mode === "address" ? spec.addressAttach : spec.pidAttach;
    const reason = cfg.skip || (mode === "picker" && cfg.picker.skip);
    const title = `${label} attach by ${mode}, step and detach`;
    it.skipIf(reason)(reason ? `${title} (${reason})` : title, () => checkAttach(spec, mode, false), 75000);
  }
  if (spec.addressAttach.externalAdapter) {
    it(`${label} uses separate adapter and debuggee addresses`, () => checkAttach(spec, "address", true), 75000);
  }
}
