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

function startCase(name, serverArgs = [], debounce = 100) {
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
              command: [process.execPath, FAKE_SERVER, trace, ...serverArgs],
              languages: { ".js": "javascript" },
            },
          },
        },
        autocomplete: {
          enabled: true,
          autoSuggest: true,
          debounce,
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
  tui.type("2");
  tui.press("enter");

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

function traceEntriesFor(trace, direction, method) {
  return trace.filter(
    (entry) => entry.direction === direction && entry.message.method === method,
  );
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
        diagnostic: { dynamicRegistration: false, relatedDocumentSupport: false },
      },
      workspace: { diagnostics: { refreshSupport: true } },
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

function assertRequestAfterDocument(trace, request, text) {
  const requestEntry = trace.find(
    (entry) =>
      entry.direction === "client->server" && entry.message.id === request.id,
  );
  const precedingChanges = traceEntriesFor(
    trace.slice(0, requestEntry.sequence - 1),
    "client->server",
    "textDocument/didChange",
  );

  expect(precedingChanges.at(-1).message.params.contentChanges).toEqual([{ text }]);
  expect(precedingChanges.at(-1).sequence).toBeLessThan(requestEntry.sequence);
}

function assertGracefulShutdown(trace) {
  const suffix = trace.slice(-3);
  const shutdownId = suffix[0].message.id;

  expect(suffix).toEqual([
    {
      sequence: trace.length - 2,
      direction: "client->server",
      message: { jsonrpc: "2.0", id: shutdownId, method: "shutdown" },
    },
    {
      sequence: trace.length - 1,
      direction: "server->client",
      message: { jsonrpc: "2.0", id: shutdownId, result: null },
    },
    {
      sequence: trace.length,
      direction: "client->server",
      message: { jsonrpc: "2.0", method: "exit" },
    },
  ]);
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
  assertGracefulShutdown(trace);
}

describe("deterministic fake LSP protocol", () => {
  it("refreshes outline symbols on unchanged save and panel return", () => {
    const { trace } = startCase("outline-refresh", ["--symbols"]);
    tui.exec("Show Outline");
    tui.waitFor("FAKE_SYMBOL_1");
    tui.exec("Save File");
    tui.waitFor("FAKE_SYMBOL_2");
    tui.exec("Show Explorer");
    tui.exec("Show Outline");
    tui.waitFor("FAKE_SYMBOL_3");
    const after = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[after]).toContain("FAKE_SYMBOL_3");
    expect(messagesFor(readTrace(trace), "textDocument/documentSymbol")).toHaveLength(3);
  });

  it("applies a rename answered with documentChanges", () => {
    const { file, trace: tracePath } = startCase("rename-document-changes");
    tui.exec("Rename Symbol");
    tui.type("renamed");
    tui.press("enter");
    tui.waitFor("// renamed protocol");
    const after = tui.snapshot();
    const { snapshots } = tui.run();
    const renames = messagesFor(readTrace(tracePath), "textDocument/rename");

    expect(snapshots[after]).toContain("Renamed across 1 file(s)");
    expect(renames).toHaveLength(1);
    expect(renames[0].params).toEqual({
      textDocument: { uri: `file://${file}` },
      position: { line: 1, character: 0 },
      newName: "renamed",
    });
  });

  it("drops a completion reply that arrives after the cursor left the word", () => {
    startCase("completion-late", ["--late=textDocument/completion"]);
    tui.type("consol");
    tui.waitFor("FAKE_LATE_PENDING");
    // Line start has no prefix, so a stale reply would show unfiltered there.
    tui.press("up");
    tui.press("home");
    tui.waitFor("FAKE_LATE_SENT");
    tui.elapse(300);
    const after = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[after]).toContain("consol");
    expect(snapshots[after]).not.toContain("console");
  });

  it("skips a pending auto-suggest once the cursor leaves the word being typed", () => {
    const { trace: tracePath } = startCase("autosuggest-moved", [], 500);
    tui.type("consol");
    tui.press("up");
    tui.elapse(800);
    const after = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[after]).not.toContain("console");
    expect(messagesFor(readTrace(tracePath), "textDocument/completion")).toEqual([]);
  });

  it("drops a signature help reply that arrives after the cursor left the call", () => {
    startCase("signature-late", ["--late=textDocument/signatureHelp"]);
    tui.type("fn(");
    tui.waitFor("FAKE_LATE_PENDING");
    tui.press("left");
    tui.press("left");
    tui.waitFor("FAKE_LATE_SENT");
    tui.elapse(300);
    const after = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[after]).not.toContain("fakeSignature");
  });

  it("cancels hover on Escape, ignores a late reply, and allows a new hover", () => {
    const { file, trace: tracePath } = startCase("hover-cancel", ["--hold-hover"]);
    tui.exec("Show Hover");
    tui.waitFor("FAKE_HOVER_PENDING");
    tui.press("esc");
    tui.waitFor("FAKE_HOVER_CANCELLED");
    const cancelled = tui.snapshot();
    tui.exec("Show Hover");
    tui.waitFor("FAKE_HOVER_INFO");
    const visible = tui.snapshot();
    tui.press("esc");
    const dismissed = tui.snapshot();
    const { snapshots } = tui.run();
    const trace = readTrace(tracePath);
    const hovers = messagesFor(trace, "textDocument/hover");

    expect(snapshots[cancelled]).not.toContain("FAKE_STALE_HOVER");
    expect(snapshots[visible]).toContain("FAKE_HOVER_INFO");
    expect(snapshots[dismissed]).not.toContain("FAKE_HOVER_INFO");
    expect(hovers).toHaveLength(2);
    expect(hovers[0].params).toEqual({
      textDocument: { uri: `file://${file}` },
      position: { line: 1, character: 0 },
    });
    expect(messagesFor(trace, "$/cancelRequest")).toEqual([
      { jsonrpc: "2.0", method: "$/cancelRequest", params: { id: hovers[0].id } },
    ]);
    assertGracefulShutdown(trace);
  });

  it("pulls diagnostics from a pull-only server on open and after edits", () => {
    const { file, trace: tracePath } = startCase("pull-diagnostics", ["--pull"]);

    tui.type("bad");
    tui.waitFor("FAKE_PULL_BAD");
    const added = tui.snapshot();
    tui.type(" clean");
    tui.waitFor("No problems detected");
    const cleared = tui.snapshot();

    const { snapshots } = tui.run();
    const trace = readTrace(tracePath);
    const pulls = messagesFor(trace, "textDocument/diagnostic");

    expect(snapshots[added]).toContain("FAKE_LSP_READY");
    expect(snapshots[cleared]).not.toContain("FAKE_PULL_BAD");
    expect(traceEntriesFor(trace, "server->client", "textDocument/publishDiagnostics")).toEqual([]);
    for (const pull of pulls) {
      expect(pull.params).toEqual({
        textDocument: { uri: `file://${file}` },
        identifier: "fake",
      });
    }
    assertRequestAfterDocument(trace, pulls.at(-1), `${INITIAL_TEXT}bad clean`);
    expect(pulls.length).toBeLessThan("bad clean".length);
  });

  it("keeps pushed and pulled diagnostics from the same server side by side", () => {
    startCase("push-and-pull", ["--pull", "--push"]);

    tui.type("bad");
    tui.waitFor("FAKE_PULL_BAD");
    const both = tui.snapshot();

    const { snapshots } = tui.run();

    expect(snapshots[both]).toContain("FAKE_PUSHED");
    expect(snapshots[both]).toContain("FAKE_LSP_READY");
  });

  it("resends a pull the server cancelled with retriggerRequest", () => {
    const { trace: tracePath } = startCase("pull-retrigger", ["--pull", "--retrigger-first"]);

    tui.run();
    const trace = readTrace(tracePath);
    const cancelled = trace.find(
      (entry) => entry.direction === "server->client" && entry.message.error?.code === -32802,
    );
    const pulls = traceEntriesFor(trace, "client->server", "textDocument/diagnostic");

    expect(cancelled).toBeDefined();
    expect(pulls.at(-1).sequence).toBeGreaterThan(cancelled.sequence);
  });

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
    assertRequestAfterDocument(trace, completions[0], `${INITIAL_TEXT}consol`);
    assertRequestAfterDocument(trace, completions[1], `${INITIAL_TEXT}consol.`);
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
    const signatures = messagesFor(trace, "textDocument/signatureHelp");
    expect(signatures).toEqual([
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
    assertRequestAfterDocument(trace, signatures[0], `${INITIAL_TEXT}console.log(`);
  });

  it("uses byte-length framing in both directions for a multibyte dot completion", () => {
    const { file, trace: tracePath } = startCase("string-dot-trigger");
    const typed = '"héllo界".';

    tui.type(typed);
    tui.waitFor("toÜpper");
    const visible = tui.snapshot();

    const { snapshots } = tui.run();
    const trace = readTrace(tracePath);
    const completions = messagesFor(trace, "textDocument/completion");

    expect(snapshots[visible]).toContain("toÜpper");
    assertProtocol(trace, file, expectedDocumentTexts(typed));
    expect(completions).toEqual([
      {
        jsonrpc: "2.0",
        id: expect.any(Number),
        method: "textDocument/completion",
        params: {
          textDocument: { uri: `file://${file}` },
          position: { line: 1, character: 9 },
          context: { triggerKind: 2, triggerCharacter: "." },
        },
      },
    ]);
    assertRequestAfterDocument(trace, completions[0], `${INITIAL_TEXT}${typed}`);
    expect(
      trace.find(
        (entry) =>
          entry.direction === "server->client" &&
          entry.message.id === completions[0].id,
      ).message,
    ).toEqual({
      jsonrpc: "2.0",
      id: completions[0].id,
      result: {
        isIncomplete: false,
        items: [{ label: "toÜpper", kind: 2, insertText: "toÜpper" }],
      },
    });
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
    const completions = messagesFor(trace, "textDocument/completion");
    expect(completions).toEqual([
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
    ]);
    assertRequestAfterDocument(trace, completions[0], `${INITIAL_TEXT}console.`);
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
