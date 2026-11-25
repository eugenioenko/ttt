import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, it } from "vitest";
import * as tui from "../../functional/tui.js";
import { cases, copyTarget, executionMarker, startEditor } from "./harness.js";

for (const spec of cases) {
  it(`[${spec.name.replace("-", "/")}] launch, inspect locals, step over, in and out, continue to exit`, () => {
    const dir = mkdtempSync(join(tmpdir(), "ttt-dap-launch-"));
    try {
      const file = copyTarget(dir, spec);
      startEditor(dir, file, spec, spec.launch);
      tui.setSize(140, 40);
      const marker = executionMarker(dir, file);
      const snapshots = [];
      const inspect = ({line, locals}) => {
        tui.waitFor(marker(line), 15000);
        const variables = Object.entries(locals).map(([key, value]) => `│    ${key} = ${value} `);
        for (const variable of variables) tui.waitFor(variable, 15000);
        snapshots.push({index: tui.snapshot(), line, variables});
      };
      const {breakpoint, afterOver, insideCall, afterOut, output} = spec.baseline;
      for (let line = 1; line < breakpoint.line; line++) tui.press("down");
      tui.exec("Toggle Breakpoint");
      tui.exec("Start Debugging");
      inspect(breakpoint);
      tui.exec("Debug: Step Over");
      inspect(afterOver);
      tui.exec("Debug: Step In");
      inspect(insideCall);
      tui.exec("Debug: Step Out");
      inspect(afterOut);
      tui.exec("Debug: Continue");
      tui.panel("output");
      tui.waitFor(`[debug:${spec.launch.name}] ${output}`);
      tui.waitFor("Debug session ended");
      const result = tui.run(90000);
      for (const {index, line, variables} of snapshots) {
        expect(result.snapshots[index]).toContain(marker(line));
        for (const variable of variables) expect(result.snapshots[index]).toContain(variable);
      }
    } finally {
      rmSync(dir, {recursive: true, force: true});
    }
  }, 95000);
}
