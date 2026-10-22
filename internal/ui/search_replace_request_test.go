package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestSearchReplaceRequestSnapshotsAndInvalidation(t *testing.T) {
	s := NewSearchWidget()
	s.Input.Text, s.ReplaceInput.Text = "foo", "bar"
	s.Include.Text, s.Exclude.Text = "*.go", "vendor/**"
	s.Options = SearchOptions{CaseSensitive: true, UseRegex: true}
	s.SetWorkDirs([]string{"/repo"})
	s.Groups = []SearchFileGroup{{FilePath: "/repo/a.go", Matches: []SearchMatch{{LineText: "foo"}}}}
	var req SearchReplaceRequest
	s.OnReplaceRequest = func(r SearchReplaceRequest) { req = r }
	s.ReplaceAllFiles()
	s.Groups[0].FilePath, s.Groups[0].Matches[0].LineText = "changed", "changed"
	if req.Paths[0] != "/repo/a.go" || req.Fallback[req.Paths[0]][0].LineText != "foo" ||
		req.Options.Include != "*.go" || req.Options.Exclude != "vendor/**" ||
		!req.Options.CaseSensitive || !req.Options.UseRegex || !req.AllFiles || !s.ReplacementCurrent(req) {
		t.Fatalf("request did not snapshot inputs: %+v", req)
	}
	s.ReplaceInput.Text = "other"
	s.ReplaceInput.OnChange("other")
	s.ReplaceInput.Text = "bar"
	s.ReplaceInput.OnChange("bar")
	if s.ReplacementCurrent(req) {
		t.Fatal("returning to the original text revived a stale request")
	}
}

func TestSearchReplacePendingCanBeCancelled(t *testing.T) {
	for _, action := range []string{"button", "escape"} {
		t.Run(action, func(t *testing.T) {
			s := NewSearchWidget()
			cancelled := false
			s.OnCancelReplace = func() { cancelled = true }
			s.SetReplacementPending(true)
			grid := makeGrid(40, 12)
			s.Render(NewRenderSurface(grid, Rect{W: 40, H: 12}))
			if s.ReplaceInput.Actions[0].Label != "Cancel" {
				t.Fatal("pending replacement has no cancel action")
			}
			if action == "button" {
				s.ReplaceInput.Actions[0].OnClick()
			} else {
				s.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, "", tcell.ModNone))
			}
			if !cancelled || s.replacing || s.ReplaceInput.Actions[0].Label != "⟳All" {
				t.Fatal("cancel left a pending replacement")
			}
		})
	}
}
