import { fork, spawn } from "node:child_process";
import { once } from "node:events";
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { it, expect } from "vitest";
import * as tui from "./tui.js";
import { configureDebugAdapters } from "./debug-adapters.js";

it("cancels a waiting attach, releases the connection and reports an unreachable endpoint on retry", async () => {
  const dir = mkdtempSync(join(tmpdir(), "ttt-attach-cancel-"));
  let peer;
  try {
    const script = join(dir, "listener.cjs");
    writeFileSync(script, `
      const net = require("node:net");
      const fs = require("node:fs");
      const server = net.createServer(socket => {
        server.close();
        socket.once("data", () => {
          const event = JSON.stringify({type:"event",event:"output",body:{category:"console",output:"WAITING_FOR_ATTACH"}});
          socket.write("Content-Length: "+Buffer.byteLength(event)+"\\r\\n\\r\\n"+event);
        });
        socket.on("end", () => fs.writeFileSync("closed","yes"));
      });
      server.listen(0,"127.0.0.1", () => process.send(server.address().port));
    `);
    peer = fork(script, [], { cwd: dir, stdio: ["ignore", "ignore", "ignore", "ipc"] });
    const [port] = await once(peer, "message");
    mkdirSync(join(dir, ".ttt"));
    writeFileSync(join(dir, ".ttt/launch.json"), JSON.stringify({ configurations: [
      { name: "Waiting", type: "debugpy", request: "attach", connect: { port } },
    ] }));
    tui.start(dir);
    configureDebugAdapters(dir);
    tui.exec("Start Debugging");
    tui.panel("output");
    tui.waitFor("[debug:Waiting] WAITING_FOR_ATTACH");
    tui.exec("Detach Debugger");
    tui.waitFor("Debug session ended");
    tui.exec("Start Debugging");
    tui.waitFor("Waiting: dial tcp");
    tui.run(20000);
    expect(readFileSync(join(dir, "closed"), "utf8")).toBe("yes");
  } finally {
    if (peer?.exitCode === null) peer.kill();
    rmSync(dir, { recursive: true, force: true });
  }
}, 25000);

it.skipIf(!process.env.TTT_TEST_DEBUGPY)("keeps an independent Python process alive after PID attach startup fails", async () => {
  const dir = mkdtempSync(join(tmpdir(), "ttt-attach-fail-"));
  const child = spawn(process.env.TTT_TEST_DEBUGPY, ["-u", "-c", 'import sys; print("ready"); print(input())'], {stdio:"pipe"});
  try {
    await once(child.stdout, "data");
    mkdirSync(join(dir, ".ttt"));
    writeFileSync(join(dir, ".ttt/launch.json"), JSON.stringify({ configurations: [
      { name: "Failed", type: "debugpy", request: "attach", processId: child.pid, python: join(dir,"missing-interpreter") },
    ] }));
    tui.start(dir);
    configureDebugAdapters(dir);
    tui.exec("Start Debugging");
    tui.panel("output");
    tui.waitFor("Failed: fork/exec");
    tui.run();
    const response = once(child.stdout, "data");
    child.stdin.end("still alive\n");
    expect(String((await response)[0]).trim()).toBe("still alive");
  } finally {
    if (child.exitCode === null) child.kill();
    rmSync(dir, {recursive:true,force:true});
  }
});
