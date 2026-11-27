import {cpSync, mkdirSync, readFileSync, readdirSync, writeFileSync} from "node:fs";
import {basename, dirname, join} from "node:path";
import {fileURLToPath} from "node:url";
import * as tui from "../../functional/tui.js";

const root = fileURLToPath(new URL("../", import.meta.url));
const selected = JSON.parse(process.env.DAP_CASES || "null");
export const cases = readdirSync(join(root, "cases")).map(entry =>
  JSON.parse(readFileSync(join(root, "cases", entry), "utf8"))).filter(spec => !selected || selected.includes(spec.name));

export function copyTarget(dir, fixture) {
  cpSync(dirname(join(root, fixture.source)), dir, {recursive: true});
  if (fixture.binary) cpSync(join(root, fixture.binary), join(dir, basename(fixture.binary)), {recursive: true});
  return basename(fixture.source);
}

export function startEditor(dir, file, spec, target) {
  mkdirSync(join(dir, ".ttt"));
  writeFileSync(join(dir, ".ttt/launch.json"), JSON.stringify({configurations: [
    {type: spec.launch.type, ...target},
  ]}));
  const config = join(dir, "config");
  mkdirSync(config);
  writeFileSync(join(config, "settings.json"), JSON.stringify(spec.settings));
  const workspace = spec.workspacePath || dir;
  tui.start(workspace, join(workspace, file));
  if (spec.workspacePath) tui.setCwd(dir);
  tui.setEnv({TTT_CONFIG_DIR: config});
}

export function executionMarker(dir, file) {
  const lines = readFileSync(join(dir, file), "utf8").split("\n").length;
  const width = Math.max(2, String(lines).length);
  return line => `▶${String(line).padStart(width)} `;
}
