package e2e

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
)

func applyApps(h *testHarness, apps ...config.Application) {
	h.t.Helper()
	s := *h.app.Settings
	s.Applications = apps
	h.app.ApplySettings(s)
	h.redraw()
}

func waitForFile(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			return string(data)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s was never written", path)
	return ""
}

func TestAppsTabFollowsSettings(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	if !h.app.Sidebar.HasPanel("apps") {
		t.Fatal("the empty Apps tab must remain discoverable")
	}

	applyApps(h,
		config.Application{Name: "viewer", Command: "true"},
		config.Application{Name: "", Command: "true"},
		config.Application{Name: "viewer", Command: "false"},
	)
	if !h.app.Sidebar.HasPanel("apps") {
		t.Fatal("configuring an app must add the Apps tab")
	}
	if _, ok := h.reg.Get("apps.run:viewer"); !ok {
		t.Fatal("each app must get a run command")
	}
	if n := len(h.app.Apps.Tree.Config.Items); n != 1 {
		t.Fatalf("apps listed = %d, want 1 (invalid and duplicate entries dropped)", n)
	}

	applyApps(h)
	if !h.app.Sidebar.HasPanel("apps") {
		t.Fatal("removing every app must keep the empty Apps tab")
	}
	if _, ok := h.reg.Get("apps.run:viewer"); ok {
		t.Fatal("a removed app's command must be unregistered")
	}
}

func TestLaunchAppQuotesPlaceholdersAndReusesItsTab(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command")
	}
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	file := filepath.Join(h.dir, "it's here.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(h.dir, "out.txt")
	applyApps(h, config.Application{
		Name:     "echo",
		Command:  "printf '%s|%s' {file} {dir} > " + out + "; sleep 30",
		KeepOpen: true,
	})

	if err := h.app.EditorGroup.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	h.exec("apps.run:echo")
	if len(h.app.EditorTerminals) != 1 {
		t.Fatalf("editor terminals = %d, want 1", len(h.app.EditorTerminals))
	}
	defer h.app.EditorTerminals[0].Term.Close()
	if want := file + "|" + h.dir; strings.TrimSpace(waitForFile(t, out)) != want {
		t.Fatalf("command saw %q, want %q", waitForFile(t, out), want)
	}

	h.exec("apps.run:echo")
	if len(h.app.EditorTerminals) != 1 {
		t.Fatal("running an app again must focus its tab, not open a second one")
	}
	if h.app.EditorGroup.ActiveTerminalTab() != h.app.EditorTerminals[0].Tab {
		t.Fatal("running an app again must focus its tab")
	}
}

func TestLaunchAppFromAnotherAppTabUsesTheLastFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command")
	}
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	file := filepath.Join(h.dir, "alpha.txt")
	out := filepath.Join(h.dir, "out.txt")
	applyApps(h,
		config.Application{Name: "first", Command: "sleep 30"},
		config.Application{Name: "second", Command: "printf '%s' {file} > " + out + "; sleep 30"},
	)
	if err := h.app.EditorGroup.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	h.exec("apps.run:first")
	h.exec("apps.run:second")
	for _, tt := range h.app.EditorTerminals {
		defer tt.Term.Close()
	}
	if len(h.app.EditorTerminals) != 2 {
		t.Fatalf("editor terminals = %d, want 2", len(h.app.EditorTerminals))
	}
	if got := waitForFile(t, out); got != file {
		t.Fatalf("{file} = %q, want the file active before the first app, %q", got, file)
	}
}

func TestDirFallsBackToWorkspaceWithoutAFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command")
	}
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	out := filepath.Join(h.dir, "out.txt")
	applyApps(h, config.Application{Name: "browse", Command: "printf '%s' {dir} > " + out + "; sleep 30"})
	h.app.EditorGroup.CloseAllTabs()
	h.exec("apps.run:browse")
	if len(h.app.EditorTerminals) != 1 {
		t.Fatal("an app using only {dir} must launch without an open file")
	}
	defer h.app.EditorTerminals[0].Term.Close()
	if got, want := waitForFile(t, out), h.app.Workspace.Primary(); got != want {
		t.Fatalf("{dir} = %q, want the workspace %q", got, want)
	}
}

