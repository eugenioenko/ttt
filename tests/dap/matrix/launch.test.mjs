import assert from "node:assert/strict";
import {test} from "node:test";
import {mkdtempSync, readFileSync, rmSync} from "node:fs";
import {tmpdir} from "node:os";
import {join} from "node:path";
import * as tui from "../../functional/tui.js";
import {cases, copyTarget, executionMarker, startEditor} from "./harness.js";

for (const spec of cases) {
  test(`[${spec.name}] launch, hit a breakpoint, and stop`, () => {
    const dir = mkdtempSync(join(tmpdir(), "ttt-dap-launch-"));
    try {
      const file = copyTarget(dir, spec);
      startEditor(dir, file, spec, spec.launch);
      // Add a trailing blank line so launching has to save the source first.
      tui.press("ctrl+end");
      tui.press("enter");
      tui.press("ctrl+home");
      const before = readFileSync(join(dir, file), "utf8");
      const line = spec.breakpoint.line;
      // Default 120x40 layout: the first editor gutter cell is (32, 4).
      tui.click(32, 4 + line - 1);
      const execution = executionMarker(dir, file)(line);
      const breakpoint = execution.replace("▶", "⦁");
      tui.waitFor(breakpoint);
      tui.exec("Start Debugging");
      tui.waitFor(execution, 20000);
      const stopped = tui.snapshot();
      for (const {key, line: expected, variable} of spec.inspection) {
        if (key) tui.press(key);
        tui.waitFor(executionMarker(dir, file)(expected), 10000);
        tui.waitFor(`│    ${variable} `, 10000);
      }
      tui.exec("Stop Debugging");
      tui.waitFor("Debug session ended", 5000);
      const ended = tui.snapshot();
      const {snapshots} = tui.run(35000);
      assert.ok(readFileSync(join(dir, file), "utf8").length > before.length);
      assert.ok(snapshots[stopped].includes(execution));
      assert.ok(snapshots[ended].includes(breakpoint));
    } finally {
      rmSync(dir, {recursive: true, force: true});
    }
  });
}
