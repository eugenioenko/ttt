import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { resolve, dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { tmpdir } from "node:os";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, "../..");
const BINARY = resolve(ROOT, "bin/ttt");

const KEY_MAP = {
  arrow_left: "left",
  arrow_right: "right",
  arrow_up: "up",
  arrow_down: "down",
  page_up: "pgup",
  page_down: "pgdn",
};

let commands = [];
let args = [];
let snapCount = 0;
let tmpDir = "";
let size = "120x40";
let extraEnv = {};

export function start(...startArgs) {
  commands = [];
  snapCount = 0;
  size = "120x40";
  tmpDir = mkdtempSync(join(tmpdir(), "ttt-bb-"));
  args = [];
  extraEnv = {};
  for (const a of startArgs) {
    args.push(a);
  }
}

export function setEnv(vars) {
  extraEnv = { ...extraEnv, ...vars };
}

// Override the terminal size for this run (default 120x40). Reset by start().
export function setSize(w, h) {
  size = `${w}x${h}`;
}

// Simulate a mouse click at screen coordinates (col x, row y).
export function click(x, y) {
  commands.push(`click ${x} ${y}`);
}

export function drag(x1, y1, x2, y2) {
  commands.push(`drag ${x1} ${y1} ${x2} ${y2}`);
}

// Simulate a right-click (opens context menus) at screen coordinates.
export function rclick(x, y) {
  commands.push(`rclick ${x} ${y}`);
}

export function type(text) {
  let start = 0;
  while (start < text.length && text[start] === " ") {
    commands.push("key space");
    start++;
  }
  let end = text.length;
  while (end > start && text[end - 1] === " ") {
    end--;
  }
  if (start < end) {
    commands.push(`type ${text.slice(start, end)}`);
  }
  for (let k = end; k < text.length; k++) {
    commands.push("key space");
  }
}

export function press(key) {
  const mapped = KEY_MAP[key] || key;
  commands.push(`key ${mapped}`);
}

export function pressChord(first, second) {
  const a = KEY_MAP[first] || first;
  const b = KEY_MAP[second] || second;
  commands.push(`key ${a} ${b}`);
}

export function exec(command) {
  commands.push(`exec "${command}"`);
}

export function paste(text) {
  commands.push(`paste ${text}`);
}

export function copy() {
  commands.push("copy");
}

export function panel(id) {
  commands.push(`panel ${id}`);
}

export function wait(ms = 200) {
  commands.push(`wait ${ms}`);
}

export function waitFor(text) {
  commands.push(`wait-for ${JSON.stringify(String(text))}`);
}

export function waitStable(ms = 200) {
  commands.push(`wait ${ms}`);
}

export function snapshot() {
  const idx = snapCount++;
  const path = join(tmpDir, `snap-${idx}.txt`);
  commands.push(`screenshot ${path}`);
  return idx;
}

// ASCII unit separator: cannot appear in a key name or typed text, so tests are
// free to send a literal ";" (a Vim motion, among other things) without it
// being mistaken for a command boundary.
const SEP = "\x1f";

export function run(timeout = 15000) {
  commands.push("quit");
  const script = commands.join(SEP);
  let runError;

  try {
    execFileSync(BINARY, ["--size", size, "--exec-split-on", SEP, "--exec", script, ...args], {
      encoding: "utf8",
      timeout,
      stdio: "pipe",
      // Isolate from the real ~/.config/ttt — settings toggles persist and race across test files.
      env: { ...process.env, TTT_CONFIG_DIR: join(tmpDir, "config"), ...extraEnv },
    });
  } catch (err) {
    runError = err;
  }

  const snapshots = [];
  for (let i = 0; i < snapCount; i++) {
    try {
      snapshots.push(readFileSync(join(tmpDir, `snap-${i}.txt`), "utf8"));
    } catch {
      snapshots.push("");
    }
  }

  cleanup();
  if (runError) throw runError;
  return { snapshots };
}

export function kill() {
  // no-op: compatibility with old afterEach
}

function cleanup() {
  if (tmpDir) {
    try {
      rmSync(tmpDir, { recursive: true, force: true });
    } catch {}
    tmpDir = "";
  }
}