func TestEmptyAppsDialogCanSaveAndCancel(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.exec("sidebar.apps")
	h.assertContains("Add App")
	if !h.app.Root.HasOverlay() {
		t.Fatal("empty apps must open the add dialog")
	}
	adapter := h.app.Root.TopOverlayWidget().(*ui.WidgetAdapter)
	if h.app.Root.Focused != adapter {
		t.Fatal("dialog lost keyboard focus")
	}
	h.pressKey(tcell.KeyEscape, 0)
	if h.app.Root.HasOverlay() || len(h.app.Settings.Applications) != 0 {
		t.Fatal("cancel added an app")
	}
	h.exec("sidebar.apps")
	adapter = h.app.Root.TopOverlayWidget().(*ui.WidgetAdapter)
	dialog := adapter.Inner().(*widgets.DialogWidget)
	content := dialog.Content.(*widgets.VStackWidget)
	input := func(index int) *widgets.InputWidget {
		return content.Children[index].(*widgets.HStackWidget).Children[1].(*widgets.InputWidget)
	}
	dialog.Buttons[1].Handler()
	h.redraw()
	h.assertContains("Enter a name and command")
	for _, r := range "viewer" {
		h.pressRune(r)
	}
	h.pressKey(tcell.KeyTab, 0)
	for _, r := range "echo {file}" {
		h.pressRune(r)
	}
	input(4).SetText("V")
	input(5).SetText("Preview Markdown")
	content.Children[8].(*widgets.CheckboxWidget).Config.Checked = true
	dialog.Buttons[1].Handler()
	h.redraw()
	if h.app.Root.HasOverlay() {
		t.Fatal("successful save kept the dialog open")
	}
	apps := config.LoadSettings().Applications
	if len(apps) != 1 || apps[0].CommandText != "Preview Markdown" || apps[0].Name != "viewer" || apps[0].Command != "echo {file}" || apps[0].Icon != "V" || !apps[0].OpenWith || !apps[0].KeepOpen {
		t.Fatalf("saved apps: %+v", apps)
	}
	if _, ok := h.reg.Get("apps.openWith:viewer"); !ok {
		t.Fatal("save did not register Open with")
	}
}

func TestExplorerOpenWithUsesClickedPathAndReusesOnlyThatTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command")
	}
	h := newTestHarness(t, 110, 32)
	defer h.stop()
	active := filepath.Join(h.dir, "alpha.txt")
	target := filepath.Join(h.dir, "it's {dir} here.txt")
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(h.dir, "out.txt")
	applyApps(h, config.Application{Name: "viewer", Command: "printf '%s|%s|%s' {file} {dir} {workspace} > " + out + "; sleep 30", OpenWith: true, Icon: "V"})
	if err := h.app.EditorGroup.OpenFile(active); err != nil {
		t.Fatal(err)
	}
	h.app.Explorer.OnRightClick(&widgets.TreeNode{ID: target}, 3, 6)
	h.redraw()
	h.assertContains("Open with")
	h.pressKey(tcell.KeyDown, 0)
	h.pressKey(tcell.KeyRight, 0)
	h.assertContains("V viewer")
	h.pressKey(tcell.KeyEnter, 0)
	defer func() {
		for _, tt := range h.app.EditorTerminals {
			tt.Term.Close()
		}
	}()
	if got, want := waitForFile(t, out), target+"|"+h.dir+"|"+h.dir; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if h.app.ExplorerContextNode != nil {
		t.Fatal("retained context target")
	}
	if h.app.EditorGroup.RecentDocumentPath() != active {
		t.Fatal("Open with changed the current file")
	}
	h.app.ExplorerContextNode = &widgets.TreeNode{ID: target}
	h.exec("apps.openWith:viewer")
	if len(h.app.EditorTerminals) != 1 {
		t.Fatal("same target must reuse its tab")
	}
	h.app.ExplorerContextNode = &widgets.TreeNode{ID: active}
	h.exec("apps.openWith:viewer")
	if len(h.app.EditorTerminals) != 2 {
		t.Fatal("different target must start its own app")
	}
	s := *h.app.Settings
	s.Appearance.Icons = config.IconsNerdFont
	h.app.ApplySettings(s)
	h.redraw()
	tabs := h.app.EditorGroup.TabBar.Tabs
	if tabs[len(tabs)-1].Icon != "V" || h.app.Apps.Tree.Config.Items[0].Icon != "V" {
		t.Fatalf("custom icon changed with font mode: %+v", tabs)
	}
}

func TestExplorerOpenWithCanAddAnAppWhenEmpty(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.app.Explorer.OnRightClick(&widgets.TreeNode{ID: filepath.Join(h.dir, "alpha.txt")}, 3, 6)
	h.redraw()
	h.pressKey(tcell.KeyDown, 0)
	h.pressKey(tcell.KeyRight, 0)
	h.pressKey(tcell.KeyEnter, 0)
	h.assertContains("Add App")
	dialog := h.app.Root.TopOverlayWidget().(*ui.WidgetAdapter).Inner().(*widgets.DialogWidget)
	if !dialog.Content.(*widgets.VStackWidget).Children[8].(*widgets.CheckboxWidget).Config.Checked {
		t.Fatal("adding from Open with must enable Explorer integration")
	}
}

