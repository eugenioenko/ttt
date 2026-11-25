import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { setEnv } from "../../functional/tui.js";

export function configureDebugAdapters(dir) {
  const config = join(dir, "user-config");
  mkdirSync(config, { recursive: true });
  const adapters = {};
  for (const name of ["go-delve", "python-debugpy", "ts-js-debug"]) {
    const spec = JSON.parse(readFileSync(new URL(`../cases/${name}.json`, import.meta.url), "utf8"));
    Object.assign(adapters, spec.settings.debug.adapters);
  }
  adapters.node = adapters["pwa-node"];
  writeFileSync(join(config, "settings.json"), JSON.stringify({ debug: { adapters } }));
  setEnv({ TTT_CONFIG_DIR: config });
}
