import { spawn, spawnSync, execFileSync } from "node:child_process";
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { once } from "node:events";
import { setTimeout as delay } from "node:timers/promises";
import { it, expect } from "vitest";
import * as tui from "./tui.js";
import { configureDebugAdapters } from "./debug-adapters.js";

const python = process.env.TTT_TEST_DEBUGPY;
const linux = process.platform === "linux";
const hasDelve = spawnSync("dlv", ["version"], { stdio: "ignore" }).status === 0;

async function checkPID(type, picker) {
  const dir = mkdtempSync(join(tmpdir(), "ttt-pid-"));
  let child;
  try {
    const go = type === "go";
    const node = type === "node";
    const file = join(dir, go ? "main.go" : node ? "app.js" : "example.py");
    // These fixtures authorize their own parent test runner and its debugger descendants under Linux Yama.
    const lines = go ? [
      'package main', 'import ("os"; "syscall"; "time"; "strconv")', 'func main() {',
      ' if _,_,err:=syscall.Syscall6(syscall.SYS_PRCTL,0x59616d61,uintptr(os.Getppid()),0,0,0,0);err!=0{panic(err)}',
      ' for i:=0;;i++ {', '  value := 40+2', '  os.WriteFile("heartbeat", []byte(strconv.Itoa(i+value)), 0600)',
      '  time.Sleep(25*time.Millisecond)', ' }', '}',
    ] : node ? [
      'const fs = require("node:fs");', 'fs.writeFileSync("port", String(process.debugPort)); let i = 0;', 'setInterval(() => {',
      '  const value = 40 + 2;', '  fs.writeFileSync("heartbeat", String(i + value));',
      '  i++;', '}, 25);',
    ] : [
      'import ctypes, os, pathlib, time', 'assert ctypes.CDLL(None).prctl(0x59616d61, os.getppid(), 0, 0, 0) == 0',
      'def run():', '  i = 0', '  while True:', '    value = 40 + 2', '    pathlib.Path("heartbeat").write_text(str(i + value))',
      '    i += 1', '    time.sleep(0.025)', 'run()',
    ];
    writeFileSync(file, lines.join("\n"));
    const exe = go ? join(dir, "go-pid-fixture") : node ? process.execPath : python;
    if (go) execFileSync("go", ["build", "-gcflags=all=-N -l", "-o", exe, file]);
    child = spawn(exe, go ? [] : node ? ["--inspect=127.0.0.1:0", file] : [file], { cwd: dir });
    const pulse = () => { try {return Number(readFileSync(join(dir,"heartbeat"),"utf8"));} catch {return 0;} };
    for(let i=0; i<200 && !pulse(); i++) await delay(25);
    expect(pulse()).toBeGreaterThan(0);
    mkdirSync(join(dir, ".ttt"));
    writeFileSync(join(dir, ".ttt/launch.json"), JSON.stringify({
      configurations: [{ name: "Attach", type, request: "attach", processId: picker ? "${command:pickProcess}" : child.pid, ...(node ? {port:Number(readFileSync(join(dir,"port"),"utf8"))} : {}), ...(type === "debugpy" ? {python} : {}) }],
    }));
    const line = go ? 7 : node ? 5 : 7;
    tui.start(dir, file);
    configureDebugAdapters(dir);
    tui.exec("Show Debug");
    tui.click(32, 3 + line);
    tui.exec("Start Debugging");
    if (picker) {
      tui.waitFor("Select a Process to Debug");
      tui.type(String(child.pid));
      tui.press("enter");
    }
    tui.waitFor("value = 42");
    tui.exec("Debug: Step Over");
    tui.waitFor((go ? "main.go:" : node ? "app.js:" : "example.py:") + (line + 1));
    tui.exec("Detach Debugger");
    tui.waitFor("Debug session ended");
    tui.run(60000);
    const first = pulse();
    for(let i=0; i<200 && pulse()<=first; i++) await delay(25);
    expect(pulse()).toBeGreaterThan(first);
  } finally {
    if(child && child.exitCode===null && child.signalCode===null){const done=once(child,"exit");child.kill();await done;}
    rmSync(dir,{recursive:true,force:true});
  }
}
for (const picker of [true,false]) {
  it.skipIf(!process.env.TTT_JS_DEBUG_SERVER)("attaches to Node by "+(picker ? "picker" : "configured PID"), () => checkPID("node",picker), 70000);
  const via = picker ? "picker" : "configured PID";
  it.skipIf(!linux || !hasDelve)("debugs a Go process via "+via, () => checkPID("go",picker), 70000);
  it.skipIf(!linux || !python)("debugs a Python process via "+via, () => checkPID("debugpy",picker), 70000);
}
