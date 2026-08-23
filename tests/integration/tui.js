import { execFileSync } from "node:child_process";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, "../..");
const BINARY = resolve(ROOT, "bin/ttt");

function run(args, timeout = 10000) {
  return execFileSync("tui-use", args, {
    encoding: "utf8",
    timeout,
  }).trim();
}

export function start(...args) {
  const session = run(["start", "--", BINARY, ...args], 30000);
  return session;
}

export function startWithEnv(env, ...args) {
  const assignments = Object.entries(env).map(([key, value]) => `${key}=${value}`);
  return run(["start", "--", "env", ...assignments, BINARY, ...args], 30000);
}

export function snapshot() {
  return run(["snapshot"]);
}

export function type(text) {
  run(["type", text]);
}

export function press(key) {
  run(["press", key]);
}

export function waitFor(text) {
  run(["wait", "--text", text]);
}

export function waitStable(ms = 100) {
  run(["wait", "--debounce", String(ms)]);
}

export function find(pattern) {
  return run(["find", pattern]);
}

export function pressChord(first, second) {
  press(first);
  if (second.length === 1) {
    type(second);
  } else {
    press(second);
  }
}

export function exec(command) {
  press("ctrl+p");
  waitStable();
  type(command);
  waitStable();
  press("enter");
}

export function kill() {
  try {
    const out = run(["list", "--format", "json"]);
    const { sessions } = JSON.parse(out);
    for (const s of sessions) {
      try {
        run(["use", s.session_id]);
        run(["kill"]);
      } catch {}
    }
  } catch {}
}
