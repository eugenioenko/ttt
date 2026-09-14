package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v3"
)

func guidesBefore(t *testing.T, h *testHarness, code string) bool {
	t.Helper()
	for _, row := range strings.Split(h.screenText(), "\n") {
		if i := strings.Index(row, code); i >= 0 {
			// Only the cells just before the code: window chrome elsewhere
			// on the row also uses │.
			start := i - 8
			if start < 0 {
				start = 0
			}
			return strings.Contains(row[start:i], "│")
		}
	}
	t.Fatalf("could not find %q on screen:\n%s", code, h.screenText())
	return false
}

// guideForeground mirrors production, where config.Load resolves the theme:
// the harness builds its style map from an unresolved DefaultTheme, in which
// the indent-guide fallback never ran.
func guideForeground() tcell.Color {
	th := config.DefaultTheme()
	th.ResolveColors()
	return app.BuildStyleMap(th)[term.StyleIndentGuide].GetForeground()
}

func TestIndentGuidesRender(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	fp := filepath.Join(h.dir, "guides.go")
	content := "package main\n\nfunc main() {\n\tinner()\n\tresult := f(1,  2)\n}\n"
	os.WriteFile(fp, []byte(content), 0644)
	h.app.EditorGroup.OpenFile(fp)
	h.redraw()

	if guidesBefore(t, h, "inner()") {
		t.Error("indent guide drawn while the setting is off")
	}

	h.app.Settings.Editor.IndentGuides = true
	h.app.EditorGroup.Editor.IndentGuides = true
	th := config.DefaultTheme()
	th.ResolveColors()
	h.app.Screen.SetStyleMap(app.BuildStyleMap(th))
	h.redraw()

	if !guidesBefore(t, h, "inner()") {
		t.Errorf("no indent guide before indented code:\n%s", h.screenText())
	}

	cells, w, ht := h.screen.GetContents()
	wantFg := guideForeground()
	foundGuide, foundGap := false, false
	for y := 0; y < ht; y++ {
		row := h.screenRow(y)
		if strings.Contains(row, "inner()") {
			// Tab-indented: the guide sits exactly one tab stop before 'i'.
			// (Byte index ≠ cell column: chrome left of the code is multibyte.)
			i := displayColumnOf(row, "inner()")
			g := cells[y*w+i-4]
			if runeAt(cells, y*w+i-4) != '│' {
				t.Errorf("expected │ at row %d col %d", y, i-4)
			} else if g.Style.GetForeground() != wantFg {
				t.Errorf("guide cell has fg %v, want indent-guide fg %v", g.Style.GetForeground(), wantFg)
			} else {
				foundGuide = true
			}
		}
		if strings.Contains(row, "1,") {
			// Mid-line double space must stay plain spaces, never guides.
			i := displayColumnOf(row, "1,")
			for _, dx := range []int{2, 3} {
				c := cells[y*w+i+dx]
				if runeAt(cells, y*w+i+dx) != ' ' || c.Style.GetForeground() == wantFg {
					t.Errorf("mid-line whitespace at row %d col %d must not be a guide", y, i+dx)
				} else {
					foundGap = true
				}
			}
		}
	}
	if !foundGuide {
		t.Error("guide cell assertion never ran")
	}
	if !foundGap {
		t.Error("mid-line gap assertion never ran")
	}
}

func TestIndentGuidesSettingsToggle(t *testing.T) {
	h := openSettings(t)
	defer h.stop()

	clickRowControl(t, h, "Appearance", "Appearance")
	if !rowHas(h, "Indent guides", uncheckedBox) {
		t.Fatalf("Indent guides row missing or not off by default:\n%s", h.screenText())
	}

	clickRowControl(t, h, "Indent guides", uncheckedBox)
	h.exec("settings.apply")
	if !h.app.Settings.Editor.IndentGuides {
		t.Error("IndentGuides not persisted after Apply")
	}
	if !h.app.EditorGroup.Editor.IndentGuides {
		t.Error("IndentGuides not live-applied to the editor")
	}
}

func TestIndentGuidesSurviveWordWrap(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	fp := filepath.Join(h.dir, "wrap.go")
	content := strings.Repeat(" ", 60) + "WRAPMARK\n"
	os.WriteFile(fp, []byte(content), 0644)
	h.app.Settings.Editor.IndentGuides = true
	h.app.EditorGroup.Editor.IndentGuides = true
	h.app.EditorGroup.Editor.WordWrap = true
	h.app.EditorGroup.OpenFile(fp)
	h.redraw()

	// The 60-space indent wraps: the continuation row holding WRAPMARK must
	// keep guides aligned with the first segment (absolute visual columns).
	rows := strings.Split(h.screenText(), "\n")
	markRow := -1
	for y, row := range rows {
		if strings.Contains(row, "WRAPMARK") {
			markRow = y
			break
		}
	}
	if markRow < 0 {
		t.Fatalf("WRAPMARK not on screen:\n%s", h.screenText())
	}
	if before := rows[markRow][:strings.Index(rows[markRow], "WRAPMARK")]; !strings.Contains(before, "│   │") {
		t.Errorf("wrap continuation lost its guides:\n%s", h.screenText())
	}
}
