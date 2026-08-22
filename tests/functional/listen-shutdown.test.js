import { spawn } from "node:child_process";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { request as httpRequest } from "node:http";
import { createConnection, createServer } from "node:net";
import { join, resolve } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { cleanupDir, createTempDir } from "./helpers.js";

const BINARY = resolve(import.meta.dirname, "../../bin/ttt");
let child;
let dir;

function allocatePort() {
  return new Promise((resolvePort, reject) => {
    const server = createServer();
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address();
      server.close((err) => (err ? reject(err) : resolvePort(port)));
    });
  });
}

function postExec(port, body, timeout = 1000) {
  return new Promise((resolveResponse, reject) => {
    const req = httpRequest(
      {
        hostname: "127.0.0.1",
        port,
        path: "/exec",
        method: "POST",
        headers: { "Content-Length": Buffer.byteLength(body) },
      },
      (res) => {
        let responseBody = "";
        res.setEncoding("utf8");
        res.on("data", (chunk) => {
          responseBody += chunk;
        });
        res.once("aborted", () =>
          reject(new Error("response aborted before completion")),
        );
        res.once("end", () =>
          resolveResponse({ status: res.statusCode, body: responseBody }),
        );
      },
    );
    req.setTimeout(timeout, () => req.destroy(new Error("request timed out")));
    req.once("error", reject);
    req.end(body);
  });
}

function waitForExit(process, timeout = 3000) {
  if (process.exitCode !== null || process.signalCode !== null) {
    return Promise.resolve({
      code: process.exitCode,
      signal: process.signalCode,
    });
  }
  return new Promise((resolveExit, reject) => {
    const timer = setTimeout(() => {
      process.removeListener("exit", onExit);
      reject(new Error("process did not exit before timeout"));
    }, timeout);
    const onExit = (code, signal) => {
      clearTimeout(timer);
      resolveExit({ code, signal });
    };
    process.once("exit", onExit);
  });
}

function probeListener(port) {
  return new Promise((resolveProbe, reject) => {
    const socket = createConnection({ host: "127.0.0.1", port });
    const timer = setTimeout(() => {
      socket.destroy();
      reject(new Error("listener probe timed out"));
    }, 250);
    socket.once("connect", () => {
      clearTimeout(timer);
      socket.destroy();
      resolveProbe();
    });
    socket.once("error", (err) => {
      clearTimeout(timer);
      reject(err);
    });
  });
}

async function waitForTCPListener(port, process) {
  const deadline = Date.now() + 5000;
  let lastError;
  while (Date.now() < deadline) {
    if (process.exitCode !== null) {
      throw new Error(`listener exited early with code ${process.exitCode}`);
    }
    try {
      await probeListener(port);
      return;
    } catch (err) {
      lastError = err;
    }
    await new Promise((resolveDelay) => setTimeout(resolveDelay, 25));
  }
  throw lastError ?? new Error("listener did not become ready");
}

async function waitForFile(path, process) {
  const deadline = Date.now() + 3000;
  while (Date.now() < deadline) {
    if (existsSync(path)) return;
    if (process.exitCode !== null) {
      throw new Error(`process exited before creating ${path}`);
    }
    await new Promise((resolveDelay) => setTimeout(resolveDelay, 10));
  }
  throw new Error(`process did not create ${path}`);
}

function writeBlockingTimerPlugin(directory) {
  const plugin = join(directory, "blocking-timer.lua");
  const startedPath = join(directory, "timer-started");
  writeFileSync(
    plugin,
    `local ttt = require("ttt")
local fs = require("ttt.fs")
ttt.set_timeout(100, function()
  fs.write(${JSON.stringify(startedPath)}, "started")
  local started = os.clock()
  while os.clock() - started < 6 do end
  ttt.log("info", "BLOCK DONE")
end)
`,
  );
  return { plugin, startedPath };
}

async function waitForListener(port, process) {
  const deadline = Date.now() + 5000;
  let lastError;
  while (Date.now() < deadline) {
    if (process.exitCode !== null) {
      throw new Error(`listener exited early with code ${process.exitCode}`);
    }
    try {
      const response = await postExec(port, "wait 0", 250);
      if (response.status === 200 && response.body === "ok") return;
      lastError = new Error(
        `unexpected readiness response ${response.status} ${response.body}`,
      );
    } catch (err) {
      lastError = err;
    }
    await new Promise((resolveDelay) => setTimeout(resolveDelay, 25));
  }
  throw lastError ?? new Error("listener did not become ready");
}

