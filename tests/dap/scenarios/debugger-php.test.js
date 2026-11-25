import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { it } from "vitest";
import * as tui from "../../functional/tui.js";
import { cases, copyTarget, startEditor } from "../matrix/harness.js";

it("[php/xdebug] stops two independent PHP invocations at the same breakpoint", () => {
  const spec = cases.find(c => c.name === "php-xdebug");
  const dir = mkdtempSync(join(tmpdir(), "ttt-xdebug-"));
  try {
    const file = copyTarget(dir, spec);
    const launch = {...spec.launch, runtimeExecutable: "bash",
      runtimeArgs: ["-ec", 'for initial in 21 41; do php -dxdebug.start_with_request=yes main.php "$initial"; done']};
    delete launch.program;
    startEditor(dir, file, spec, launch);
    for (let line = 1; line < spec.baseline.breakpoint.line; line++) tui.press("down");
    tui.exec("Toggle Breakpoint");
    tui.exec("Start Debugging");
    for (const initial of [21, 41]) {
      tui.waitFor(`│    $count = ${initial} `);
      tui.exec("Debug: Continue");
    }
    tui.panel("output");
    tui.waitFor("[debug:php-xdebug] count = 43");
    tui.waitFor("Debug session ended");
    tui.run(30000);
  } finally {
    rmSync(dir, {recursive: true, force: true});
  }
}, 35000);
