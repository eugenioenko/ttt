package app

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
)

func TestAppTerminalSpecPassesExecutableAndOrderedArgs(t *testing.T) {
	app := config.Application{Name: "viewer", Command: `C:\Program Files\viewer.exe`, Args: []string{"--mode", "preview", "{file}", "--readonly", "{dir}", "{workspace}"}, Cwd: "{dir}", Location: "panel", KeepOpen: true}
	file := `C:\it's {dir}\文 $x & 100%.txt`
	spec := appTerminalSpec(app, file, "folder", "root")
	want := []string{"--mode", "preview", file, "--readonly", "folder", "root"}
	if spec.Shell != app.Command || !slices.Equal(spec.Args, want) || spec.Cwd != "folder" || spec.InEditor || !spec.KeepOpen || spec.Label != "viewer" || spec.PaddingX == nil || *spec.PaddingX != 1 {
		t.Fatalf("spec=%+v", spec)
	}
	app.Args = nil
	if spec := appTerminalSpec(app, "", "folder", "root"); spec.Shell != app.Command || len(spec.Args) != 0 {
		t.Fatalf("bare command=%+v", spec)
	}
}

func TestAppCommandPassesLiteralPathsToNativeProgram(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{`C:\it's {dir}\文 $x & 100%.txt`, `C:\a space\folder`, `C:\workspace`}
	if runtime.GOOS != "windows" {
		paths = []string{`/tmp/it's {dir}/文 "$x"; & 100%.txt`, "/tmp/a space/folder", "/tmp/workspace"}
	}
	for _, hasFile := range []bool{true, false} {
		t.Run(fmt.Sprintf("file=%t", hasFile), func(t *testing.T) {
			app := config.Application{Command: exe, Args: []string{"-test.run=^TestAppCommandArgumentsHelper$", "--", "--new-window", "{file}", "{dir}", "{workspace}"}}
			file := paths[0]
			want := append([]string{"--new-window"}, paths...)
			if !hasFile {
				file = ""
				want = append([]string{"--new-window"}, paths[1:]...)
			}
			spec := appTerminalSpec(app, file, paths[1], paths[2])
			out := filepath.Join(t.TempDir(), "args.json")
			cmd := exec.Command(spec.Shell, spec.Args...)
			cmd.Env = append(os.Environ(), "TTT_APP_TEST_HELPER=1", "TTT_APP_TEST_OUT="+out)
			if data, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("launch: %v\n%s", err, data)
			}
			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, want) {
				t.Fatalf("arguments=%q,want %q", got, want)
			}
		})
	}
}

