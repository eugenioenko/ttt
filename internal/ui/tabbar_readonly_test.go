package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

func TestTabLabelReadOnlyFollowsIconMode(t *testing.T) {
	lock := icons.Get(config.IconsNerdFont, icons.Lock)
	cases := []struct {
		name string
		mode string
		tab  Tab
		want string
	}{
		{
			name: "disk read-only plain",
			mode: config.IconsNone,
			tab:  Tab{Name: "/tmp/notes.txt", ReadOnly: true, Active: true, Closable: true},
			want: " notes.txt (readonly) ✕ ",
		},
		{
			name: "viewer read-only plain",
			mode: config.IconsNone,
			tab:  Tab{Name: "notes.txt", ReadOnly: true, Active: true, Closable: true},
			want: " notes.txt (readonly) ✕ ",
		},
		{
			name: "disk read-only nerd font",
			mode: config.IconsNerdFont,
			tab:  Tab{Name: "/tmp/notes.txt", ReadOnly: true, Active: true, Closable: true, Dirty: true},
			want: " ● " + lock + " notes.txt ✕ ",
		},
		{
			name: "viewer read-only nerd font",
			mode: config.IconsNerdFont,
			tab:  Tab{Name: "notes.txt", ReadOnly: true, Active: true, Closable: true},
			want: " " + lock + " notes.txt ✕ ",
		},
		{
			name: "writable tab has no lock",
			mode: config.IconsNerdFont,
			tab:  Tab{Name: "notes.txt", Active: true, Closable: true},
			want: " notes.txt ✕ ",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tb := NewTabBarWidget()
			tb.Icons = tc.mode
			if got := tb.tabLabel(tc.tab); got != tc.want {
				t.Fatalf("tabLabel = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTabLabelMeasuresReadOnlyGlyphWithTextWidth(t *testing.T) {
	tb := NewTabBarWidget()
	tb.Icons = config.IconsNerdFont
	tab := Tab{Name: "筆.txt", ReadOnly: true, Active: true, Closable: true}
	tb.SetTabs([]Tab{tab})
	tb.SetRect(Rect{X: 0, Y: 0, W: 40, H: 3})
	tb.Render(NewRenderSurface(makeGrid(40, 3), Rect{X: 0, Y: 0, W: 40, H: 3}))

	label := tb.tabLabel(tab)
	if textwidth.String(label) == len([]rune(label)) {
		t.Fatal("fixture must be wider than its rune count")
	}
	span := tb.tabSpans[0]
	if got, want := span.end-span.start, textwidth.String(label)+2; got != want {
		t.Fatalf("active span width = %d, want %d from textwidth", got, want)
	}
	lock := icons.Get(config.IconsNerdFont, icons.Lock)
	lockAt := strings.Index(label, lock)
	nameAt := strings.Index(label, "筆.txt")
	if lockAt < 0 || nameAt < 0 || lockAt > nameAt {
		t.Fatalf("lock glyph = %q, want it before the name", label)
	}
}

func TestReadOnlySourcesShareTabLabel(t *testing.T) {
	dir := t.TempDir()
	diskPath := filepath.Join(dir, "locked.txt")
	if err := os.WriteFile(diskPath, []byte("disk\n"), 0444); err != nil {
		t.Fatal(err)
	}
	viewerPath := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(viewerPath, []byte("view\n"), 0644); err != nil {
		t.Fatal(err)
	}

	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.TabBar.Icons = config.IconsNone
	g.OpenFile(diskPath)
	g.OpenFileReadOnly(viewerPath, "")

	var disk, viewer Tab
	for _, tab := range g.TabBar.Tabs {
		switch filepath.Base(tab.Name) {
		case "locked.txt":
			disk = tab
		case "notes.txt":
			viewer = tab
		}
	}
	if !disk.ReadOnly || !viewer.ReadOnly {
		t.Fatalf("read-only flags: disk=%v name=%q viewer=%v name=%q", disk.ReadOnly, disk.Name, viewer.ReadOnly, viewer.Name)
	}
	if strings.Contains(viewer.Name, "readonly") {
		t.Fatalf("viewer title baked the suffix into %q", viewer.Name)
	}

	for _, tab := range []Tab{disk, viewer} {
		got := g.TabBar.tabLabel(tab)
		if !strings.Contains(got, "(readonly)") {
			t.Fatalf("plain label %q missing readonly text", got)
		}
	}

	g.TabBar.Icons = config.IconsNerdFont
	lock := icons.Get(config.IconsNerdFont, icons.Lock)
	for _, tab := range []Tab{disk, viewer} {
		got := g.TabBar.tabLabel(tab)
		name := filepath.Base(tab.Name)
		if strings.Contains(got, "(readonly)") || !strings.Contains(got, lock) || strings.Index(got, lock) > strings.Index(got, name) {
			t.Fatalf("nerd label = %q, want the lock glyph before %s", got, name)
		}
	}
}

func TestTabLabelPinnedFollowsIconMode(t *testing.T) {
	pin := icons.Get(config.IconsNerdFont, icons.Pinned)
	cases := []struct {
		name string
		mode string
		want string
	}{
		{name: "pinned active plain", mode: config.IconsNone, want: " notes.txt ♦ "},
		{name: "pinned active nerd font", mode: config.IconsNerdFont, want: " notes.txt " + pin + " "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tb := NewTabBarWidget()
			tb.Icons = tc.mode
			got := tb.tabLabel(Tab{Name: "notes.txt", Active: true, Closable: true, Pinned: true})
			if got != tc.want {
				t.Fatalf("tabLabel = %q, want %q", got, tc.want)
			}
		})
	}
}
