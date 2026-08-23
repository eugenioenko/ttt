import { afterEach, describe, expect, it } from "vitest";
import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import * as tui from "./tui.js";
import { cleanupDir, createTempDir, createTempFile } from "./helpers.js";

const __dirname = dirname(fileURLToPath(import.meta.url));
const FAKE_SERVER = resolve(__dirname, "fixtures/fake-lsp.mjs");
const INITIAL_TEXT = "// fake protocol\n";
const LIFECYCLE_METHODS = new Set([
  "initialize",
  "initialized",
  "textDocument/didOpen",
  "textDocument/didChange",
]);

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
  dir = null;
});

function startCase(name) {
  dir = createTempDir();
  const file = createTempFile(dir, `${name}.js`, INITIAL_TEXT);
  const trace = join(dir, "fake-lsp-trace.jsonl");
  const config = join(dir, "settings.json");
  writeFileSync(
    config,
    `${JSON.stringify(
      {
        lsp: {
          servers: {
            javascript: {
              command: [process.execPath, FAKE_SERVER, trace],
              languages: { ".js": "javascript" },
            },
          },
        },
        autocomplete: {
          enabled: true,
          autoSuggest: true,
          debounce: 100,
          signatureHelp: true,
        },
      },
      null,
      2,
    )}\n`,
    "utf8",
  );

  tui.start("--config", config, dir, file);
  tui.waitFor("// fake protocol");
  tui.panel("problems");
  tui.waitFor("FAKE_LSP_READY");
  tui.exec("View: Focus Editor");
  tui.press("ctrl+g");
  tui.waitStable();
  tui.type("2");
  tui.press("enter");
  tui.waitStable();

  return { file, trace };
}

function readTrace(path) {
  return readFileSync(path, "utf8")
    .trim()
    .split("\n")
    .filter(Boolean)
    .map((line) => JSON.parse(line));
}

function clientMessages(trace) {
  return trace
    .filter((entry) => entry.direction === "client->server")
    .map((entry) => entry.message);
}

function messagesFor(trace, method) {
  return clientMessages(trace).filter((message) => message.method === method);
}

function expectedInitialize(root) {
  return {
    processId: expect.any(Number),
    rootUri: `file://${root}`,
    capabilities: {
      textDocument: {
        completion: {
          completionItem: {
            snippetSupport: false,
            resolveSupport: { properties: ["additionalTextEdits"] },
          },
        },
        publishDiagnostics: {},
        documentSymbol: { hierarchicalDocumentSymbolSupport: true },
      },
    },
  };
}

function expectedDocumentTexts(typed) {
  let suffix = "";
  return [...typed].map((character) => {
    suffix += character;
    return `${INITIAL_TEXT}${suffix}`;
  });
}

function assertProtocol(trace, file, expectedTexts) {
  const messages = clientMessages(trace);
  const lifecycle = messages.filter((message) => LIFECYCLE_METHODS.has(message.method));
  const changes = messagesFor(trace, "textDocument/didChange");
  const uri = `file://${file}`;

  expect(trace.map((entry) => entry.sequence)).toEqual(
    Array.from({ length: trace.length }, (_, index) => index + 1),
  );
  expect(lifecycle.map((message) => message.method)).toEqual([
    "initialize",
    "initialized",
    "textDocument/didOpen",
    ...Array.from({ length: expectedTexts.length }, () => "textDocument/didChange"),
  ]);
  expect(messagesFor(trace, "initialize")).toEqual([
    {
      jsonrpc: "2.0",
      id: expect.any(Number),
      method: "initialize",
      params: expectedInitialize(dir),
    },
  ]);
  expect(messagesFor(trace, "initialized")).toEqual([
    { jsonrpc: "2.0", method: "initialized", params: {} },
  ]);
  expect(messagesFor(trace, "textDocument/didOpen")).toEqual([
    {
      jsonrpc: "2.0",
      method: "textDocument/didOpen",
      params: {
        textDocument: {
          uri,
          languageId: "javascript",
          version: 1,
          text: INITIAL_TEXT,
        },
      },
    },
  ]);
  expect(changes.map((message) => message.params.textDocument)).toEqual(
    expectedTexts.map((_, index) => ({ uri, version: index + 2 })),
  );
  expect(changes.map((message) => message.params.contentChanges)).toEqual(
    expectedTexts.map((text) => [{ text }]),
  );
}

