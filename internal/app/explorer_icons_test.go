package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/fileicons"
	"github.com/eugenioenko/ttt/internal/term"
)

func explorerIconFixture(t *testing.T) string {
	t.Helper()
	rootPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootPath, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"main.go", ".env", "src/app.ts"} {
		if err := os.WriteFile(filepath.Join(rootPath, filepath.FromSlash(name)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return rootPath
}

func TestExplorerIconsMuteGitIgnoredFiles(t *testing.T) {
	rootPath := explorerIconFixture(t)
	if err := os.WriteFile(filepath.Join(rootPath, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootPath, "debug.log"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	runTreeGit(t, rootPath, "init", "-q")
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), rootPath)
	if log := nodeWithLabel(explorer.Tree.Config.Items, "debug.log"); log == nil || log.Icon == "" || log.IconStyle != term.StyleMuted {
		t.Errorf("git-ignored debug.log icon = %+v, want a muted icon", log)
	}
	if mainGo := nodeWithLabel(explorer.Tree.Config.Items, "main.go"); mainGo == nil || mainGo.IconStyle == term.StyleMuted {
		t.Errorf("tracked main.go icon was muted: %+v", mainGo)
	}
}

func TestExplorerIconsNoneOmitsIcons(t *testing.T) {
	settings := config.DefaultExplorerSettings()
	settings.Icons = config.IconsNone
	explorer := NewNavigationPanel(settings, explorerIconFixture(t))
	for _, label := range []string{"src", "main.go", ".env"} {
		node := nodeWithLabel(explorer.Tree.Config.Items, label)
		if node == nil {
			t.Fatalf("missing %q", label)
		}
		if node.Icon != "" || node.ExpandedIcon != "" {
			t.Errorf("%q has an icon with icons disabled: %+v", label, node)
		}
	}
}

func TestExplorerIconsDefaultToNerdFontForFilesAndFolders(t *testing.T) {
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), explorerIconFixture(t))
	items := explorer.Tree.Config.Items

	if root := items[0]; root.Icon != "" {
		t.Errorf("workspace root should stay a plain header, got icon %q", root.Icon)
	}

	src := nodeWithLabel(items, "src")
	if src.Icon != fileicons.ForFolder(false).Glyph || src.ExpandedIcon != fileicons.ForFolder(true).Glyph {
		t.Errorf("folder icons = %q/%q, want closed and open folder glyphs", src.Icon, src.ExpandedIcon)
	}

	mainGo := nodeWithLabel(items, "main.go")
	if want := fileicons.ForFile("main.go"); mainGo.Icon != want.Glyph || mainGo.IconStyle != fileIconStyle(want.Color) {
		t.Errorf("main.go icon = %q style %v, want %q style %v", mainGo.Icon, mainGo.IconStyle, want.Glyph, fileIconStyle(want.Color))
	}

	if env := nodeWithLabel(items, ".env"); env.Icon == "" || env.IconStyle != term.StyleMuted {
		t.Errorf(".env icon = %q style %v, want a muted icon", env.Icon, env.IconStyle)
	}

	explorer.Tree.SelectByID(src.ID)
	explorer.Tree.ActivateSelected()
	if appTS := nodeWithLabel(src.Children, "app.ts"); appTS == nil || appTS.Icon != fileicons.ForFile("app.ts").Glyph {
		t.Errorf("lazily loaded child app.ts is missing its icon: %+v", appTS)
	}
}

func TestExplorerIconSettingChangeAppliesOnReload(t *testing.T) {
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), explorerIconFixture(t))
	explorer.Settings.Icons = config.IconsNone
	explorer.Reload()
	if node := nodeWithLabel(explorer.Tree.Config.Items, "main.go"); node == nil || node.Icon != "" {
		t.Fatalf("reload after disabling icons left main.go decorated: %+v", node)
	}
}

func TestFileIconStyleCoversEveryHueFamily(t *testing.T) {
	want := map[fileicons.Color]term.Style{
		fileicons.ColorDefault: term.StyleDefault,
		fileicons.ColorRed:     term.StyleFileIconRed,
		fileicons.ColorYellow:  term.StyleFileIconYellow,
		fileicons.ColorGreen:   term.StyleFileIconGreen,
		fileicons.ColorCyan:    term.StyleFileIconCyan,
		fileicons.ColorBlue:    term.StyleFileIconBlue,
		fileicons.ColorMagenta: term.StyleFileIconMagenta,
	}
	for color, style := range want {
		if got := fileIconStyle(color); got != style {
			t.Errorf("fileIconStyle(%d) = %v, want %v", color, got, style)
		}
	}
}
