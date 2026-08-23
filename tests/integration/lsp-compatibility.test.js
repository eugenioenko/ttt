import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  copyFileSync,
  cpSync,
  existsSync,
  readFileSync,
  statSync,
  unlinkSync,
  writeFileSync,
} from "node:fs";
import { execFileSync } from "node:child_process";
import { request as httpRequest } from "node:http";
import { createServer } from "node:net";
import { dirname, extname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import * as tui from "./tui.js";
import { cleanupDir, createTempDir } from "./helpers.js";

const __dirname = dirname(fileURLToPath(import.meta.url));
const LSP_DIR = resolve(__dirname, "lsp");
const LOG_FILE = resolve(
  process.env.HOME || process.env.USERPROFILE,
  ".config",
  "ttt",
  "ttt.log",
);
const LANGUAGES = ["c", "go", "typescript"];

function sleep(ms) {
  execFileSync("sleep", [String(ms / 1000)]);
}

function allocatePort() {
  return new Promise((resolvePort, reject) => {
    const server = createServer();
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address();
      server.close((err) => (err ? reject(err) : resolvePort(port)));
    });
  });
}

function postExec(port, body) {
  return new Promise((resolveResponse, reject) => {
    const req = httpRequest(
      {
        hostname: "127.0.0.1",
        port,
        path: "/exec",
        method: "POST",
        headers: { "Content-Length": Buffer.byteLength(body) },
      },
      (res) => {
        let responseBody = "";
        res.setEncoding("utf8");
        res.on("data", (chunk) => {
          responseBody += chunk;
        });
        res.once("aborted", () =>
          reject(new Error("response aborted before completion")),
        );
        res.once("end", () =>
          resolveResponse({ status: res.statusCode, body: responseBody }),
        );
      },
    );
    req.setTimeout(3000, () => req.destroy(new Error("request timed out")));
    req.once("error", reject);
    req.end(body);
  });
}

async function previousTab(port, marker) {
  const response = await postExec(port, 'exec "View: Previous Tab"');
  expect(response).toEqual({ status: 200, body: "ok" });
  tui.waitFor(marker);
  tui.waitStable();
}

function logSize() {
  return existsSync(LOG_FILE) ? statSync(LOG_FILE).size : 0;
}

function waitForLogAfter(pattern, afterBytes = 0, timeoutMs = 20000) {
  const re = new RegExp(pattern);
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    if (existsSync(LOG_FILE)) {
      const tail = readFileSync(LOG_FILE, "utf8").substring(afterBytes);
      if (re.test(tail)) return tail;
    }
    sleep(200);
  }
  if (!existsSync(LOG_FILE)) return "";
  return readFileSync(LOG_FILE, "utf8").substring(afterBytes);
}

function navigateTo(pos) {
  const [line, col] = pos.split(":").map(Number);
  tui.press("ctrl+g");
  tui.waitStable();
  tui.type(String(line));
  tui.press("enter");
  tui.waitStable();
  tui.press("home");
  for (let i = 0; i < col; i++) tui.press("arrow_right");
}

async function typeSignature(port, text) {
  const paren = text.lastIndexOf("(");
  if (paren <= 0) {
    tui.type(text);
    return;
  }
  const completionMark = logSize();
  tui.type(text.substring(0, paren));
  const completion = waitForLogAfter("lsp completion response", completionMark);
  expect(completion).toMatch(/lsp completion response/);
  tui.waitStable();
  tui.press("escape");
  tui.waitStable();
  const response = await postExec(port, `type "${text.substring(paren)}"`);
  expect(response).toEqual({ status: 200, body: "ok" });
}

function serverAvailable(settingsPath) {
  const settings = JSON.parse(readFileSync(settingsPath, "utf8"));
  return Object.values(settings.lsp.servers).every((server) => {
    try {
      execFileSync("which", [server.command[0]], { stdio: "ignore" });
      return true;
    } catch {
      return false;
    }
  });
}

