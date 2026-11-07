package e2e

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
)

func TestWordWrapColumnLanguageLifecycle(t *testing.T) {
	h := newTestHarness(t, 180, 30)
	defer h.stop()
	h.app.EditorGroup.SyntaxHighlight = false
	if err := os.WriteFile(config.ConfigFilePath("settings.json"), []byte(`{"editor":{"wordWrap":true,"wordWrapColumn":80,"syntaxHighlight":false,"languageOverrides":{"typescript":{"wordWrapColumn":80},"markdown":{"wordWrapColumn":65}}},"lsp":{"enabled":false}}`), 0644); err != nil {
		t.Fatal(err)
	}
	h.exec("settings.reload")
	paths := []string{filepath.Join(h.dir, "file.ts"), filepath.Join(h.dir, "README.md")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte(strings.Repeat("x", 200)), 0644); err != nil {
			t.Fatal(err)
		}
		h.app.EditorGroup.OpenFile(path)
		h.app.EditorGroup.CommitActiveTab()
	}
	check := func(want int, wrap bool) {
		t.Helper()
		h.redraw()
		e := h.app.EditorGroup.Editor
		if e.WordWrap != wrap || (wrap && e.Viewport.Width != want) {
			t.Fatalf("effective wrap=%v width=%d, want %v/%d", e.WordWrap, e.Viewport.Width, wrap, want)
		}
		if e.Buf.Dirty {
			t.Fatal("settings marked buffer modified")
		}
	}
	check(65, true)
	h.app.EditorGroup.SwitchToTabByPath(paths[0])
	check(80, true)
	h.app.EditorGroup.SwitchToTabByPath(paths[1])
	check(65, true)
	zero, disabled := 0, false
	s := *h.app.Settings
	s.Editor.LanguageOverrides = maps.Clone(s.Editor.LanguageOverrides)
	s.Editor.LanguageOverrides["markdown"] = config.EditorLanguageOverride{WordWrapColumn: &zero}
	s.Editor.LanguageOverrides["typescript"] = config.EditorLanguageOverride{WordWrap: &disabled}
	if err := config.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	h.exec("settings.reload")
	h.redraw()
	if h.app.EditorGroup.Editor.Viewport.Width <= 80 {
		t.Fatal("explicit zero retained global cap")
	}
	h.app.EditorGroup.SwitchToTabByPath(paths[0])
	check(0, false)
	h.exec("options.toggleWordWrap")
	h.exec("options.toggleWordWrap")
	check(0, false)
}
