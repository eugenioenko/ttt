package update

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv("TTT_UPDATE_TEST_HELPER"); mode != "" {
		fmt.Println(strings.Repeat("old output\n", 1000))
		fmt.Println("command arguments:", strings.Join(os.Args[1:], " "))
		fmt.Fprintln(os.Stderr, "last command output")
		if mode == "fail" {
			os.Exit(31)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestPackageManagerApply(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		kind       Kind
		name, args string
	}{
		{Homebrew, "brew", "upgrade ttt"},
		{Scoop, "powershell.exe", "-NoProfile -NonInteractive -Command scoop update ttt"},
		{GoInstall, "go", "install github.com/eugenioenko/ttt/cmd/ttt@latest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			name := tc.name
			if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
				name += ".exe"
			}
			if err := os.WriteFile(filepath.Join(dir, name), data, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("TTT_UPDATE_TEST_HELPER", "success")
			if err := Apply(context.Background(), Release{Tag: "v2.0.0"}, Method{Kind: tc.kind}); err != nil {
				t.Fatal(err)
			}
			t.Setenv("TTT_UPDATE_TEST_HELPER", "fail")
			err := Apply(context.Background(), Release{Tag: "v2.0.0"}, Method{Kind: tc.kind})
			if err == nil || !strings.Contains(err.Error(), "command arguments: "+tc.args) || !strings.Contains(err.Error(), "last command output") || len(err.Error()) > 9000 {
				t.Fatalf("command error = %v", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := Apply(ctx, Release{}, Method{Kind: tc.kind}); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled command = %v", err)
			}
		})
	}
	for _, kind := range []Kind{Nix, Dev} {
		if err := Apply(context.Background(), Release{}, Method{Kind: kind}); err == nil {
			t.Fatalf("unsupported method %v updated", kind)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"dev", "v1.0.0", -1}, {"dev", "dev", 0}, {"v1.2.3", "1.2.3", 0},
		{"v1.10.0", "v1.9.9", 1}, {"1.0.0-rc.2", "1.0.0-rc.10", -1},
		{"1.0.0-rc.1", "1.0.0", -1}, {"1.0.0+build.2", "1.0.0+build.1", 0},
		{"1.0.0-alpha", "1.0.0-alpha.1", -1}, {"1.0.0-1", "1.0.0-alpha", -1},
		{"999999999999999999999.0.0", "2.0.0", 1},
	} {
		t.Run(tc.a+"/"+tc.b, func(t *testing.T) {
			got, err := CompareVersions(tc.a, tc.b)
			if err != nil || got != tc.want {
				t.Fatalf("comparison = %d, %v; want %d", got, err, tc.want)
			}
			reverse, err := CompareVersions(tc.b, tc.a)
			if err != nil || reverse != -tc.want {
				t.Fatalf("reverse = %d, %v", reverse, err)
			}
		})
	}
	for _, invalid := range []string{"", "1.2", "v01.2.3", "1.0.0-01", "1.0.0-", "other"} {
		if _, err := CompareVersions(invalid, "1.0.0"); err == nil {
			t.Errorf("accepted %q", invalid)
		}
	}
}

func TestIsDevelopmentVersion(t *testing.T) {
	for _, version := range []string{"dev", "v1.0.0-12-gabc", "v1.0.0-rc.1-12-gabc-dirty", "v1.0.0-dirty", "abc1234", ""} {
		if !IsDevelopmentVersion(version) {
			t.Errorf("%q was treated as a release", version)
		}
		if method, err := CurrentMethod(version); err != nil || method.Kind != Dev {
			t.Errorf("%q method = %+v, %v", version, method, err)
		}
	}
	for _, version := range []string{"1.0.0", "v1.0.0", "v1.0.0-rc.1", "v1.0.0+build.1"} {
		if IsDevelopmentVersion(version) {
			t.Errorf("%q was treated as a development build", version)
		}
	}
}

func TestAssetName(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "freebsd", "android", "windows"} {
		for _, arch := range []string{"amd64", "arm64"} {
			want := "ttt-" + goos + "-" + arch
			if goos == "windows" {
				want += ".exe"
			}
			if got := AssetName(goos, arch); got != want {
				t.Errorf("asset = %q, want %q", got, want)
			}
		}
	}
}

func TestDetectMethod(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOBIN", filepath.Join(root, "custom-bin"))
	t.Setenv("GOPATH", filepath.Join(root, "go"))
	info := &debug.BuildInfo{Main: debug.Module{Version: "v1.0.0"}}
	for _, tc := range []struct {
		path string
		info *debug.BuildInfo
		want Kind
	}{
		{"/opt/homebrew/Cellar/ttt/1.0/bin/ttt", nil, Homebrew},
		{"/home/linuxbrew/.linuxbrew/Cellar/ttt/1.0/bin/ttt", nil, Homebrew},
		{`C:\Users\me\scoop\apps\ttt\current\ttt.exe`, nil, Scoop},
		{"/nix/store/abc-ttt/bin/ttt", nil, Nix},
		{filepath.Join(root, "custom-bin", "ttt"), info, GoInstall},
		{filepath.Join(root, "go", "bin", "ttt"), info, GoInstall},
		{filepath.Join(root, "go", "bin", "ttt"), nil, Binary},
		{filepath.Join(root, "go", "bin", "ttt"), &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, Binary},
		{"/usr/local/bin/ttt", info, Binary},
	} {
		if got := DetectMethod(tc.path, tc.info); got.Kind != tc.want {
			t.Errorf("%s: method = %v, want %v", tc.path, got.Kind, tc.want)
		}
	}
	method, err := CurrentMethod("dev")
	if err != nil || method.Kind != Dev {
		t.Fatalf("dev method = %v, %v", method, err)
	}
	if runtime.GOOS == "windows" {
		t.Setenv("GOBIN", strings.ToUpper(filepath.Join(root, "custom-bin")))
		if got := DetectMethod(filepath.Join(root, "custom-bin", "ttt.exe"), info); got.Kind != GoInstall {
			t.Fatalf("case-insensitive GOBIN: %+v", got)
		}
	}
	bin := filepath.Join(root, "Cellar", "ttt", "bin", "ttt")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "ttt")
	if err := os.Symlink(bin, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if got := DetectMethod(link, nil); got.Kind != Homebrew || got.Path != bin {
		t.Fatalf("symlink method = %+v", got)
	}
}