func TestAddAppDuplicateAndSaveFailureLeaveSettingsUntouched(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	applyApps(h, config.Application{Name: "existing", Command: "echo"})
	h.app.AddApp()
	dialog := h.app.Root.TopOverlayWidget().(*ui.WidgetAdapter).Inner().(*widgets.DialogWidget)
	content := dialog.Content.(*widgets.VStackWidget)
	name := content.Children[0].(*widgets.HStackWidget).Children[1].(*widgets.InputWidget)
	command := content.Children[1].(*widgets.HStackWidget).Children[1].(*widgets.InputWidget)
	name.SetText("existing")
	command.SetText("echo")
	dialog.Buttons[1].Handler()
	h.redraw()
	h.assertContains("An app with this name already exists")
	name.SetText("new")
	if err := os.Mkdir(config.ConfigFilePath("settings.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	dialog.Buttons[1].Handler()
	h.redraw()
	h.assertContains("Could not save app")
	if !h.app.Root.HasOverlay() || len(h.app.Settings.Applications) != 1 {
		t.Fatal("failed save changed settings or dismissed dialog")
	}
}

func TestOpenCurrentFileWithSearchIgnoresExplorerAndSharesTargetTab(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX test program")
	}
	h := newTestHarness(t, 110, 32)
	defer h.stop()
	out := filepath.Join(h.dir, "current.txt")
	applyApps(h,
		config.Application{Name: "edamame", Command: "/bin/sh", Args: []string{"-c", `printf '%s' "$1" > "$2"; sleep 30`, "sh", "{file}", out}},
		config.Application{Name: "unrelated", Command: "false"},
	)
	h.exec("apps.openCurrentFileWith")
	if !h.app.Root.HasOverlay() {
		t.Fatal("no-file picker did not open")
	}
	h.pressKey(tcell.KeyEscape, 0)
	file := filepath.Join(h.dir, "alpha.txt")
	if err := h.app.EditorGroup.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(h.dir, "beta.txt")
	h.app.Explorer.Tree.SelectByID(other)
	h.app.OpenCommandPalette(false)
	h.redraw()
	for _, r := range "Open Current File With" {
		h.pressRune(r)
	}
	h.pressKey(tcell.KeyEnter, 0)
	h.assertContains("Open Current File With")
	h.assertContains("edamame")
	h.assertContains("unrelated")
	for _, r := range "edam" {
		h.pressRune(r)
	}
	if strings.Contains(h.screenText(), "unrelated") {
		t.Fatal("search did not filter other apps")
	}
	h.pressKey(tcell.KeyEnter, 0)
	if len(h.app.EditorTerminals) != 1 {
		t.Fatal("picker did not launch its selected app")
	}
	defer h.app.EditorTerminals[0].Term.Close()
	if got := waitForFile(t, out); got != file {
		t.Fatalf("opened %q instead of current document %q", got, file)
	}
	if h.app.EditorGroup.RecentDocumentPath() != file {
		t.Fatal("picker changed current document")
	}
	s := *h.app.Settings
	s.Applications = slices.Clone(s.Applications)
	s.Applications[0].OpenWith = true
	h.app.ApplySettings(s)
	h.app.ExplorerContextNode = &widgets.TreeNode{ID: file}
	h.exec("apps.openWith:edamame")
	if len(h.app.EditorTerminals) != 1 {
		t.Fatal("Explorer did not reuse the app/current-file terminal")
	}
}

func TestOpenCurrentFileWithCanCancelAndConfigureEmptyList(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	if err := h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	h.exec("file.new")
	h.exec("apps.openCurrentFileWith")
	if !h.app.Root.HasOverlay() {
		t.Fatal("untitled document did not open Add App")
	}
	h.pressKey(tcell.KeyEscape, 0)
	if err := h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	h.exec("apps.openCurrentFileWith")
	h.assertContains("Add App")
	dialog := h.app.Root.TopOverlayWidget().(*ui.WidgetAdapter).Inner().(*widgets.DialogWidget)
	if !dialog.Content.(*widgets.VStackWidget).Children[8].(*widgets.CheckboxWidget).Config.Checked {
		t.Fatal("new app did not enable Open with")
	}
	h.pressKey(tcell.KeyEscape, 0)
	applyApps(h, config.Application{Name: "edamame", Command: "echo", Args: []string{"{file}"}, OpenWith: true})
	h.exec("apps.openCurrentFileWith")
	h.pressKey(tcell.KeyEscape, 0)
	if h.app.Root.HasOverlay() || len(h.app.EditorTerminals) != 0 {
		t.Fatal("cancelling picker launched an app")
	}
	h.app.AddApp()
	dialog = h.app.Root.TopOverlayWidget().(*ui.WidgetAdapter).Inner().(*widgets.DialogWidget)
	content := dialog.Content.(*widgets.VStackWidget)
	input := func(i int) *widgets.InputWidget {
		return content.Children[i].(*widgets.HStackWidget).Children[1].(*widgets.InputWidget)
	}
	input(0).SetText("viewer")
	input(1).SetText("viewer")
	input(2).SetText(`"{file}"`)
	dialog.Buttons[1].Handler()
	h.redraw()
	h.assertContains("Arguments must be a JSON array of strings")
	input(2).SetText(`["{file}"]`)
	dialog.Buttons[1].Handler()
	if h.app.Root.HasOverlay() {
		t.Fatal("valid arguments kept add dialog open")
	}
	apps := config.LoadSettings().Applications
	if len(apps) != 2 || !slices.Equal(apps[1].Args, []string{"{file}"}) {
		t.Fatalf("saved args=%+v", apps)
	}
}

func TestCustomAppPaletteEntryOpensEachCurrentDocument(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX test program")
	}
	h := newTestHarness(t, 110, 32)
	defer h.stop()
	defer func() {
		for _, entry := range h.app.EditorTerminals {
			entry.Term.Close()
		}
	}()
	applyApps(h, config.Application{Name: "edamame", Command: "/bin/sh", Args: []string{"-c", `printf '%s\n' "$1"; sleep 30`, "sh", "{file}"}, CommandText: "Preview Markdown"})
	for i, name := range []string{"alpha.txt", "beta.txt"} {
		file := filepath.Join(h.dir, name)
		if err := h.app.EditorGroup.OpenFile(file); err != nil {
			t.Fatal(err)
		}
		h.app.OpenCommandPalette(false)
		h.redraw()
		for _, r := range "Preview Markdown" {
			h.pressRune(r)
		}
		h.pressKey(tcell.KeyEnter, 0)
		if len(h.app.EditorTerminals) != i+1 {
			t.Fatal("custom palette command reused another document's app")
		}
		entry := h.app.EditorTerminals[i]
		waitForRawTail(t, entry.Term, file)
		if entry.Label != "edamame" {
			t.Fatalf("app label=%q", entry.Label)
		}
	}
	h.exec("apps.run:edamame")
	if len(h.app.EditorTerminals) != 2 {
		t.Fatal("same app/file opened another terminal")
	}
	h.exec("file.new")
	h.exec("apps.run:edamame")
	if len(h.app.EditorTerminals) != 3 {
		t.Fatal("untitled document did not start the app without a file")
	}
}

