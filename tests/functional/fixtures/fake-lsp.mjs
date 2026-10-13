import { appendFileSync, writeFileSync } from "node:fs";

const tracePath = process.argv[2];
if (!tracePath) {
  process.stderr.write("usage: node fake-lsp.mjs TRACE_PATH [--hold-hover] [--late=METHOD] [--pull [--push] [--retrigger-first]]\n");
  process.exit(2);
}
// --pull answers textDocument/diagnostic and stops pushing publishDiagnostics,
// like TypeScript 7's native server. --push keeps pushing alongside it.
// --retrigger-first cancels the first pull and asks the client to resend it.
const flags = new Set(process.argv.slice(3));
const pullMode = flags.has("--pull");
const pushMode = !pullMode || flags.has("--push");
let retriggerPending = flags.has("--retrigger-first");

writeFileSync(tracePath, "", "utf8");

let input = Buffer.alloc(0);
let sequence = 0;
let documentText = "";
let holdNextHover = flags.has("--hold-hover");
let pendingHover;
let symbolRequests = 0;
// --late=METHOD answers the first request for METHOD only after a delay, so a
// test can move the cursor before the reply arrives.
let lateMethod = process.argv.find((arg) => arg.startsWith("--late="))?.slice("--late=".length);

function record(direction, message) {
  appendFileSync(
    tracePath,
    `${JSON.stringify({ sequence: ++sequence, direction, message })}\n`,
    "utf8",
  );
}

function send(message) {
  record("server->client", message);
  const body = JSON.stringify(message);
  process.stdout.write(`Content-Length: ${Buffer.byteLength(body)}\r\n\r\n${body}`);
}

function respond(id, result) {
  send({ jsonrpc: "2.0", id, result });
}

function diagnosticItem(line, message) {
  return {
    range: {
      start: { line, character: 0 },
      end: { line, character: 1 },
    },
    severity: 3,
    source: "fake-lsp",
    message,
  };
}

function diagnostic(uri, message) {
  send({
    jsonrpc: "2.0",
    method: "textDocument/publishDiagnostics",
    params: { uri, diagnostics: [diagnosticItem(0, message)] },
  });
}

function pulledDiagnostics() {
  if (documentText.includes("clean")) return [];
  const items = [diagnosticItem(0, "FAKE_LSP_READY")];
  if (documentText.includes("bad")) items.push(diagnosticItem(1, "FAKE_PULL_BAD"));
  return items;
}

function completionItem() {
  if (documentText.includes("héllo界")) {
    return { label: "toÜpper", kind: 2, insertText: "toÜpper" };
  }
  if (documentText.endsWith(".")) {
    return { label: "log", kind: 2, insertText: "log" };
  }
  return { label: "console", kind: 6, insertText: "console" };
}

function respondMaybeLate(message, result) {
  if (message.method !== lateMethod) {
    respond(message.id, result);
    return;
  }
  lateMethod = undefined;
  const uri = message.params.textDocument.uri;
  diagnostic(uri, "FAKE_LATE_PENDING");
  setTimeout(() => {
    respond(message.id, result);
    diagnostic(uri, "FAKE_LATE_SENT");
  }, 800);
}

function handle(message) {
  record("client->server", message);

  switch (message.method) {
    case "initialize":
      respond(message.id, {
        capabilities: {
          textDocumentSync: 1,
          completionProvider: { triggerCharacters: ["."] },
          signatureHelpProvider: {
            triggerCharacters: ["("],
            retriggerCharacters: [","],
          },
          documentSymbolProvider: true,
          ...(pullMode && {
            diagnosticProvider: {
              identifier: "fake",
              interFileDependencies: false,
              workspaceDiagnostics: false,
            },
          }),
        },
      });
      break;
    case "initialized":
      break;
    case "textDocument/didOpen":
      documentText = message.params.textDocument.text;
      if (!pushMode) break;
      diagnostic(message.params.textDocument.uri, pullMode ? "FAKE_PUSHED" : "FAKE_LSP_READY");
      break;
    case "textDocument/diagnostic":
      if (retriggerPending) {
        retriggerPending = false;
        send({
          jsonrpc: "2.0",
          id: message.id,
          error: { code: -32802, message: "busy", data: { retriggerRequest: true } },
        });
        break;
      }
      respond(message.id, { kind: "full", items: pulledDiagnostics() });
      break;
    case "textDocument/didChange":
      documentText = message.params.contentChanges[0].text;
      break;
    case "textDocument/completion":
      respondMaybeLate(message, { isIncomplete: false, items: [completionItem()] });
      break;
    case "textDocument/hover":
      if (holdNextHover) {
        holdNextHover = false;
        pendingHover = message;
        diagnostic(message.params.textDocument.uri, "FAKE_HOVER_PENDING");
      } else {
        respond(message.id, { contents: { kind: "markdown", value: "FAKE_HOVER_INFO" } });
      }
      break;
    case "$/cancelRequest":
      if (pendingHover?.id === message.params.id) {
        respond(pendingHover.id, { contents: { kind: "markdown", value: "FAKE_STALE_HOVER" } });
        diagnostic(pendingHover.params.textDocument.uri, "FAKE_HOVER_CANCELLED");
        pendingHover = undefined;
      }
      break;
    case "completionItem/resolve":
      respond(message.id, message.params);
      break;
    case "textDocument/signatureHelp":
      respondMaybeLate(message, {
        signatures: [
          {
            label: "fakeSignature(value: string)",
            parameters: [{ label: "value: string" }],
          },
        ],
        activeSignature: 0,
        activeParameter: 0,
      });
      break;
    case "textDocument/documentSymbol":
      respond(message.id, process.argv.includes("--symbols") ? [{
        name: `FAKE_SYMBOL_${++symbolRequests}`,
        kind: 12,
        range: { start: { line: 0, character: 0 }, end: { line: 0, character: 1 } },
        selectionRange: { start: { line: 0, character: 0 }, end: { line: 0, character: 1 } },
      }] : []);
      break;
    case "textDocument/rename":
      respond(message.id, {
        documentChanges: [{
          textDocument: { uri: message.params.textDocument.uri, version: null },
          edits: [{
            range: { start: { line: 0, character: 3 }, end: { line: 0, character: 7 } },
            newText: message.params.newName,
          }],
        }],
      });
      break;
    case "shutdown":
      respond(message.id, null);
      break;
    case "exit":
      process.exit(0);
      break;
    default:
      if (message.id !== undefined) {
        send({
          jsonrpc: "2.0",
          id: message.id,
          error: { code: -32601, message: `unsupported method ${message.method}` },
        });
      }
  }
}

function drain() {
  while (true) {
    const headerEnd = input.indexOf("\r\n\r\n");
    if (headerEnd < 0) return;

    const header = input.subarray(0, headerEnd).toString("ascii");
    const match = /^Content-Length:\s*(\d+)$/im.exec(header);
    if (!match) {
      process.stderr.write("missing Content-Length header\n");
      process.exit(2);
    }

    const length = Number(match[1]);
    const bodyStart = headerEnd + 4;
    const bodyEnd = bodyStart + length;
    if (input.length < bodyEnd) return;

    const body = input.subarray(bodyStart, bodyEnd).toString("utf8");
    input = input.subarray(bodyEnd);
    handle(JSON.parse(body));
  }
}

process.stdin.on("data", (chunk) => {
  input = Buffer.concat([input, chunk]);
  drain();
});

process.stdin.on("end", () => process.exit(0));
