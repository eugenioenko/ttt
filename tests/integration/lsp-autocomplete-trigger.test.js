import { describe, it, expect, afterEach, beforeEach } from "vitest";
import { cpSync, unlinkSync, existsSync, readFileSync, writeFileSync, statSync } from "node:fs";
import { execSync } from "node:child_process";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir } from "./helpers.js";

const __dirname = dirname(fileURLToPath(import.meta.url));
const LSP_DIR = resolve(__dirname, "lsp", "typescript");
const LOG_FILE = resolve(
  process.env.HOME || process.env.USERPROFILE,
  ".config",
  "ttt",
  "ttt.log"
);

function sleep(ms) {
  execSync(`sleep ${ms / 1000}`);
}

function logSize() {
  if (!existsSync(LOG_FILE)) return 0;
  return statSync(LOG_FILE).size;
}

function waitForLogAfter(pattern, afterBytes, timeoutMs = 15000) {
  const re = new RegExp(pattern);
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    if (existsSync(LOG_FILE)) {
      const full = readFileSync(LOG_FILE, "utf8");
      const tail = full.substring(afterBytes);
      if (re.test(tail)) return tail;
    }
    sleep(200);
  }
  if (existsSync(LOG_FILE)) {
    return readFileSync(LOG_FILE, "utf8").substring(afterBytes);
  }
  return "";
}

function lspServerAvailable() {
  try {
    execSync("which typescript-language-server", { stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

const available = lspServerAvailable();

// Retry: tsserver can be slow to respond in CI; each retry re-runs setup for a
// fresh server attempt.
describe("lsp autocomplete trigger characters", { retry: 2 }, () => {
  let dir;

  beforeEach(() => {
    if (existsSync(LOG_FILE)) unlinkSync(LOG_FILE);
  });

  afterEach(() => {
    tui.kill();
    if (dir) cleanupDir(dir);
    dir = null;
  });

  const testFn = available ? it : it.skip;

  testFn("console. triggers completions after typing consol", () => {
    dir = createTempDir();
    cpSync(LSP_DIR, dir, { recursive: true });

    const testFile = resolve(dir, "trigger.js");
    writeFileSync(testFile, "// test\n\n", "utf8");
    const configFile = resolve(LSP_DIR, "settings.json");

    tui.start("--config", configFile, testFile);
    tui.waitFor("// test");
    waitForLogAfter("lsp initialized", 0);

    // Move to line 2 and type 'consol' — should trigger completion
    tui.press("ctrl+g");
    tui.waitStable(1000);
    tui.type("2");
    tui.press("enter");
    tui.waitStable();

    tui.type("consol");
    let log = waitForLogAfter("lsp completion response.*count=[1-9]", 0);
    expect(log).toMatch(/lsp completion response.*count=[1-9]/);

    // Mark position so we only look at new log entries
    let mark = logSize();

    // Type '.' — should trigger new completions for console properties
    tui.type(".");
    log = waitForLogAfter("lsp completion response.*count=[1-9]", mark);
    expect(log).toMatch(/lsp completion response.*count=[1-9]/);

    tui.press("escape");
    tui.press("ctrl+q");
    sleep(200);
    tui.press("arrow_right");
    tui.press("enter");
  });

  testFn("typing ( after a function name triggers signature help", () => {
    dir = createTempDir();
    cpSync(LSP_DIR, dir, { recursive: true });

    const testFile = resolve(dir, "sighelp.js");
    writeFileSync(testFile, "console.log\n", "utf8");
    const configFile = resolve(LSP_DIR, "settings.json");

    tui.start("--config", configFile, testFile);
    tui.waitFor("console.log");
    waitForLogAfter("lsp initialized", 0);
    tui.waitStable(2000);

    // Cursor starts on line 1 — move to end of "console.log"
    tui.press("end");
    tui.waitStable();

    // Type '(' — should trigger signature help.
    // Retry if the LSP server isn't ready yet (returns null).
    let log = "";
    for (let attempt = 0; attempt < 3; attempt++) {
      const mark = logSize();
      tui.type("(");
      log = waitForLogAfter("lsp signature help response.*label=", mark, 5000);
      if (/lsp signature help response.*label=/.test(log)) break;
      tui.press("backspace");
      tui.waitStable(1000);
    }
    expect(log).toMatch(/lsp signature help response.*label=/);

    tui.press("escape");
    tui.press("ctrl+q");
    sleep(200);
    tui.press("arrow_right");
    tui.press("enter");
  });

  testFn("string literal dot triggers completions for string methods", () => {
    dir = createTempDir();
    cpSync(LSP_DIR, dir, { recursive: true });

    const testFile = resolve(dir, "trigger2.js");
    writeFileSync(testFile, "// test\n\n", "utf8");
    const configFile = resolve(LSP_DIR, "settings.json");

    tui.start("--config", configFile, testFile);
    tui.waitFor("// test");
    waitForLogAfter("lsp initialized", 0);

    tui.press("ctrl+g");
    tui.waitStable(1000);
    tui.type("2");
    tui.press("enter");
    tui.waitStable();

    // Type '"hello world!".' — dot after string literal triggers completions
    tui.type('"hello world!".');
    let log = waitForLogAfter("lsp completion response.*count=[1-9]", 0);
    expect(log).toMatch(/lsp completion response.*count=[1-9]/);

    tui.press("escape");
    tui.press("ctrl+q");
    sleep(200);
    tui.press("arrow_right");
    tui.press("enter");
  });
});
