import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { setEnv } from "./tui.js";

export function configureDebugAdapters(dir) {
  const config = join(dir, "user-config");
  mkdirSync(config, { recursive: true });
  const node = { provider: "jsdebug", fileLaunch: { program: "${file}" }, languages: ["javascript", "typescript"], command: ["node", process.env.TTT_JS_DEBUG_SERVER || "dapDebugServer.js",process.platform==="win32"?"0":"${socket}","127.0.0.1"],transport:"listen",addressPattern:"^Debug server listening at (.+)$" };
  const adapters = {
    go: { provider: "delve", languages: ["go"], command: ["dlv","dap","--listen=unix:${socket}"], transport:"listen", addressPattern:"^DAP server listening at: (.+)$" },
    debugpy: { provider: "debugpy", languages: ["python"], command: ["${python}", "-m", "debugpy.adapter"] },
    node, "pwa-node": node,
  };
  writeFileSync(join(config, "settings.json"), JSON.stringify({debug: {adapters}}));
  setEnv({ TTT_CONFIG_DIR: config });
}
