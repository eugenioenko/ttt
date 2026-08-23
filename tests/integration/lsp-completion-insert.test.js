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
describe("real TypeScript completion acceptance", { retry: 2 }, () => {
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

  testFn("negotiates the dot trigger and inserts a vendor completion", () => {
    dir = createTempDir();
    cpSync(LSP_DIR, dir, { recursive: true });
    const file = resolve(dir, "completion.js");
    writeFileSync(file, "// TypeScript completion smoke\n\n", "utf8");

    tui.start("--config", resolve(LSP_DIR, "settings.json"), file);
    tui.waitFor("TypeScript completion smoke");
    waitForLogAfter("lsp initialized", 0, 20000);

    tui.press("ctrl+g");
    tui.waitStable();
    tui.type("2");
    tui.press("enter");
    tui.waitStable();

    const mark = logSize();
    tui.type("console.");
    const log = waitForLogAfter("lsp completion response.*count=[1-9]", mark, 20000);
    tui.waitStable();
    const offered = tui.snapshot();
    tui.press("tab");
    tui.waitStable();
    const inserted = tui.snapshot();
    tui.press("escape");
    tui.press("ctrl+q");
    sleep(200);
    tui.press("arrow_right");
    tui.press("enter");

    expect(log).toMatch(/lsp completion request.*triggerChar=\./);
    expect(log).toMatch(/lsp completion response.*count=[1-9]/);
    const selected = offered.match(/│ ■ ([A-Za-z]\w*)\b/);
    expect(selected).not.toBeNull();
    expect(inserted).toContain(`console.${selected[1]}`);
    expect(inserted).not.toContain("console...");
  });
});