function prepareCase(language) {
  const fixtureDir = resolve(LSP_DIR, language);
  const settings = resolve(fixtureDir, "settings.json");
  const spec = JSON.parse(
    readFileSync(resolve(fixtureDir, "spec.json"), "utf8"),
  );
  const dir = createTempDir();
  cpSync(fixtureDir, dir, { recursive: true });
  const sourceName = { c: "test.c", go: "test.go", typescript: "test.ts" }[
    language
  ];
  const source = resolve(dir, sourceName);
  const extension = extname(source);
  const signature = resolve(dir, `signature${extension}`);
  copyFileSync(source, signature);
  writeFileSync(
    signature,
    `${readFileSync(signature, "utf8")}\n// TTT_SIGNATURE_COPY\n`,
    "utf8",
  );

  let completion;
  if (language === "typescript") {
    completion = resolve(dir, "completion.js");
    writeFileSync(completion, "// TypeScript completion smoke\n\n", "utf8");
  } else {
    completion = resolve(dir, `completion${extension}`);
    copyFileSync(source, completion);
    writeFileSync(
      completion,
      `${readFileSync(completion, "utf8")}\n// TTT_COMPLETION_COPY\n`,
      "utf8",
    );
  }

  return { completion, dir, settings, signature, source, spec };
}

describe("pinned real-LSP compatibility", () => {
  let dir;

  beforeEach(() => {
    if (existsSync(LOG_FILE)) unlinkSync(LOG_FILE);
  });

  afterEach(() => {
    tui.kill();
    if (dir) cleanupDir(dir);
    dir = null;
  });

  for (const language of LANGUAGES) {
    const settings = resolve(LSP_DIR, language, "settings.json");
    const testFn = serverAvailable(settings) ? it : it.skip;

    testFn(`${language} completes one server lifecycle`, async () => {
      const prepared = prepareCase(language);
      dir = prepared.dir;
      const { completion, signature, source, spec } = prepared;
      const port = await allocatePort();

      tui.startWithEnv(
        { TTT_LISTEN_PORT: String(port) },
        "--listen",
        "--config",
        prepared.settings,
        completion,
        signature,
        source,
      );
      tui.waitFor(spec.waitFor);
      const initialized = waitForLogAfter("lsp initialized");
      const diagnostics = waitForLogAfter(spec.diagnostic);
      expect(initialized).toContain("lsp initialized");
      expect(diagnostics).toMatch(new RegExp(spec.diagnostic));

      const hoverMark = logSize();
      navigateTo(spec.hover.goto);
      tui.pressChord("ctrl+k", "i");
      const hover = waitForLogAfter(spec.hover.log, hoverMark);
      expect(hover).toMatch(new RegExp(spec.hover.log));

      tui.press("escape");
      const signatureOpenMark = logSize();
      await previousTab(port, "TTT_SIGNATURE_COPY");
      const signatureReady = waitForLogAfter(
        "lsp diagnostics.*path=.*signature",
        signatureOpenMark,
      );
      expect(signatureReady).toMatch(/lsp diagnostics.*path=.*signature/);
      const signatureMark = logSize();
      navigateTo(spec.signatureHelp.goto);
      await typeSignature(port, spec.signatureHelp.type);
      const signatureLog = waitForLogAfter(
        spec.signatureHelp.log,
        signatureMark,
      );
      expect(signatureLog).toMatch(new RegExp(spec.signatureHelp.log));

      tui.press("escape");
      const completionOpenMark = logSize();
      await previousTab(
        port,
        language === "typescript"
          ? "TypeScript completion smoke"
          : "TTT_COMPLETION_COPY",
      );
      const completionReady = waitForLogAfter(
        "lsp diagnostics.*path=.*completion",
        completionOpenMark,
      );
      expect(completionReady).toMatch(/lsp diagnostics.*path=.*completion/);
      const completionMark = logSize();
      if (language === "typescript") {
        navigateTo("2:0");
        tui.type("console.");
        const completionLog = waitForLogAfter(
          "lsp completion response.*count=[1-9]",
          completionMark,
        );
        tui.waitStable();
        const offered = tui.snapshot();
        tui.press("tab");
        tui.waitStable();
        const inserted = tui.snapshot();
        const selected = offered.match(/│ ■ ([A-Za-z]\w*)\b/);

        expect(completionLog).toMatch(/lsp completion request.*triggerChar=\./);
        expect(selected).not.toBeNull();
        expect(inserted).toContain(`console.${selected[1]}`);
        expect(inserted).not.toContain("console...");
      } else {
        navigateTo(spec.completion.goto);
        tui.type(spec.completion.type);
        const completionLog = waitForLogAfter(
          spec.completion.log,
          completionMark,
        );
        expect(completionLog).toMatch(new RegExp(spec.completion.log));
      }
    });
  }
});
