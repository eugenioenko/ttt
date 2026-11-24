import { spawn, spawnSync, execFileSync } from "node:child_process";
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { once } from "node:events";
import { setTimeout as delay } from "node:timers/promises";
import { it, expect } from "vitest";
import * as tui from "./tui.js";

const python = process.env.TTT_TEST_DEBUGPY;
const hasDelve = spawnSync("dlv", ["version"], { stdio: "ignore" }).status === 0;

async function waitFor(read, child) {
  const deadline = Date.now() + 10000;
  while (Date.now() < deadline) {
    const result = read();
    if (result) return result;
    if (child.exitCode !== null) throw Error("target exited: " + child.exitCode);
    await delay(25);
  }
  throw Error("timed out waiting for target");
}

async function checkAddressAttach(type) {
  const dir = mkdtempSync(join(tmpdir(), "ttt-address-"));
  let child;
  try {
    const heartbeat = join(dir, "heartbeat");
    const file = join(dir, type === "go" ? "main.go" : "example.py");
    let ready, line;
    if (type === "go") {
      writeFileSync(file, [
        'package main', 'import ("time"; "os"; "strconv")', 'func main() {',
        ' for i:=0;;i++ {', '  value := 42', '  os.WriteFile("heartbeat", []byte(strconv.Itoa(i+value)), 0600)',
        '  time.Sleep(25*time.Millisecond)', ' }', '}',
      ].join("\n"));
      const exe = join(dir, "probe");
      execFileSync("go", ["build", "-gcflags=all=-N -l", "-o", exe, file]);
      child = spawn("dlv", ["exec", exe, "--headless", "--accept-multiclient", "--listen=127.0.0.1:0"], { cwd: dir });
      let log = "";
      child.stdout.on("data", chunk => { log += chunk; });
      ready = () => log.match(/listening at: 127\.0\.0\.1:(\d+)/)?.[1];
      line = 6;
    } else {
      writeFileSync(file, [
        'import debugpy, pathlib, time', 'port = debugpy.listen(("127.0.0.1", 0))[1]',
        'pathlib.Path("ready").write_text(str(port))', 'debugpy.wait_for_client()',
        'i = 0', 'while True:', '    value = 40 + 2', '    pathlib.Path("heartbeat").write_text(str(i + value))',
        '    i += 1', '    time.sleep(0.025)',
      ].join("\n"));
      child = spawn(python, [file], { cwd: dir });
      ready = () => { try { return readFileSync(join(dir, "ready"), "utf8"); } catch { return ""; } };
      line = 8;
    }
    const port = Number(await waitFor(ready, child));
    const endpoint = type === "go" ? { mode: "remote", host: "127.0.0.1", port } : { connect: { host: "127.0.0.1", port } };
    mkdirSync(join(dir, ".ttt"));
    writeFileSync(join(dir, ".ttt/launch.json"), JSON.stringify({
      configurations: [{ name: "Attach", type, request: "attach", ...endpoint }],
    }));
    tui.start(dir, file);
    tui.click(37, 3 + line);
    tui.exec("Start Debugging");
    tui.waitFor("value = 42");
    tui.exec("Debug: Step Over");
    tui.waitFor((type === "go" ? "main.go:" : "example.py:") + (line + 1));
    tui.exec("Detach Debugger");
    tui.waitFor("Debug session ended");
    tui.run(40000);
    const pulse = () => { try { return readFileSync(heartbeat, "utf8"); } catch { return ""; } };
    const first = Number(await waitFor(pulse, child));
    expect(await waitFor(() => Number(pulse()) > first, child)).toBe(true);
  } finally {
    if (child && child.exitCode === null && child.signalCode === null) {
      const closed = once(child, "exit");
      child.kill("SIGINT");
      await closed;
    }
    rmSync(dir, { recursive: true, force: true });
  }
}
it.skipIf(!hasDelve)("attaches to headless Delve, steps and detaches with the program running", () => checkAddressAttach("go"), 60000);
it.skipIf(!python)("attaches to a debugpy listener, steps and detaches with the program running", () => checkAddressAttach("debugpy"), 60000);
