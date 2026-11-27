// Keep the launch request pending until the editor replies to runInTerminal.
const fs = require("node:fs");
const path = require("node:path");
let input = Buffer.alloc(0);
let sequence = 0;
let launch;
let terminalRequest;

function send(packet) {
  const seq = ++sequence;
  const body = JSON.stringify({seq, ...packet});
  process.stdout.write(`Content-Length: ${Buffer.byteLength(body)}\r\n\r\n${body}`);
  return seq;
}

function reply(p, body) {
  send({type: "response", request_seq: p.seq, command: p.command, success: true, body});
}

function handle(p) {
  if (p.type === "response") {
    if (p.request_seq !== terminalRequest || !p.success) throw Error(JSON.stringify(p));
    fs.writeFileSync(path.join(launch.arguments.cwd, "adapter-pid"), String(p.body.processId));
    reply(launch);
    return;
  }
  switch (p.command) {
    case "initialize":
      if (!p.arguments.supportsRunInTerminalRequest) throw Error("missing terminal capability");
      return reply(p, {supportsConfigurationDoneRequest: true});
    case "attach":
    case "launch":
      launch = p;
      send({type: "event", event: "initialized"});
      terminalRequest = send({type: "request", command: "runInTerminal", arguments: {
        kind: "integrated", title: "Debug target", cwd: launch.arguments.cwd,
        args: [process.execPath, "-e",
          `const fs = require("node:fs");
           fs.writeFileSync("target.json", JSON.stringify([process.pid, process.cwd(), process.argv[1], process.env.DAP_VALUE, process.env.DAP_REMOVED ?? null, process.env.TERM ?? null]));
           const timer = setInterval(() => {
             if (fs.existsSync("adapter-pid")) {
               clearInterval(timer);
               console.log("TERMINAL_READY");
             }
           }, 10);
           process.stdin.resume();`,
          "one arg $HOME"],
        env: {DAP_VALUE: "set", DAP_REMOVED: null, TERM: p.command === "launch" ? "dumb" : null}
      }});
      return;
    case "disconnect":
      reply(p);
      return process.stdin.pause();
    default:
      reply(p, {});
  }
}

process.stdin.on("data", chunk => {
  input = Buffer.concat([input, chunk]);
  for (;;) {
    const end = input.indexOf("\r\n\r\n");
    if (end < 0) return;
    const length = Number(/Content-Length: (\d+)/i.exec(input.subarray(0, end).toString())[1]);
    if (input.length < end + 4 + length) return;
    const packet = JSON.parse(input.subarray(end + 4, end + 4 + length));
    input = input.subarray(end + 4 + length);
    handle(packet);
  }
});
