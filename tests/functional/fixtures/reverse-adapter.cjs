// A parent and its child each get a fresh stdio process. The parent requests
// an actual integrated terminal and waits for the child handshake to finish.
let input = Buffer.alloc(0);
let sequence = 0;
let launch;
function send(packet) {
  const body = JSON.stringify({seq: ++sequence, ...packet});
  process.stdout.write(`Content-Length: ${Buffer.byteLength(body)}\r\n\r\n${body}`);
}
function handle(p) {
  const reply = body => send({type:"response", request_seq:p.seq, command:p.command, success:true, body});
  if (p.type === "response") {
    if (!p.success) throw Error(p.message);
    if (p.command === "runInTerminal") {
      if (!(p.body.processId > 0)) throw Error("missing terminal PID");
      send({type:"request", command:"startDebugging", arguments:{request:"launch", configuration:{child:true}}});
    } else if (p.command === "startDebugging") {
      send({type:"event",event:"output",body:{output:"CHILD_CONFIGURED\n"}});
    }
    return;
  }
  switch (p.command) {
    case "initialize":
      if (!p.arguments.supportsRunInTerminalRequest || !p.arguments.supportsStartDebuggingRequest) throw Error("missing capability");
      return reply({supportsConfigurationDoneRequest:true});
    case "launch":
      launch = p.arguments;
      reply({});
      return send({type:"event",event:"initialized"});
    case "configurationDone":
      reply({});
      if (!launch.child) {
        const cwd = launch.paths[0];
        send({type:"request",command:"runInTerminal",arguments:{
          kind:"integrated", cwd, args:[process.execPath, "-e",
            'require("node:fs").writeFileSync("terminal.json",JSON.stringify([process.cwd(),process.argv[1],process.env.DAP_VALUE,process.env.DAP_REMOVED??null]));console.log("TERMINAL_COMPLETED");process.stdin.resume()',
            "one arg $HOME"],env:{DAP_VALUE:"set",DAP_REMOVED:null},
        }});
      } else {
        send({type:"event",event:"output",body:{output:"CHILD_INITIALIZED\n"}});
      }
      return;
    case "disconnect":
      reply({});
      return process.stdin.pause();
    default: return reply({});
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
