package app

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
)

func TestAppTerminalSpecUsesPlatformShell(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		t.Run(goos, func(t *testing.T) {
			file := `C:\it's {dir}\文 $x & 100%.txt`
			app := config.Application{Name: "viewer", Command: "viewer {file} {dir} {workspace}", Cwd: "{dir}", Location: "panel", KeepOpen: true}
			spec := appTerminalSpec(app, file, "folder", "root", goos)
			if spec.Cwd != "folder" || spec.InEditor || !spec.KeepOpen || spec.Label != "viewer" {
				t.Fatalf("spec = %+v", spec)
			}
			script := spec.Args[len(spec.Args)-1]
			if goos == "windows" {
				if spec.Shell != "powershell.exe" || !slices.Equal(spec.Args[:3], []string{"-NoLogo", "-NoProfile", "-EncodedCommand"}) {
					t.Fatalf("Windows spec = %+v", spec)
				}
				data, err := base64.StdEncoding.DecodeString(script)
				if err != nil {
					t.Fatal(err)
				}
				units := make([]uint16, len(data)/2)
				for i := range units {
					units[i] = binary.LittleEndian.Uint16(data[2*i:])
				}
				script = string(utf16.Decode(units))
				if script != `viewer "${env:TTT_APP_FILE}" "${env:TTT_APP_DIR}" "${env:TTT_APP_WORKSPACE}"` {
					t.Fatalf("script = %q", script)
				}
			} else if spec.Shell != "/bin/sh" || script != `viewer "${TTT_APP_FILE}" "${TTT_APP_DIR}" "${TTT_APP_WORKSPACE}"` {
				t.Fatalf("POSIX spec = %+v", spec)
			}
			if !slices.Contains(spec.Env, "TTT_APP_FILE="+file) {
				t.Fatalf("path not preserved: %v", spec.Env)
			}
		})
	}
}

func TestAppCommandPassesLiteralPathsToNativeProgram(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := `"$TTT_APP_TEST_EXE" -test.run=^TestAppCommandArgumentsHelper$ -- {file} {dir} {workspace}`
	if runtime.GOOS == "windows" {
		command = `& $env:TTT_APP_TEST_EXE -test.run=^TestAppCommandArgumentsHelper$ -- {file} {dir} {workspace}`
	}
	paths := []string{`C:\it's {dir}\文 $x & 100%.txt`, `C:\a space\folder`, `C:\workspace`}
	if runtime.GOOS != "windows" {
		paths = []string{`/tmp/it's {dir}/文 "$x"; & 100%.txt`, "/tmp/a space/folder", "/tmp/workspace"}
	}
	spec := appTerminalSpec(config.Application{Command: command}, paths[0], paths[1], paths[2], runtime.GOOS)
	out := filepath.Join(t.TempDir(), "args.json")
	cmd := exec.Command(spec.Shell, spec.Args...)
	cmd.Env = append(os.Environ(), spec.Env...)
	cmd.Env = append(cmd.Env, "TTT_APP_TEST_HELPER=1", "TTT_APP_TEST_EXE="+exe, "TTT_APP_TEST_OUT="+out)
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("shell: %v\n%s", err, data)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, paths) {
		t.Fatalf("arguments = %q, want %q", got, paths)
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
