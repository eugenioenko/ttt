import { appendFileSync, writeFileSync } from "node:fs";

const tracePath = process.argv[2];
if (!tracePath) {
  process.stderr.write("usage: node fake-lsp.mjs TRACE_PATH\n");
  process.exit(2);
}

writeFileSync(tracePath, "", "utf8");

let input = Buffer.alloc(0);
let sequence = 0;
let documentText = "";

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

function completionItem() {
  if (documentText.endsWith('"hello world!".')) {
    return { label: "toUpperCase", kind: 2, insertText: "toUpperCase" };
  }
  if (documentText.endsWith(".")) {
    return { label: "log", kind: 2, insertText: "log" };
  }
  return { label: "console", kind: 6, insertText: "console" };
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
        },
      });
      break;
    case "initialized":
      break;
    case "textDocument/didOpen":
      documentText = message.params.textDocument.text;
      send({
        jsonrpc: "2.0",
        method: "textDocument/publishDiagnostics",
        params: {
          uri: message.params.textDocument.uri,
          diagnostics: [
            {
              range: {
                start: { line: 0, character: 0 },
                end: { line: 0, character: 1 },
              },
              severity: 3,
              source: "fake-lsp",
              message: "FAKE_LSP_READY",
            },
          ],
        },
      });
      break;
    case "textDocument/didChange":
      documentText = message.params.contentChanges[0].text;
      break;
    case "textDocument/completion":
      respond(message.id, { isIncomplete: false, items: [completionItem()] });
      break;
    case "completionItem/resolve":
      respond(message.id, message.params);
      break;
    case "textDocument/signatureHelp":
      respond(message.id, {
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
      respond(message.id, []);
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
