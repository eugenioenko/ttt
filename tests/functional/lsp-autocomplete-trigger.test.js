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

describe("lsp autocomplete trigger characters", () => {
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

  testFn("console. triggers completions, typing ( shows signature help", () => {
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
    tui.waitStable();
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

    // Mark again for signature help detection
    mark = logSize();

    // Type 'log(' — should trigger signature help
    tui.type("log(");
    log = waitForLogAfter("lsp signature help response.*label=", mark);
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
    tui.waitStable();
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