afterEach(async () => {
  if (child && child.exitCode === null && child.signalCode === null) {
    child.kill("SIGTERM");
    try {
      await waitForExit(child, 1000);
    } catch {
      child.kill("SIGKILL");
      await waitForExit(child, 1000).catch(() => {});
    }
  }
  child = undefined;
  if (dir) cleanupDir(dir);
  dir = undefined;
});

describe("live listen shutdown", () => {
  it("drains a claimed timer callback before CLI shutdown exits", async () => {
    dir = createTempDir();
    const { plugin, startedPath } = writeBlockingTimerPlugin(dir);
    const started = Date.now();
    child = spawn(
      BINARY,
      [
        "--size",
        "40x12",
        "--plugin",
        plugin,
        "--exec",
        "wait 200;quit",
        "README.md",
      ],
      {
        cwd: resolve(import.meta.dirname, "../.."),
        env: { ...process.env, TTT_CONFIG_DIR: join(dir, "config") },
        stdio: ["ignore", "ignore", "pipe"],
      },
    );

    await waitForFile(startedPath, child);
    await expect(waitForExit(child, 10000)).resolves.toEqual({
      code: 0,
      signal: null,
    });
    expect(Date.now() - started).toBeGreaterThanOrEqual(5900);
  }, 12000);

  it("flushes the success response before the process exits", async () => {
    dir = createTempDir();
    const port = await allocatePort();
    child = spawn(BINARY, ["--size", "80x24", "--listen", "--exec", "wait 0"], {
      cwd: dir,
      env: {
        ...process.env,
        TTT_CONFIG_DIR: join(dir, "config"),
        TTT_LISTEN_PORT: String(port),
      },
      stdio: ["ignore", "ignore", "pipe"],
    });

    await waitForListener(port, child);
    const response = await postExec(port, "shutdown");
    expect(response).toEqual({ status: 200, body: "ok" });
    await expect(waitForExit(child)).resolves.toEqual({
      code: 0,
      signal: null,
    });
  });

  it("waits for a claimed slow command instead of returning a false timeout", async () => {
    dir = createTempDir();
    const port = await allocatePort();
    const plugin = join(dir, "blocking.lua");
    const debugPath = join(dir, "state.json");
    writeFileSync(
      plugin,
      `local ttt = require("ttt")
ttt.register({
  commands = {
    {
      id = "review.block",
      title = "Review: Block",
      handler = function()
        local started = os.clock()
        while os.clock() - started < 6 do end
        ttt.log("info", "BLOCK DONE")
      end,
    },
  },
})
`,
    );
    child = spawn(
      BINARY,
      ["--size", "80x24", "--plugin", plugin, "--listen", "--exec", "wait 0"],
      {
        cwd: dir,
        env: {
          ...process.env,
          TTT_CONFIG_DIR: join(dir, "config"),
          TTT_LISTEN_PORT: String(port),
        },
        stdio: ["ignore", "ignore", "pipe"],
      },
    );

    await waitForListener(port, child);
    const started = Date.now();
    const response = await postExec(port, `exec "Review: Block"`, 10000);
    expect(response).toEqual({ status: 200, body: "ok" });
    expect(Date.now() - started).toBeGreaterThanOrEqual(5900);

    await expect(postExec(port, `debug ${debugPath}`)).resolves.toEqual({
      status: 200,
      body: "ok",
    });
    expect(readFileSync(debugPath, "utf8")).toContain("BLOCK DONE");

    await expect(postExec(port, "shutdown")).resolves.toEqual({
      status: 200,
      body: "ok",
    });
    await expect(waitForExit(child)).resolves.toEqual({
      code: 0,
      signal: null,
    });
  }, 15000);

  it("honors concurrent HTTP and CLI shutdown queued behind a claimed timer callback", async () => {
    dir = createTempDir();
    const port = await allocatePort();
    const { plugin, startedPath } = writeBlockingTimerPlugin(dir);
    child = spawn(
      BINARY,
      [
        "--size",
        "40x12",
        "--plugin",
        plugin,
        "--listen",
        "--exec",
        "wait 200;quit",
      ],
      {
        cwd: dir,
        env: {
          ...process.env,
          TTT_CONFIG_DIR: join(dir, "config"),
          TTT_LISTEN_PORT: String(port),
        },
        stdio: ["ignore", "ignore", "pipe"],
      },
    );

    await waitForTCPListener(port, child);
    await waitForFile(startedPath, child);
    const started = Date.now();
    await expect(postExec(port, "shutdown", 2000)).resolves.toEqual({
      status: 200,
      body: "ok",
    });
    await expect(waitForExit(child, 10000)).resolves.toEqual({
      code: 0,
      signal: null,
    });
    expect(Date.now() - started).toBeGreaterThanOrEqual(5500);
  }, 12000);
});
