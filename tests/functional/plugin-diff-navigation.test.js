import { describe, it, expect, afterEach } from "vitest";
import * as tui from "./tui.js";
import { createTempDir, createTempFile, cleanupDir } from "./helpers.js";
import { writeFileSync } from "node:fs";
import { join } from "node:path";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

function writePlugin(dir, name, lua) {
  const path = join(dir, name);
  writeFileSync(path, lua, "utf8");
  return path;
}

// Reports editor.diff() on the status bar so the assertion reads real state
// from the running binary rather than a mock.
const REPORT_PLUGIN = `
  local ttt = require("ttt")
  local editor = require("ttt.editor")
  ttt.register({
    commands = {
      { id = "test.report", title = "Test Report", handler = function()
          local d = editor.diff()
          if not d then
            ttt.set_status_item("right", "probe", "DIFF=nil")
            return
          end
          ttt.set_status_item("right", "probe",
            "DIFF=n" .. #d.hunks .. " cur=" .. d.current_hunk ..
            " k1=" .. tostring(d.hunks[1] and d.hunks[1].kind))
        end
      },
      { id = "test.next", title = "Test Next", handler = function()
          local d = editor.diff()
          if not d then return end
          local n = d.current_hunk + 1
          if n > #d.hunks then n = 1 end
          editor.go_to_diff_hunk(n)
          local after = editor.diff()
          ttt.set_status_item("right", "probe", "DIFF cur=" .. after.current_hunk)
        end
      },
    },
  })
`;

const TWO_HUNK_DIFF = `diff --git a/f.txt b/f.txt
--- a/f.txt
+++ b/f.txt
@@ -1,6 +1,6 @@
 line 1
 line 2
-line 3
+CHANGED 3
 line 4
 line 5
 line 6
@@ -20,4 +20,3 @@
 line 20
-line 21
 line 22
 line 23
`;

describe("editor.diff() on a read-only diff tab", () => {
  it("enumerates change blocks and walks them", () => {
    dir = createTempDir();
    const file = createTempFile(dir, "test.txt", "hello\n");
    const plugin = writePlugin(
      dir,
      "test.lua",
      REPORT_PLUGIN +
        `
      ttt.set_timeout(400, function()
        ttt.open_changes({ id = "probe", title = "Probe", diff = [=[${TWO_HUNK_DIFF}]=] })
      end)
    `,
    );

    tui.start("--plugin", plugin, file);
    // wait-for proves the diff tab is up before the probe reads it.
    tui.waitFor("Probe");
    tui.exec("Test Report");
    const before = tui.snapshot();
    tui.exec("Test Next");
    const afterFirst = tui.snapshot();
    tui.exec("Test Next");
    const afterSecond = tui.snapshot();
    const { snapshots } = tui.run();

    expect(snapshots[before]).toContain("DIFF=n2 cur=0 k1=modified");
    expect(snapshots[afterFirst]).toContain("DIFF cur=1");
    expect(snapshots[afterSecond]).toContain("DIFF cur=2");
  });
});
