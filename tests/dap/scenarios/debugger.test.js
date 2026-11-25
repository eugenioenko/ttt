import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, it } from "vitest";
import * as tui from "../../functional/tui.js";
import { configureDebugAdapters } from "./debug-adapters.js";


it("[go] saves, inspects, steps and reuses a gutter breakpoint with Delve", () => {
  const dir = mkdtempSync(join(tmpdir(), "ttt-dap-"));
  try {
    const file = join(dir, "main.go");
    writeFileSync(file, [
      "package main", 'import "fmt"', "func report(arg int) {",
      " local := arg + 1", ' fmt.Println("result", local)', " local++",
      ' fmt.Println("done", local)', "}", "func main() {", " caller := 41",
      " report(caller)", "}", "",
    ].join("\n"));
    writeFileSync(join(dir, "go.mod"), "module example\ngo 1.25\n");
    mkdirSync(join(dir, ".ttt"));
    writeFileSync(join(dir, ".ttt/launch.json"), JSON.stringify({
      configurations: [{ name: "Example", type: "go", request: "launch", program: "." }],
    }));
    tui.start(dir, file);
    configureDebugAdapters(dir);
    tui.type("// saved by launch");
    tui.press("enter");
    tui.click(32, 9);
    tui.exec("Start Debugging");
    tui.waitFor("local = 42");
    const stopped = tui.snapshot();
    tui.exec("Toggle Font Icons");
    const nerd = tui.snapshot();
    tui.exec("Toggle Font Icons");
    tui.click(9, 21);
    tui.waitFor("caller = 41");
    const caller = tui.snapshot();
    tui.click(2, 4);
    tui.waitFor("main.report  main.go:7");
    const stepped = tui.snapshot();
    tui.click(24, 4);
    tui.panel("output");
    tui.waitFor("Debug session ended");
    const output = tui.snapshot();
    tui.exec("Start Debugging");
    tui.waitFor("local = 42");
    tui.click(29, 4);
    tui.waitFor("Debug session ended");
    const { snapshots } = tui.run(60000);
    expect(readFileSync(file, "utf8")).toMatch(/^\/\/ saved by launch\n/);
    expect(snapshots[stopped]).toContain("main.report  main.go:6");
    expect(snapshots[stopped]).toMatch(/ ↷   ⤓   ⤒ +⏵   ∎ /);
    expect(snapshots[nerd]).toMatch(/ \uead6   \uead4   \uead5 +\ueacf   \uf04d /);
    expect(snapshots[caller]).toContain("Ln 12, Col 1");
    expect(snapshots[stopped]).toMatch(/▶ +6 +fmt\.Println/);
    expect(snapshots[caller]).toMatch(/▶ +6 +fmt\.Println/);
    expect(snapshots[stepped]).toMatch(/⦁ +6 +fmt\.Println/);
    expect(snapshots[stepped]).toMatch(/▶ +7 +local\+\+/);
    expect(snapshots[output]).toContain("[debug:Example] done 43");
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}, 70000);
