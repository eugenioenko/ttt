import { describe, it, expect, afterEach, beforeEach } from "vitest";
import {
  cpSync,
  readFileSync,
  readdirSync,
  unlinkSync,
  existsSync,
} from "node:fs";
import { execFileSync } from "node:child_process";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import * as tui from "./tui.js";
import { createTempDir, cleanupDir } from "./helpers.js";

const __dirname = dirname(fileURLToPath(import.meta.url));
const LSP_DIR = resolve(__dirname, "lsp");
const LOG_FILE = resolve(
  process.env.HOME || process.env.USERPROFILE,
  ".config",
  "ttt",
  "ttt.log"
);

function sleep(ms) {
  execFileSync("sleep", [String(ms / 1000)]);
}

function waitForLog(pattern, timeoutMs = 10000) {
  const re = new RegExp(pattern);
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    if (existsSync(LOG_FILE)) {
      const log = readFileSync(LOG_FILE, "utf8");
      if (re.test(log)) return log;
    }
    sleep(200);
  }
  return existsSync(LOG_FILE) ? readFileSync(LOG_FILE, "utf8") : "";
}

function navigateTo(pos) {
  const [line, col] = pos.split(":").map(Number);
  tui.press("ctrl+g");
  tui.waitStable();
  tui.type(String(line));
  tui.press("enter");
  tui.waitStable();
  tui.press("home");
  for (let i = 0; i < col; i++) {
    tui.press("arrow_right");
  }
}

function lspServerAvailable(fixtureDir) {
  const settings = JSON.parse(
    readFileSync(resolve(fixtureDir, "settings.json"), "utf8")
  );
  for (const server of Object.values(settings.lsp.servers)) {
    try {
      execFileSync("which", [server.command[0]], { stdio: "ignore" });
    } catch {
      return false;
    }
  }
  return true;
}

const SKIP_LANGUAGES = ["svelte"];

const languages = readdirSync(LSP_DIR, { withFileTypes: true })
  .filter((d) => d.isDirectory())
  .filter((d) => !SKIP_LANGUAGES.includes(d.name))
  .filter((d) => existsSync(resolve(LSP_DIR, d.name, "spec.json")))
  .map((d) => d.name);

function prepareFixture(fixtureDir) {
  const dir = createTempDir();
  cpSync(fixtureDir, dir, { recursive: true });
  const files = readdirSync(dir).filter(
    (file) => !["spec.json", "settings.json", "install.sh"].includes(file)
  );
  return {
    configFile: resolve(fixtureDir, "settings.json"),
    dir,
    testFile: resolve(
      dir,
      files.find((file) => file.startsWith("test."))
    ),
  };
}

describe("lsp", { retry: 2 }, () => {
  let dir;

  beforeEach(() => {
    if (existsSync(LOG_FILE)) unlinkSync(LOG_FILE);
  });

  afterEach(() => {
    tui.kill();
    if (dir) cleanupDir(dir);
    dir = null;
  });

  for (const lang of languages) {
    const fixtureDir = resolve(LSP_DIR, lang);
    const spec = JSON.parse(
      readFileSync(resolve(fixtureDir, "spec.json"), "utf8")
    );
    const available = lspServerAvailable(fixtureDir);

    describe(lang, () => {
      const testFn = available ? it : it.skip;

      testFn("diagnostics", () => {
        const prepared = prepareFixture(fixtureDir);
        dir = prepared.dir;
        const { configFile, testFile } = prepared;

        tui.start("--config", configFile, testFile);
        tui.waitFor(spec.waitFor);
        waitForLog("lsp initialized", 20000);

        const log = waitForLog(spec.diagnostic, 20000);
        tui.press("ctrl+q");

        expect(log).toContain("lsp starting server");
        expect(log).toMatch(new RegExp(spec.diagnostic));
      });

      if (spec.hover) {
        testFn("hover", () => {
          const prepared = prepareFixture(fixtureDir);
          dir = prepared.dir;
          const { configFile, testFile } = prepared;

          tui.start("--config", configFile, testFile);
          tui.waitFor(spec.waitFor);
          waitForLog("lsp initialized", 20000);
          waitForLog(spec.diagnostic, 20000);

          navigateTo(spec.hover.goto);
          tui.waitStable();
          tui.pressChord("ctrl+k", "i");

          const log = waitForLog(spec.hover.log, 20000);
          tui.press("ctrl+q");

          expect(log).toMatch(new RegExp(spec.hover.log));
        });
      }

      if (spec.completion) {
        testFn("completion", () => {
          const prepared = prepareFixture(fixtureDir);
          dir = prepared.dir;
          const { configFile, testFile } = prepared;

          tui.start("--config", configFile, testFile);
          tui.waitFor(spec.waitFor);
          waitForLog("lsp initialized", 20000);
          waitForLog(spec.diagnostic, 20000);

          navigateTo(spec.completion.goto);
          tui.waitStable();
          tui.type(spec.completion.type);

          const log = waitForLog(spec.completion.log, 20000);
          tui.press("escape");
          tui.press("ctrl+q");
          sleep(200);
          tui.press("arrow_right");
          tui.press("enter");

          expect(log).toMatch(new RegExp(spec.completion.log));
        });
      }

      if (spec.signatureHelp) {
        testFn("signature help", () => {
          const prepared = prepareFixture(fixtureDir);
          dir = prepared.dir;
          const { configFile, testFile } = prepared;

          tui.start("--config", configFile, testFile);
          tui.waitFor(spec.waitFor);
          waitForLog("lsp initialized", 20000);
          waitForLog(spec.diagnostic, 20000);

          navigateTo(spec.signatureHelp.goto);
          tui.waitStable();

          // Type text that triggers signature help, splitting before the
          // trigger character so autocomplete from '.' doesn't race with input.
          const text = spec.signatureHelp.type;
          const parenIdx = text.lastIndexOf("(");
          if (parenIdx > 0) {
            tui.type(text.substring(0, parenIdx));
            tui.waitStable();
            tui.press("escape");
            tui.waitStable();
            tui.type(text.substring(parenIdx));
          } else {
            tui.type(text);
          }

          const log = waitForLog(spec.signatureHelp.log, 20000);
          tui.press("escape");
          tui.press("ctrl+q");
          sleep(200);
          tui.press("arrow_right");
          tui.press("enter");

          expect(log).toMatch(new RegExp(spec.signatureHelp.log));
        });
      }
    });
  }
});
