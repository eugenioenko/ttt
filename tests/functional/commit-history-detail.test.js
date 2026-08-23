import { afterEach, describe, expect, it } from "vitest";
import * as tui from "./tui.js";
import { cleanupDir, createGitRepo, createTempDir, createTempFile, git } from "./helpers.js";

let dir;

afterEach(() => {
  tui.kill();
  if (dir) cleanupDir(dir);
});

describe("commit history detail", () => {
  it("opens a selected commit file as its own diff", () => {
    dir = createGitRepo(createTempDir());
    createTempFile(dir, "selected-detail.txt", "selected commit content\n");
    git(dir, "add", "-A");
    git(dir, "commit", "-qm", "selected detail");

    tui.start(dir);
    tui.pressChord("ctrl+k", "c");
    tui.waitFor("selected detail");
    tui.press("tab");
    tui.press("tab");
    tui.press("down");
    tui.press("right");
    tui.waitFor("selected-detail.txt");
    tui.press("down");
    tui.exec("Git: Open Changes Only");
    tui.waitFor("selected-detail.txt @");
    const opened = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[opened]).toMatch(/selected-detail\.txt @ [0-9a-f]{7,}/);
    expect(snapshots[opened]).toContain("selected commit content");
  });

  it("appends bounded history pages only from the explicit sentinel", () => {
    dir = createGitRepo(createTempDir());
    for (let index = 1; index <= 60; index++) {
      git(dir, "commit", "--allow-empty", "-qm", `paged ${String(index).padStart(2, "0")}`);
    }

    tui.start(dir);
    tui.pressChord("ctrl+k", "c");
    tui.waitFor("paged 60");
    tui.press("tab");
    tui.press("tab");
    for (let index = 0; index < 11; index++) tui.press("down");
    const sentinel = tui.snapshot();
    tui.press("enter");
    tui.waitFor("paged 50");
    const firstPage = tui.snapshot();
    for (let index = 0; index < 50; index++) tui.press("down");
    tui.press("enter");
    tui.waitFor("initial commit");
    const lastPage = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[sentinel]).toContain("Load older commits…");
    expect(snapshots[firstPage]).toContain("paged 50");
    expect(snapshots[lastPage]).toContain("initial commit");
    expect(snapshots[lastPage]).not.toContain("Load older commits…");
  });

  it("opens one read-only document with metadata and every changed file", () => {
    dir = createGitRepo(createTempDir());
    createTempFile(dir, "first-detail.txt", "first old\n");
    createTempFile(dir, "second-detail.txt", "second old\n");
    git(dir, "add", "-A");
    git(dir, "commit", "-qm", "detail subject", "-m", "Full detail body.");

    tui.start(dir);
    tui.pressChord("ctrl+k", "c");
    tui.waitFor("detail subject");
    // Changes focus starts in the working tree. The next two stops are the
    // commit input and the responsive Commit History tree.
    tui.press("tab");
    tui.press("tab");
    tui.press("down");
    tui.press("enter");
    tui.waitFor("Full detail body.");
    const detail = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[detail]).toMatch(/Commit [0-9a-f]{7,}/);
    expect(snapshots[detail]).toContain("detail subject");
    expect(snapshots[detail]).toContain("Full detail body.");
    expect(snapshots[detail]).toMatch(/Authored [A-Z][a-z]{2} \d{1,2}, \d{4} at \d{1,2}:\d{2}:\d{2} [AP]M [+-]\d{4}/);
    expect(snapshots[detail]).toContain("first-detail.txt");
    expect(snapshots[detail]).toContain("first old");
    expect(snapshots[detail]).toContain("second-detail.txt");
    expect(snapshots[detail]).toContain("second old");
	});

	it("loads the whole typed file snapshot for Full File context", () => {
		dir = createGitRepo(createTempDir());
		const lines = Array.from({ length: 30 }, (_, index) => `unchanged line ${index + 1}`);
		createTempFile(dir, "context.txt", `${lines.join("\n")}\n`);
		git(dir, "add", "-A");
		git(dir, "commit", "-qm", "context base");
		lines[1] = "changed near top";
		lines[28] = "changed near bottom";
		createTempFile(dir, "context.txt", `${lines.join("\n")}\n`);
		git(dir, "add", "-A");
		git(dir, "commit", "-qm", "two distant edits");

		tui.start(dir);
		tui.pressChord("ctrl+k", "c");
		tui.waitFor("two distant edits");
		tui.press("tab");
		tui.press("tab");
		tui.press("down");
		tui.press("enter");
		tui.waitFor("changed near top");
		const changesOnly = tui.snapshot();
		tui.exec("Git: Show Full File");
		tui.waitFor("unchanged line 15");
		const fullFile = tui.snapshot();

		const { snapshots } = tui.run();
		expect(snapshots[changesOnly]).not.toContain("unchanged line 15");
		expect(snapshots[fullFile]).toContain("unchanged line 15");
	});

	it("changes the active commit detail from its right-click menus", () => {
		dir = createGitRepo(createTempDir());
		createTempFile(dir, "menu-detail.txt", "old detail line\n");
		git(dir, "add", "-A");
		git(dir, "commit", "-qm", "menu base");
		createTempFile(dir, "menu-detail.txt", "new detail line with a long wrapped suffix\n");
		git(dir, "add", "-A");
		git(dir, "commit", "-qm", "menu detail");

		tui.start(dir);
		tui.setSize(70, 16);
		tui.pressChord("ctrl+k", "c");
		tui.waitFor("menu detail");
		tui.press("tab");
		tui.press("tab");
		tui.press("down");
		tui.press("enter");
		tui.waitFor("Authored");
		tui.rclick(60, 12);
		const contentMenu = tui.snapshot();
		for (let index = 0; index < 4; index++) tui.press("down");
		tui.press("enter");
		tui.rclick(50, 2);
		const compactTabMenu = tui.snapshot();
		for (let index = 0; index < 8; index++) tui.press("down");
		tui.press("right");
		const tabMenu = tui.snapshot();

		const { snapshots } = tui.run();
		expect(snapshots[compactTabMenu]).toContain("Diff View");
		for (const label of ["Split", "Unified", "Changes Only", "Full File", "Wrap Lines"]) {
			expect(snapshots[contentMenu]).toContain(label);
			expect(snapshots[tabMenu]).toContain(label);
		}
		expect(snapshots[tabMenu]).toContain("✓ Unified");
	});

  it("uses shared Git bulk commands for commit-detail file groupings", () => {
    dir = createGitRepo(createTempDir());
    createTempFile(dir, "bulk-detail.txt", "visible detail line\n");
    git(dir, "add", "-A");
    git(dir, "commit", "-qm", "bulk detail");

    tui.start(dir);
    tui.pressChord("ctrl+k", "c");
    tui.waitFor("bulk detail");
    tui.press("tab");
    tui.press("tab");
    tui.press("down");
    tui.press("enter");
    tui.waitFor("visible detail line");
    const expanded = tui.snapshot();
    tui.exec("Git: Collapse All File Trees");
    const collapsed = tui.snapshot();
    tui.exec("Git: Expand All File Trees");
    const restored = tui.snapshot();
    tui.rclick(40, 12);
    const context = tui.snapshot();

    const { snapshots } = tui.run();
    expect(snapshots[expanded]).toContain("visible detail line");
    expect(snapshots[collapsed]).not.toContain("visible detail line");
    expect(snapshots[restored]).toContain("visible detail line");
    expect(snapshots[context]).toContain("Expand All Files");
    expect(snapshots[context]).toContain("Collapse All Files");
  });
});
