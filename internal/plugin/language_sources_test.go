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

func TestEnglishSourceIsSharedAndWorksOffline(t *testing.T) {
	dir, commit := t.TempDir(), strings.Repeat("a", 40)
	requests := 0
	client := &http.Client{Transport: languageTransport(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.String() != "https://raw.githubusercontent.com/eugenioenko/ttt/"+commit+"/internal/i18n/en.json" {
			t.Fatal("English source was not pinned to the TTT commit", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"general.save":"Save"}`))}, nil
	})}
	if _, err := loadLanguageSource(context.Background(), client, dir, commit); err != nil {
		t.Fatal(err)
	}
	client.Transport = languageTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("cached source caused another network request")
		return nil, nil
	})
	if _, err := loadLanguageSource(context.Background(), client, dir, commit); err != nil {
		t.Fatal(err)
	}
	if err := prepareLanguageSources(dir, Manifest{Languages: []Language{{Locale: "es", SourceCommit: commit}, {Locale: "fr", SourceCommit: commit}}}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "languages", "sources"))
	if err != nil || requests != 1 || len(entries) != 1 || entries[0].Name() != commit+".json" {
		t.Fatalf("cache=%v, requests=%d, err=%v", entries, requests, err)
	}
}

func TestEnglishSourceRejectsInvalidDataAndCacheEscapes(t *testing.T) {
	commit := strings.Repeat("b", 40)
	for _, body := range []string{"", `{"general.save":null}`, `{"a":"one","a":"two"}`} {
		dir := t.TempDir()
		client := &http.Client{Transport: languageTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		if _, err := loadLanguageSource(context.Background(), client, dir, commit); err == nil {
			t.Errorf("accepted malformed source %q", body)
		}
		if _, err := os.Stat(filepath.Join(dir, LanguageSourcePath(commit))); !os.IsNotExist(err) {
			t.Fatal("invalid source was cached")
		}
	}
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "languages")); err != nil {
		t.Skip(err)
	}
	client := &http.Client{Transport: languageTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	if _, err := loadLanguageSource(context.Background(), client, dir, commit); err == nil {
		t.Fatal("source cache followed an external symlink")
	}
	if _, err := loadLanguageSource(context.Background(), client, t.TempDir(), "../../escape"); err == nil {
		t.Fatal("accepted invalid source commit")
	}
}
