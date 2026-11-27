import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {test} from "node:test";
import * as tui from "../../functional/tui.js";
import { cases, copyTarget, startEditor } from "./harness.js";

for (const spec of cases.filter(c => c.name === "cpp-codelldb")) {
  test("[cpp/codelldb] expands this and inspects a changed member", () => {
    const dir = mkdtempSync(join(tmpdir(), "ttt-cpp-this-"));
    try {
      const file = copyTarget(dir, spec);
      startEditor(dir, file, spec, spec.launch);
      for (let line = 1; line < spec.inspection.find(step => step.key === "f11").line; line++) tui.press("down");
      tui.exec("Toggle Breakpoint");
      tui.exec("Start Debugging");
      tui.waitFor("› this = {total:1}");
      tui.click(3, 9);
      tui.waitFor("total = 1");
      tui.exec("Debug: Step Over");
      tui.waitFor("this = {total:43}");
      tui.exec("Debug: Continue");
      tui.waitFor("Debug session ended");
      tui.run(30000);
    } finally {
      rmSync(dir, {recursive: true, force: true});
    }
  });
}
