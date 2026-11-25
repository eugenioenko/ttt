import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { homedir, tmpdir } from "node:os";
import { basename, dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(fileURLToPath(import.meta.url));
const tools = process.env.DAP_TOOLS || join(process.env.XDG_CACHE_HOME || join(homedir(), ".cache"), "ttt-dap");
process.env.DAP_TOOLS = tools;
process.env.PATH = `${join(tools, "bin")}:${process.env.PATH}`;
const cases = readdirSync(join(root, "cases")).sort().map(file =>
  JSON.parse(readFileSync(join(root, "cases", file), "utf8")));
const [option, ...names] = process.argv.slice(2);
assert(["--install", "--run", "--scenarios"].includes(option), "Usage: node run.mjs --install|--run [case ...] or --scenarios");
for (const name of names) assert(cases.some(c => c.name === name), `Unknown case: ${name}`);
const selected = cases.filter(c => !names.length || names.includes(c.name));

if (option === "--install") {
  mkdirSync(join(tools, "bin"), { recursive: true });
  for (const script of new Set(selected.flatMap(c => c.requires))) {
    execFileSync("bash", [join(root, script)], { stdio: "inherit" });
  }
} else if (option === "--scenarios") {
  assert.equal(names.length, 0, "Scenario tests run as a suite");
  execFileSync("pnpm", ["--dir", join(root, "../functional"), "exec", "vitest", "run", "--reporter=verbose", "--root", root, "scenarios"], {
    stdio: "inherit",
    env: {...process.env, TTT_TEST_DEBUGPY: process.env.TTT_TEST_DEBUGPY || join(tools, "debugpy/bin/python"),
      TTT_JS_DEBUG_SERVER: process.env.TTT_JS_DEBUG_SERVER || join(tools, "js-debug/src/dapDebugServer.js")},
  });
} else {
  for (const spec of selected) {
    const label = `[${spec.name.replace("-", "/")}] ${spec.launch.name}`;
    console.log(`Running ${label}`);
    const dir = mkdtempSync(join(tmpdir(), "ttt-dap-"));
    const workspace = join(dir, "workspace");
    const config = join(dir, "config");
    try {
      cpSync(dirname(join(root, spec.source)), workspace, { recursive: true });
      if (spec.binary) cpSync(join(root, spec.binary), join(workspace, basename(spec.binary)));
      mkdirSync(join(workspace, ".ttt"));
      mkdirSync(config);
      writeFileSync(join(workspace, ".ttt/launch.json"), JSON.stringify({configurations: [spec.launch]}));
      writeFileSync(join(config, "settings.json"), JSON.stringify(spec.settings));
      runCase(spec, workspace, config);
      console.log(`PASS ${label}`);
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  }
}

function runCase(spec, workspace, config) {
  const source = basename(spec.source);
  const commands = [];
  const snapshots = [];
  const lastScreen = join(config, "last-screen.txt");
  const lastState = join(config, "last-state.json");
  const wait = text => commands.push(`screenshot "${lastScreen}"`, `debug "${lastState}"`, `wait-for ${JSON.stringify(text)} timeout=15000`);
  for (const step of spec.steps) {
    assert.equal(Object.keys(step).length, 1);
    if (step.breakpoint) {
      assert(commands.length === 0 && Number.isInteger(step.breakpoint.line) && step.breakpoint.line > 0,
        "Set the initial source breakpoint first");
      for (let line = 1; line < step.breakpoint.line; line++) commands.push("key down");
      commands.push('exec "Toggle Breakpoint"');
    } else if (step.run === "launch") {
      commands.push('exec "Start Debugging"');
    } else if (step.expectStopped) {
      const {line, locals} = step.expectStopped;
      wait("⏵   ∎");
      wait(`  ${source}:${line}`);
      for (const [key, value] of Object.entries(locals)) wait(`│    ${key} = ${value} `);
      const path = join(config, `stop-${snapshots.length}.txt`);
      commands.push(`screenshot "${path}"`);
      snapshots.push({path, line, locals});
    } else if (step.step === "over") {
      commands.push('exec "Debug: Step Over"');
    } else if (step.continue === true) {
      commands.push('exec "Debug: Continue"');
    } else if (step.expectExit) {
      commands.push("panel output");
      if (step.expectExit.output) wait(`[debug:${spec.launch.name}] ${step.expectExit.output}`);
      wait("Debug session ended");
    } else {
      throw Error(`Unknown step: ${JSON.stringify(step)}`);
    }
  }
  commands.push("quit");
  try {
    execFileSync(join(root, "../../bin/ttt"), [
      "--size", "140x40", "--exec-split-on", "\x1f", "--exec", commands.join("\x1f"), workspace, join(workspace, source),
    ], { cwd: workspace, env: {...process.env, TTT_CONFIG_DIR: config}, stdio: "inherit", timeout: 90000 });
  } catch (error) {
    if (existsSync(lastScreen)) console.error(readFileSync(lastScreen, "utf8"));
    if (existsSync(lastState)) console.error(JSON.parse(readFileSync(lastState, "utf8")).output.join("\n"));
    throw error;
  }
  for (const {path, line, locals} of snapshots) {
    const screen = readFileSync(path, "utf8");
    assert.match(screen, new RegExp(`▶\\s+${line}\\s`), `Expected execution marker on line ${line}:\n${screen}`);
    for (const [name, value] of Object.entries(locals)) {
      assert(screen.split("\n").some(row => row.startsWith(`│    ${name} = ${value} `)),
        `Expected ${name} = ${value} in sidebar:\n${screen}`);
    }
  }
}
