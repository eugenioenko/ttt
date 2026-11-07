package plugin

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type languageTransport func(*http.Request) (*http.Response, error)

func (f languageTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestLanguageUpdateVersionComparison(t *testing.T) {
	for _, test := range []struct {
		editor, pack string
		want         bool
	}{
		{"v1.12.0", "1.11.9", true}, {"1.12.0", "1.12.0", false}, {"1.12.0", "1.13.0", false},
		{"1.12.0", "", true}, {"1.12.0", "dev", true}, {"dev", "1.11.0", false},
		{"v1.12.0-5-gabcd", "v1.11.0", true}, {"1.9.0", "1.10.0", false},
	} {
		if got := LanguageNeedsUpdate(test.editor, test.pack); got != test.want {
			t.Errorf("%s vs %s = %v", test.editor, test.pack, got)
		}
	}
}

func TestFetchLanguageUpdateDownloadsOnlyPinnedLanguageData(t *testing.T) {
	sha := strings.Repeat("a", 40)
	var requests []string
	client := &http.Client{Transport: languageTransport(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.URL.String())
		body := `{"sha":"` + sha + `"}`
		if request.URL.Host == "raw.githubusercontent.com" {
			if !strings.Contains(request.URL.Path, "/"+sha+"/") && !strings.Contains(request.URL.Path, "/"+strings.Repeat("b", 40)+"/") {
				t.Fatal("download was not pinned to a commit")
			}
			switch filepath.Base(request.URL.Path) {
			case "plugin.ttt.json":
				body = `{"entry":"never-execute.lua","permissions":{"system.exec":["sh"]},"languages":[{"locale":"es","name":"Español","path":"messages.json","sourceCommit":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","tttVersion":"1.12.0"}]}`
			case "messages.json":
				body = `{"general.save":"Guardar"}`
			case "en.json":
				body = `{"general.save":"Save"}`
			default:
				t.Fatalf("unexpected download %s", request.URL)
			}
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	source := LanguageUpdateSource{Name: "spanish", Repo: "https://github.com/example/translations.git", RepoPath: "packs/es", CacheDir: t.TempDir(), Languages: []Language{{Locale: "es", TTTVersion: "1.11.0"}}}
	update, err := FetchLanguageUpdate(context.Background(), client, source, "1.12.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 4 || string(update.Files["es.json"]) != `{"general.save":"Guardar"}` || len(update.Languages) != 1 || update.Languages[0].TTTVersion != "1.12.0" {
		t.Fatalf("update=%+v, requests=%v", update, requests)
	}
	for _, invalid := range []string{"http://localhost/repo", "https://example.com/o/r", "https://github.com/../r", "https://github.com/o/r?token=bad"} {
		if _, err := languageRepository(invalid); err == nil {
			t.Errorf("accepted %q", invalid)
		}
	}
}

func TestLanguageOnlyManifestAndPathIsolation(t *testing.T) {
	dir := t.TempDir()
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"name":"spanish","languages":[{"locale":"es","name":"Español","path":"es.json"}]}`)
	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := &Plugin{Name: manifest.Name, Dir: dir, Manifest: manifest}
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	if !p.Enabled || manifest.Entry != "" || !DiffPermissions(PermissionSet{}, manifest.Permissions).IsEmpty() {
		t.Fatal("language-only plugin requires runtime capabilities")
	}
	for _, languages := range []string{
		`[{"locale":"","name":"bad","path":"es.json"}]`,
		`[{"locale":"en","name":"bad","path":"es.json"}]`,
		`[{"locale":"../es","name":"bad","path":"es.json"}]`,
		`[{"locale":"es","name":"bad","path":"../es.json"}]`,
		`[{"locale":"es","name":"bad","path":"es.json","sourceCommit":"../invalid"}]`,
		`[{"locale":"es","name":"bad","path":"es.json"},{"locale":"es","name":"duplicate","path":"other.json"}]`,
	} {
		write(`{"name":"bad","languages":` + languages + `}`)
		if _, err := LoadManifest(dir); err == nil {
			t.Errorf("accepted %s", languages)
		}
	}
	outside := filepath.Join(t.TempDir(), "messages.json")
	os.WriteFile(outside, []byte(`{}`), 0644)
	if err := os.Symlink(outside, filepath.Join(dir, "outside.json")); err != nil {
		t.Skip(err)
	}
	if _, err := p.LanguageFile("outside.json"); err == nil {
		t.Fatal("language symlink escaped plugin root")
	}
}

func TestLanguageDownloadSizeLimit(t *testing.T) {
	if _, err := readLanguageData(strings.NewReader(strings.Repeat("x", maxLanguageDataBytes+1))); err == nil {
		t.Fatal("accepted oversized language data")
	}
}

func TestLanguageUpdateUnavailableOrInvalid(t *testing.T) {
	for _, test := range []struct {
		name, version, messages string
		status                  int
		wantError               bool
	}{
		{"offline", "1.12.0", `{}`, http.StatusServiceUnavailable, true},
		{"catalog-not-ready", "1.11.0", `{}`, http.StatusOK, false},
		{"unreleased-catalog", "dev", `{}`, http.StatusOK, false},
		{"malformed-catalog", "1.12.0", `{"general.save":null}`, http.StatusOK, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: languageTransport(func(request *http.Request) (*http.Response, error) {
				body := `{"sha":"` + strings.Repeat("a", 40) + `"}`
				if request.URL.Host == "raw.githubusercontent.com" {
					body = `{"languages":[{"locale":"es","name":"Español","path":"messages.json","tttVersion":"` + test.version + `"}]}`
					if strings.HasSuffix(request.URL.Path, "/messages.json") {
						body = test.messages
					}
				}
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			update, err := FetchLanguageUpdate(context.Background(), client, LanguageUpdateSource{Repo: "https://github.com/example/packs", Languages: []Language{{Locale: "es"}}}, "1.12.0")
			if (err != nil) != test.wantError || len(update.Files) != 0 || len(update.Languages) != 0 {
				t.Fatalf("update=%+v, err=%v", update, err)
			}
		})
	}
}
