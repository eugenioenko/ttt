import { describe, it, expect, afterEach } from "vitest";
import { spawn } from "node:child_process";
import { writeFileSync, existsSync } from "node:fs";
import { join } from "node:path";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";

let dir;
let hub;

afterEach(() => {
  tui.kill();
  hub?.kill();
  hub = undefined;
  if (dir) cleanupDir(dir);
});

// Runs in its own process: tui.run() blocks this one until ttt exits.
const hubSource = `
const net = require("node:net");
const socketPath = process.argv[2];
const server = net.createServer((sock) => {
  let buf = Buffer.alloc(0);
  const send = (msg) => {
    const body = Buffer.from(JSON.stringify({ jsonrpc: "2.0", ...msg }));
    sock.write("Content-Length: " + body.length + "\\r\\n\\r\\n");
    sock.write(body);
  };
  sock.on("data", (chunk) => {
    buf = Buffer.concat([buf, chunk]);
    for (;;) {
      const end = buf.indexOf("\\r\\n\\r\\n");
      if (end < 0) return;
      const len = Number(/Content-Length: (\\d+)/.exec(buf.subarray(0, end).toString())[1]);
      if (buf.length < end + 4 + len) return;
      const msg = JSON.parse(buf.subarray(end + 4, end + 4 + len).toString());
      buf = buf.subarray(end + 4 + len);
      if (msg.method === "hello") {
        send({ id: msg.id, result: { from: "hub" } });
        send({ id: 1, method: "editor/type", params: { text: "typed by hub" } });
      } else if (msg.id === 1 && msg.result) {
        send({ method: "done", params: { reply: msg.result.typed } });
      }
    }
  });
});
server.listen(socketPath);
`;

function pluginSource(endpoint) {
  return `local ttt = require("ttt")
local rpc = require("ttt.rpc")
ttt.register({
  commands = {
    { id = "hub.connect", title = "Hub Connect", handler = function()
      rpc.connect(${JSON.stringify(endpoint)}, {
        on_request = function(method, params)
          if method == "editor/type" then
            ttt.type(params.text)
            return { typed = #params.text }
          end
          error("unknown method " .. method)
        end,
        on_notification = function(method, params)
          ttt.log("info", "hub " .. method .. " reply=" .. tostring(params.reply))
        end,
      }, function(conn, err)
        if not conn then
          ttt.log("error", "connect failed: " .. tostring(err))
          return
        end
        conn:request("hello", {}, function(result)
          ttt.log("info", "hello from " .. result.from)
        end)
      end)
    end },
  },
})
`;
}

describe("plugin rpc", () => {
  it("lets a peer process drive the editor over a socket", async () => {
    dir = createTempDir();
    const file = createTempFile(dir, "notes.txt", "");
    const socketPath = join(dir, "hub.sock");
    const hubPath = join(dir, "hub.cjs");
    const pluginPath = join(dir, "hub-plugin.lua");
    writeFileSync(hubPath, hubSource);
    writeFileSync(pluginPath, pluginSource(`unix:${socketPath}`));
    hub = spawn(process.execPath, [hubPath, socketPath], { stdio: "ignore" });
    for (let i = 0; i < 100 && !existsSync(socketPath); i++) {
      await new Promise((r) => setTimeout(r, 20));
    }

    tui.start(file, "--plugin", pluginPath);
    tui.exec("Hub Connect");
    tui.waitFor("typed by hub");
    const typed = tui.snapshot();
    tui.panel("output");
    tui.waitFor("hub done reply=12");
    const output = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[typed]).toMatch(/1\s+typed by hub/);
    expect(snapshots[output]).toContain("hello from hub");
  });
});