func TestAppWithoutFileOmitsOnlyFileArgsAndKeepsStaticFlags(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX test program")
	}
	h := newTestHarness(t, 110, 32)
	defer h.stop()
	defer func() {
		for _, entry := range h.app.EditorTerminals {
			entry.Term.Close()
		}
	}()
	applyApps(h, config.Application{Name: "ttt", Command: "/bin/sh", Args: []string{"-c", `printf 'argc=%s|flag=%s|file=%s' "$#" "$1" "$2"; sleep 30`, "sh", "--new-window", "{file}"}})
	file := filepath.Join(h.dir, "alpha.txt")
	if err := h.app.EditorGroup.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	h.exec("file.new")
	h.exec("apps.run:ttt")
	if len(h.app.EditorTerminals) != 1 {
		t.Fatal("app did not launch without a file")
	}
	waitForRawTail(t, h.app.EditorTerminals[0].Term, "argc=1|flag=--new-window|file=")
	if err := h.app.EditorGroup.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	h.exec("apps.run:ttt")
	if len(h.app.EditorTerminals) != 2 {
		t.Fatal("resolved file did not get its own terminal")
	}
	waitForRawTail(t, h.app.EditorTerminals[1].Term, "argc=2|flag=--new-window|file="+file)
	h.exec("file.new")
	h.exec("apps.openCurrentFileWith")
	h.pressKey(tcell.KeyEnter, 0)
	if len(h.app.EditorTerminals) != 2 || h.app.EditorGroup.ActiveTerminalTab() != h.app.EditorTerminals[0].Tab {
		t.Fatal("no-file picker did not reuse the app started without a file")
	}
}
