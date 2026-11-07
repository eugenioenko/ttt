package ui

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/undo"
)

func TestEditableDiffUnchangedBufferReplacementPreservesMarkers(t *testing.T) {
	for _, mode := range []DiffMode{DiffModeSplit, DiffModeUnified} {
		for _, blank := range []bool{false, true} {
			t.Run(fmt.Sprintf("mode=%d/blank=%v", mode, blank), func(t *testing.T) {
				old := []string{"# README", "", "Original paragraph.", "", "## Usage", "", "Run the editor."}
				cur := slices.Clone(old)
				cur[2] = "Edited paragraph."
				f := newLiveDiffFixture(t, old, cur, mode, 100, 20)
				f.d.SetContextMode(DiffContextFullFile)
				if blank {
					f.e.ExecCommand(&undo.SplitLineCommand{Line: 1})
				}
				beforeScreen := f.render()
				cur = slices.Clone(f.e.Buf.Lines)
				before := slices.Clone(f.e.DiffOverlay.Kinds)
				f.e.ExecCommand(&undo.BatchCommand{Commands: []undo.EditCommand{
					&undo.DeleteSelectionCommand{EndLine: len(cur) - 1, EndCol: len(cur[len(cur)-1])},
					&undo.PasteCommand{Text: strings.Join(cur, "\n")},
				}})
				afterScreen := f.render()
				if !slices.Equal(beforeScreen, afterScreen) {
					t.Fatal("unchanged buffer replacement changed the rendered diff")
				}
				if !reflect.DeepEqual(f.e.DiffOverlay.Kinds, before) {
					t.Fatalf("unchanged formatter replacement changed markers: before %v, after %v", before, f.e.DiffOverlay.Kinds)
				}
			})
		}
	}
}