func TestParseChecksums(t *testing.T) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte("ttt")))
	checksums, err := ParseChecksums(strings.NewReader(hash + "  ttt-linux-amd64\n" + strings.ToUpper(hash) + " *ttt-windows-arm64.exe\n"))
	if err != nil || len(checksums) != 2 || checksums["ttt-windows-arm64.exe"] != hash {
		t.Fatalf("checksums = %v, %v", checksums, err)
	}
	for _, bad := range []string{"not a checksum", strings.Repeat("x", 64) + " ttt", hash + " ttt\n" + hash + " ttt"} {
		if _, err := ParseChecksums(strings.NewReader(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestLatest(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		fails  bool
	}{
		{200, `{"tag_name":"v1.2.3","body":"# Notes","assets":[{"name":"checksums.txt","browser_download_url":"https://example.com/sums"}]}`, false},
		{403, "rate limited", true}, {200, "not JSON", true},
		{200, `{"tag_name":"dev"}`, true}, {200, `{"tag_name":"garbage"}`, true},
	} {
		t.Run(fmt.Sprint(tc.status, tc.body), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("User-Agent") != "ttt-update" {
					t.Error("missing user agent")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			service := Service{URL: server.URL}
			release, err := service.Latest(context.Background())
			if (err != nil) != tc.fails {
				t.Fatalf("Latest = %+v, %v", release, err)
			}
			if !tc.fails && (release.Tag != "v1.2.3" || release.Notes != "# Notes" || len(release.Assets) != 1) {
				t.Fatalf("release = %+v", release)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := service.Latest(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled lookup = %v", err)
			}
		})
	}
}

func TestBinaryReplacement(t *testing.T) {
	for _, scenario := range []string{"success", "checksum", "readonly", "rollback", "rollback-failure", "backup", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			if scenario == "readonly" && runtime.GOOS == "windows" {
				t.Skip("directory write permissions require Unix mode bits")
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "ttt")
			if err := os.WriteFile(path, []byte("old"), 0o751); err != nil {
				t.Fatal(err)
			}
			data := "new binary"
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(data)))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/sums" {
					if scenario == "checksum" {
						hash = strings.Repeat("0", 64)
					}
					fmt.Fprintf(w, "%s  asset\n", hash)
				} else {
					fmt.Fprint(w, data)
				}
			}))
			defer server.Close()
			release := Release{Tag: "v2.0.0", Assets: []Asset{{Name: "checksums.txt", URL: server.URL + "/sums"}, {Name: "asset", URL: server.URL + "/binary"}}}
			if scenario == "readonly" {
				if err := os.Chmod(dir, 0o555); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(dir, 0o755)
			}
			if scenario == "backup" {
				if err := os.WriteFile(path+".old", []byte("backup"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			renames := 0
			rename := func(from, to string) error {
				renames++
				if (scenario == "rollback" || scenario == "rollback-failure") && renames == 2 || scenario == "rollback-failure" && renames == 3 {
					return errors.New("injected rename failure")
				}
				return os.Rename(from, to)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "canceled" {
				cancel()
			}
			err := (Service{}).replace(ctx, release, path, "asset", rename)
			if (err == nil) != (scenario == "success") {
				t.Fatalf("replacement error = %v", err)
			}
			if scenario == "readonly" && !errors.Is(err, ErrNotWritable) {
				t.Fatalf("read-only error = %v", err)
			}
			readPath, want := path, "old"
			if scenario == "success" {
				want = data
				info, _ := os.Stat(path)
				if runtime.GOOS != "windows" && info.Mode().Perm() != 0o751 {
					t.Errorf("permissions = %v", info.Mode())
				}
			}
			if scenario == "rollback-failure" {
				readPath = path + ".old"
			}
			contents, readErr := os.ReadFile(readPath)
			if readErr != nil || string(contents) != want {
				t.Fatalf("binary = %q, %v; want %q", contents, readErr, want)
			}
			leftovers, _ := filepath.Glob(filepath.Join(dir, ".ttt-update-*"))
			if len(leftovers) != 0 {
				t.Fatalf("temporary files left: %v", leftovers)
			}
			if scenario == "success" || scenario == "rollback" {
				if _, err := os.Stat(path + ".old"); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("backup remains: %v", err)
				}
			}
		})
	}
}

func TestCommandOutputIsBounded(t *testing.T) {
	w := &tailWriter{}
	w.Write([]byte(strings.Repeat("a", 9000)))
	w.Write([]byte("\nlast line"))
	if len(w.data) != 8192 || !strings.HasSuffix(string(w.data), "last line") {
		t.Fatalf("tail: %d bytes, suffix %q", len(w.data), w.data[len(w.data)-10:])
	}
}