func TestAppCommandArgumentsHelper(t *testing.T) {
	if os.Getenv("TTT_APP_TEST_HELPER") != "1" {
		return
	}
	index := slices.Index(os.Args, "--")
	if index < 0 {
		t.Fatal("missing arguments")
	}
	data, err := json.Marshal(os.Args[index+1:])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("TTT_APP_TEST_OUT"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAppsSearchDebouncesLatestAndInvalidatesOnSettingsAndClose(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.Reg = command.NewRegistry()
	a.Settings.Applications = []config.Application{{Name: "Alpha", Command: "echo"}, {Name: "Beta", Command: "echo"}}
	a.syncApps()
	defer a.CloseTasks()
	clock := gitGutterTestClock{now: time.Unix(0, 0)}
	var work, delivery []func()
	a.tasks.post = func(fn func()) error { delivery = append(delivery, fn); return nil }
	a.appsSearch = NewTask[[]config.Application](a, asynctask.TaskOptions{Delay: appsSearchDelay, Clock: &clock, Run: func(fn func()) { work = append(work, fn) }})
	a.Apps.SearchInput.Config.OnChange("alpha")
	clock.advance(appsSearchDelay - time.Millisecond)
	a.Apps.SearchInput.Config.OnChange("  BEtA  ")
	clock.advance(appsSearchDelay - time.Millisecond)
	if len(work) != 0 {
		t.Fatal("search did not debounce")
	}
	clock.advance(time.Millisecond)
	if len(work) != 1 {
		t.Fatalf("work = %d", len(work))
	}
	work[0]()
	if len(delivery) != 1 {
		t.Fatalf("delivery = %d", len(delivery))
	}
	delivery[0]()
	if nodes := a.Apps.Tree.Config.Items; len(nodes) != 1 || nodes[0].ID != "Beta" {
		t.Fatalf("matches = %+v", nodes)
	}
	a.scheduleAppsSearch("Alpha")
	clock.advance(appsSearchDelay)
	work[1]()
	a.Settings.Applications = []config.Application{{Name: "New", Command: "echo"}}
	a.syncApps()
	delivery[1]()
	if nodes := a.Apps.Tree.Config.Items; len(nodes) != 1 || nodes[0].ID != "New" {
		t.Fatal("old search overwrote reloaded settings")
	}
	a.scheduleAppsSearch("missing")
	clock.advance(appsSearchDelay)
	work[2]()
	a.CloseTasks()
	delivery[2]()
	if len(a.Apps.Tree.Config.Items) != 1 {
		t.Fatal("closed search applied")
	}
	a.Apps.Filter("missing")
	if len(a.Apps.Tree.Config.Items) != 0 {
		t.Fatal("unmatched query retained apps")
	}
	a.Apps.Filter("")
	if len(a.Apps.Tree.Config.Items) != 1 {
		t.Fatal("clearing search did not restore apps")
	}
}

func TestAppsOpenWithMenuOnlyIncludesEnabledApps(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.Reg = command.NewRegistry()
	a.Settings.Applications = []config.Application{{Name: "enabled", Command: "echo", OpenWith: true}, {Name: "hidden", Command: "echo"}}
	a.syncApps()
	menu := a.appsOpenWithMenu()
	if len(menu.Submenu) != 3 || !strings.Contains(menu.Submenu[0].Label, "enabled") {
		t.Fatalf("menu = %+v", menu)
	}
	a.Settings.Applications = nil
	a.syncApps()
	if _, ok := a.Reg.Get("apps.openWith:enabled"); ok {
		t.Fatal("removed app kept Open with command")
	}
}

func TestAppsSearchSubmitUsesCurrentQueryBeforeDebounce(t *testing.T) {
	p := NewAppsPanel()
	p.SetApps([]config.Application{{Name: "Alpha"}, {Name: "Beta"}})
	var launched string
	p.OnLaunch = func(name string) { launched = name }
	p.SearchInput.Config.OnSubmit("beta")
	if launched != "Beta" {
		t.Fatalf("launched %q instead of current search result", launched)
	}
}

func TestAppsReloadPreservesShortcutLabels(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.Reg = command.NewRegistry()
	a.Settings.Applications = []config.Application{{Name: "viewer", Command: "echo", OpenWith: true}}
	a.syncApps()
	ids := []string{"apps.run:viewer", "apps.openWith:viewer", "apps.remove:viewer"}
	for _, id := range ids {
		a.Reg.SetShortcut(id, "Ctrl+K X")
	}
	a.Settings.Applications = append(a.Settings.Applications, config.Application{Name: "other", Command: "echo"})
	a.syncApps()
	for _, id := range ids {
		cmd, _ := a.Reg.Get(id)
		if cmd.Shortcut != "Ctrl+K X" {
			t.Fatalf("%s lost shortcut", id)
		}
	}
	cmd, _ := a.Reg.Get("apps.run:other")
	if cmd.Shortcut != "" {
		t.Fatal("new command inherited another app's shortcut")
	}
	for _, cmd := range a.Reg.List() {
		if strings.HasPrefix(cmd.ID, "apps.openWith:") {
			t.Fatal("Explorer-only command exposed in palette")
		}
	}
}

func TestAppArgsExpandOnceAndTrackFile(t *testing.T) {
	app := config.Application{Command: "{workspace}/viewer", Args: []string{"--file={file}", "{dir}"}, Cwd: "{dir}"}
	spec := appTerminalSpec(app, "it's {dir}.md", "folder", "root")
	if spec.Shell != "root/viewer" || !slices.Equal(spec.Args, []string{"--file=it's {dir}.md", "folder"}) || spec.Cwd != "folder" {
		t.Fatalf("spec=%+v", spec)
	}
	if !appUsesFile(app) {
		t.Fatal("file placeholder in args was ignored")
	}
	missing := appTerminalSpec(app, "", "folder", "root")
	if !slices.Equal(missing.Args, []string{"folder"}) {
		t.Fatalf("unresolved file argument was retained: %v", missing.Args)
	}
	app.Args = []string{}
	if appUsesFile(app) {
		t.Fatal("empty args require a file")
	}
	spec = appTerminalSpec(app, "", "folder", "root")
	if spec.Shell != "root/viewer" || len(spec.Args) != 0 {
		t.Fatalf("empty direct args=%+v", spec)
	}
}

func TestAppsCommandTextUsesNameAsFallbackAndKeepsIdentity(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.Reg = command.NewRegistry()
	var builtinRan bool
	a.Reg.Register(command.Command{ID: "existing.preview", Title: "App: Preview Markdown", Handler: func() { builtinRan = true }})
	for _, tc := range []struct{ text, want string }{{"Preview Markdown", "Preview Markdown"}, {"  Preview Markdown  ", "Preview Markdown"}, {"", "edamame"}, {"  ", "edamame"}} {
		a.Settings.Applications = []config.Application{{Name: "edamame", Command: "edamame {file}", CommandText: tc.text}}
		a.syncApps()
		cmd, ok := a.Reg.Get("apps.run:edamame")
		if !ok || cmd.Title != "App: "+tc.want {
			t.Fatalf("command text %q: %+v", tc.text, cmd)
		}
		if !slices.Contains(cmd.Keywords, "edamame") {
			t.Fatal("custom palette title lost app-name search")
		}
		if node := a.Apps.Tree.Config.Items[0]; node.ID != "edamame" || node.Label != "edamame" {
			t.Fatalf("palette title changed app identity: %+v", node)
		}
	}
	a.Reg.Execute("existing.preview")
	if !builtinRan {
		t.Fatal("custom palette title replaced another command")
	}
}
