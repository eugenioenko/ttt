import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, readdirSync } from "node:fs";
import { homedir } from "node:os";
import { dirname, join } from "node:path";
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
} else {
  if (option === "--scenarios") assert.equal(names.length, 0, "Scenario tests run as a suite");
  execFileSync("pnpm", ["--dir", join(root, "../functional"), "exec", "vitest", "run", "--reporter=verbose", "--root", root, option === "--run" ? "matrix" : "scenarios"], {
    stdio: "inherit",
    env: {...process.env, DAP_CASES: JSON.stringify(selected.map(c => c.name)), TTT_TEST_DEBUGPY: process.env.TTT_TEST_DEBUGPY || join(tools, "debugpy/bin/python"),
      TTT_JS_DEBUG_SERVER: process.env.TTT_JS_DEBUG_SERVER || join(tools, "js-debug/src/dapDebugServer.js")},
  });
}
