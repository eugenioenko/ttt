package widgets

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"golang.org/x/text/unicode/norm"
)

func terminalRow(t *testing.T, surface *testSurface, y int) []string {
	t.Helper()
	sim := term.NewSimScreen()
	sim.SetSize(surface.w, surface.h)
	screen := term.NewTcellScreenFrom(sim)
	for y, row := range surface.cells {
		for x, cell := range row {
			screen.SetCell(x, y, cell)
		}
	}
	row := make([]string, surface.w)
	for x := range row {
		row[x], _, _ = sim.Get(x, y)
	}
	return row
}

func TestTreeUnicodeFilenameCells(t *testing.T) {
	for _, tc := range []struct {
		label string
		cells []string
	}{
		{"example.txt", []string{"e", "x", "a", "m", "p", "l", "e", ".", "t", "x", "t"}},
		{"café.txt", []string{"c", "a", "f", "é", ".", "t", "x", "t"}},
		{"cafe\u0301.txt", []string{"c", "a", "f", "é", ".", "t", "x", "t"}},
		{"e\u0301lastique.txt", []string{"é", "l", "a", "s", "t", "i", "q", "u", "e", ".", "t", "x", "t"}},
		{"e\u0301te\u0301-a\u0300.txt", []string{"é", "t", "é", "-", "à", ".", "t", "x", "t"}},
		{"日本.txt", []string{"日", " ", "本", " ", ".", "t", "x", "t"}},
		{"\u1100\u1161.txt", []string{"가", " ", ".", "t", "x", "t"}},
		{"q\u0301.txt", []string{"q\u0301", ".", "t", "x", "t"}},
		{"\u0301.txt", []string{"◌\u0301", ".", "t", "x", "t"}},
	} {
		t.Run(tc.label, func(t *testing.T) {
			node := &TreeNode{ID: "original/" + tc.label, Label: tc.label}
			tree := NewTreeWidget(TreeConfig{Items: []*TreeNode{node}})
			row := terminalRow(t, renderWidget(tree, 0, 0, 40, 1), 0)
			start := node.LabelX()
			for i, want := range tc.cells {
				if got := norm.NFC.String(row[start+i]); got != want {
					t.Errorf("column %d = %q, want %q", i, got, want)
				}
			}
			if node.Label != tc.label || node.ID != "original/"+tc.label {
				t.Fatalf("render changed filesystem identity: %+v", node)
			}
		})
	}
}

func TestTreeUnicodeFilenameRightBadge(t *testing.T) {
	nodes := []*TreeNode{
		{ID: "nfc", Label: "café.txt", Badge: "M"},
		{ID: "nfd", Label: "cafe\u0301.txt", Badge: "M"},
	}
	tree := NewTreeWidget(TreeConfig{Items: nodes, BadgeRight: true})
	surface := renderWidget(tree, 0, 0, 12, 2)
	first, second := terminalRow(t, surface, 0), terminalRow(t, surface, 1)
	for x := range first {
		if norm.NFC.String(first[x]) != norm.NFC.String(second[x]) {
			t.Errorf("column %d differs: NFC=%q NFD=%q", x, first[x], second[x])
		}
	}
	if !strings.Contains(strings.Join(first, ""), "M") {
		t.Fatal("right badge missing")
	}
}

func TestTreeUnicodeFilenameClipping(t *testing.T) {
	for _, tc := range []struct {
		label string
		left  bool
		want  string
	}{
		{"café.txt", false, "café…"},
		{"cafe\u0301.txt", false, "café…"},
		{"café.txt", true, "….txt"},
		{"cafe\u0301.txt", true, "….txt"},
		{"prefix-e\u0301.txt", true, "….txt"},
		{"prefix-q\u0301.txt", true, "….txt"},
		{"prefix-e\u0301xyz", true, "…éxyz"},
		{"prefix-q\u0301xyz", true, "…q\u0301xyz"},
		{"ab日本.txt", false, "ab日…"},
	} {
		t.Run(tc.label+tc.want, func(t *testing.T) {
			node := &TreeNode{ID: tc.label, Label: tc.label, TruncateLeft: tc.left}
			tree := NewTreeWidget(TreeConfig{Items: []*TreeNode{node}})
			surface := renderWidget(tree, 0, 0, 7, 1)
			row := terminalRow(t, surface, 0)
			var text strings.Builder
			for x := 0; x < 5; x++ {
				text.WriteString(row[x])
				if surface.cells[0][x].Ch == '日' {
					x++
				}
			}
			if got := norm.NFC.String(text.String()); got != tc.want {
				t.Errorf("row = %q, want %q", got, tc.want)
			}
			if row[5] != " " || row[6] != " " {
				t.Fatalf("label crossed clipping boundary: %q", row)
			}
		})
	}
}

func TestTreeCanonicallyEquivalentNamesRemainDistinct(t *testing.T) {
	nodes := []*TreeNode{
		{ID: "/café.txt", Label: "café.txt"},
		{ID: "/cafe\u0301.txt", Label: "cafe\u0301.txt"},
	}
	var activated []string
	tree := NewTreeWidget(TreeConfig{Items: nodes, OnCommand: func(_ string, node *TreeNode) {
		activated = append(activated, node.ID)
	}})
	for _, node := range nodes {
		tree.SelectByID(node.ID)
		tree.ActivateSelected()
	}
	if len(activated) != 2 || activated[0] != nodes[0].ID || activated[1] != nodes[1].ID {
		t.Fatalf("activated = %q", activated)
	}
}