describe("deterministic fake LSP protocol", () => {
  it("records invoked and dot-trigger completions after ordered full-document changes", () => {
    const { file, trace: tracePath } = startCase("completion-triggers");

    tui.type("consol");
    tui.waitFor("console");
    const invoked = tui.snapshot();
    tui.type(".");
    tui.waitFor("log");
    const triggered = tui.snapshot();

    const { snapshots } = tui.run();
    const trace = readTrace(tracePath);
    const completions = messagesFor(trace, "textDocument/completion");

    expect(snapshots[invoked]).toContain("console");
    expect(snapshots[triggered]).toContain("log");
    assertProtocol(trace, file, expectedDocumentTexts("consol."));
    expect(completions).toEqual([
      {
        jsonrpc: "2.0",
        id: expect.any(Number),
        method: "textDocument/completion",
        params: {
          textDocument: { uri: `file://${file}` },
          position: { line: 1, character: 6 },
          context: { triggerKind: 1 },
        },
      },
      {
        jsonrpc: "2.0",
        id: expect.any(Number),
        method: "textDocument/completion",
        params: {
          textDocument: { uri: `file://${file}` },
          position: { line: 1, character: 7 },
          context: { triggerKind: 2, triggerCharacter: "." },
        },
      },
    ]);
  });

  it("sends signature help at the exact parenthesis position and renders its label", () => {
    const { file, trace: tracePath } = startCase("signature-trigger");

    tui.type("console.log(");
    tui.waitFor("fakeSignature(value: string)");
    const visible = tui.snapshot();

    const { snapshots } = tui.run();
    const trace = readTrace(tracePath);

    expect(snapshots[visible]).toContain("fakeSignature(value: string)");
    assertProtocol(trace, file, expectedDocumentTexts("console.log("));
    expect(messagesFor(trace, "textDocument/signatureHelp")).toEqual([
      {
        jsonrpc: "2.0",
        id: expect.any(Number),
        method: "textDocument/signatureHelp",
        params: {
          textDocument: { uri: `file://${file}` },
          position: { line: 1, character: 12 },
        },
      },
    ]);
  });

  it("uses dot-trigger context for a string literal completion", () => {
    const { file, trace: tracePath } = startCase("string-dot-trigger");
    const typed = '"hello world!".';

    tui.type(typed);
    tui.waitFor("toUpperCase");
    const visible = tui.snapshot();

    const { snapshots } = tui.run();
    const trace = readTrace(tracePath);

    expect(snapshots[visible]).toContain("toUpperCase");
    assertProtocol(trace, file, expectedDocumentTexts(typed));
    expect(messagesFor(trace, "textDocument/completion")).toEqual([
      {
        jsonrpc: "2.0",
        id: expect.any(Number),
        method: "textDocument/completion",
        params: {
          textDocument: { uri: `file://${file}` },
          position: { line: 1, character: 15 },
          context: { triggerKind: 2, triggerCharacter: "." },
        },
      },
    ]);
  });

  it("accepts a dot completion without duplicating the trigger character", () => {
    const { file, trace: tracePath } = startCase("completion-insert");

    tui.type("console.");
    tui.waitFor("log");
    tui.press("tab");
    tui.waitFor("console.log");
    const inserted = tui.snapshot();

    const { snapshots } = tui.run();
    const trace = readTrace(tracePath);

    expect(snapshots[inserted]).toContain("console.log");
    expect(snapshots[inserted]).not.toContain("console..");
    assertProtocol(trace, file, [
      ...expectedDocumentTexts("console."),
      `${INITIAL_TEXT}console.log`,
    ]);
    expect(messagesFor(trace, "textDocument/completion")).toEqual([
      {
        jsonrpc: "2.0",
        id: expect.any(Number),
        method: "textDocument/completion",
        params: {
          textDocument: { uri: `file://${file}` },
          position: { line: 1, character: 8 },
          context: { triggerKind: 2, triggerCharacter: "." },
        },
      },
      {
        jsonrpc: "2.0",
        id: expect.any(Number),
        method: "textDocument/completion",
        params: {
          textDocument: { uri: `file://${file}` },
          position: { line: 1, character: 11 },
          context: { triggerKind: 1 },
        },
      },
    ]);
    expect(messagesFor(trace, "completionItem/resolve")).toEqual([
      {
        jsonrpc: "2.0",
        id: expect.any(Number),
        method: "completionItem/resolve",
        params: { label: "log", kind: 2, insertText: "log" },
      },
    ]);
  });
});
