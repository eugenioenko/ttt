import { execFileSync } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { it, expect } from "vitest";
import * as tui from "./tui.js";

const python = process.env.TTT_TEST_DEBUGPY;
it.skipIf(!python)("debugs a module with a dependency from its workspace virtual environment", () => {
  const dir = mkdtempSync(join(tmpdir(), "ttt-python-debug-"));
  try {
    const adapterLib = execFileSync(python, ["-c", "import debugpy,pathlib;print(pathlib.Path(debugpy.__file__).resolve().parents[1])"], { encoding: "utf8" }).trim();
    execFileSync(python, ["-m", "venv", "--without-pip", join(dir, ".venv")]);
    const exe = join(dir, ".venv", process.platform === "win32" ? "Scripts/python.exe" : "bin/python");
    const site = execFileSync(exe, ["-c", "import sysconfig;print(sysconfig.get_path('purelib'))"], { encoding: "utf8" }).trim();
    writeFileSync(join(site, "adapter.pth"), adapterLib + "\n");
    writeFileSync(join(site, "ttt_private_fixture.py"), "VALUE = 41\n");
    const file = join(dir, "appmod.py");
    writeFileSync(file, ['from ttt_private_fixture import VALUE', 'def report(arg):', '    local = arg + 1',
      '    emit(local)', '    print("PY_DONE", local + 1)', 'def emit(value):', '    print("PY_RESULT", value)', 'report(VALUE)', ''].join("\n"));
    mkdirSync(join(dir, ".ttt"));
    writeFileSync(join(dir, ".ttt", "launch.json"), JSON.stringify({
      configurations: [{ name: "Example", type: "debugpy", request: "launch", module: "appmod" }],
    }));
    tui.start(dir, file);
    tui.exec("Show Debug");
    tui.click(32, 7);
    tui.exec("Start Debugging");
    tui.waitFor("local = 42");
    const stopped = tui.snapshot();
    tui.press("f11");
    tui.waitFor("emit  appmod.py:7");
    tui.waitFor("value = 42");
    tui.press("shift+f11");
    tui.waitFor("local = 42");
    const returned = tui.snapshot();
    tui.press("f10");
    tui.waitFor("report  appmod.py:5");
    tui.exec("Debug: Continue");
    tui.panel("output");
    tui.waitFor("Debug session ended");
    const output = tui.snapshot();
    const { snapshots } = tui.run(60000);
    expect(snapshots[stopped]).toContain("report  appmod.py:4");
    expect(snapshots[returned]).toContain("report  appmod.py:4");
    expect(snapshots[output]).toContain("[debug:Example] PY_DONE 43");
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}, 70000);
