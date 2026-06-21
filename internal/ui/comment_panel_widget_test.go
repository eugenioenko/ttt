package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v2"
)

func TestNewCommentPanelWidget(t *testing.T) {
	panel := NewCommentPanelWidget("PR #42: Fix bug")
	if panel.Title != "PR #42: Fix bug" {
		t.Errorf("expected title 'PR #42: Fix bug', got %q", panel.Title)
	}
	if panel.Input == nil {
		t.Fatal("expected Input to be initialized")
	}
	if !panel.Focusable() {
		t.Error("expected panel to be focusable")
	}
}

func TestCommentPanelSetComments(t *testing.T) {
	panel := NewCommentPanelWidget("Test PR")
	panel.Loading = true
	items := []CommentItem{
		{ID: 1, Author: "user1", Timestamp: "2024-01-15T10:00:00Z", Body: "LGTM"},
		{ID: 2, Author: "user2", Timestamp: "2024-01-15T11:00:00Z", Body: "Needs fix"},
	}
	panel.SetComments(items)
	if panel.Loading {
		t.Error("expected Loading to be false after SetComments")
	}
	if len(panel.Comments) != 2 {
		t.Errorf("expected 2 comments, got %d", len(panel.Comments))
	}
	if panel.scrollTop != 0 {
		t.Error("expected scrollTop to reset to 0")
	}
}

func TestCommentPanelRender(t *testing.T) {
	panel := NewCommentPanelWidget("PR #1: Test")
	panel.SetComments([]CommentItem{
		{ID: 1, Author: "alice", Timestamp: "2024-06-15T10:00:00Z", Body: "Hello world"},
	})

	w, h := 80, 24
	panel.SetRect(Rect{X: 0, Y: 0, W: w, H: h})
	cells := make([][]term.Cell, h)
	for y := range cells {
		cells[y] = make([]term.Cell, w)
	}
	surface := NewRenderSurface(cells, Rect{X: 0, Y: 0, W: w, H: h})
	panel.Render(surface)

	// Verify panel renders on the right side
	panelW := panel.panelWidth(w)
	panelX := w - panelW

	// Left border should be a vertical line
	borderCell := cells[0][panelX]
	bs := term.SingleBorderSet()
	if borderCell.Ch != bs.Vertical {
		t.Errorf("expected left border at x=%d, got %q", panelX, string(borderCell.Ch))
	}

	// Close button should exist in header
	closeX := panelX + panelW - 4
	if closeX >= 0 && closeX < w {
		if cells[0][closeX+1].Ch != 'X' {
			t.Errorf("expected close button 'X' at x=%d, got %q", closeX+1, string(cells[0][closeX+1].Ch))
		}
	}
}

func TestCommentPanelRenderNoComments(t *testing.T) {
	panel := NewCommentPanelWidget("Empty PR")
	panel.SetComments(nil)

	w, h := 80, 24
	panel.SetRect(Rect{X: 0, Y: 0, W: w, H: h})
	cells := make([][]term.Cell, h)
	for y := range cells {
		cells[y] = make([]term.Cell, w)
	}
	surface := NewRenderSurface(cells, Rect{X: 0, Y: 0, W: w, H: h})
	panel.Render(surface)

	// Should show "No comments yet" message
	found := false
	for y := 0; y < h; y++ {
		var line []rune
		for x := 0; x < w; x++ {
			if cells[y][x].Ch != 0 {
				line = append(line, cells[y][x].Ch)
			}
		}
		if strings.Contains(string(line), "No comments yet") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'No comments yet' message in empty panel")
	}
}

func TestCommentPanelRenderLoading(t *testing.T) {
	panel := NewCommentPanelWidget("Loading PR")
	panel.Loading = true

	w, h := 80, 24
	panel.SetRect(Rect{X: 0, Y: 0, W: w, H: h})
	cells := make([][]term.Cell, h)
	for y := range cells {
		cells[y] = make([]term.Cell, w)
	}
	surface := NewRenderSurface(cells, Rect{X: 0, Y: 0, W: w, H: h})
	panel.Render(surface)

	// Should show "Loading comments..." message
	found := false
	for y := 0; y < h; y++ {
		var line []rune
		for x := 0; x < w; x++ {
			if cells[y][x].Ch != 0 {
				line = append(line, cells[y][x].Ch)
			}
		}
		if strings.Contains(string(line), "Loading comments") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Loading comments...' message")
	}
}

func TestCommentPanelHandleEscape(t *testing.T) {
	panel := NewCommentPanelWidget("Test")
	closed := false
	panel.OnClose = func() { closed = true }

	ev := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
	result := panel.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("expected Escape to be consumed")
	}
	if !closed {
		t.Error("expected OnClose to be called")
	}
}

func TestCommentPanelHandleScrollKeys(t *testing.T) {
	panel := NewCommentPanelWidget("Test")
	panel.scrollTop = 5

	// Up arrow
	ev := tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	panel.HandleEvent(ev)
	if panel.scrollTop != 4 {
		t.Errorf("expected scrollTop=4 after Up, got %d", panel.scrollTop)
	}

	// Down arrow
	ev = tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	panel.HandleEvent(ev)
	if panel.scrollTop != 5 {
		t.Errorf("expected scrollTop=5 after Down, got %d", panel.scrollTop)
	}

	// PgUp
	panel.scrollTop = 15
	ev = tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone)
	panel.HandleEvent(ev)
	if panel.scrollTop != 5 {
		t.Errorf("expected scrollTop=5 after PgUp, got %d", panel.scrollTop)
	}

	// PgDn
	ev = tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone)
	panel.HandleEvent(ev)
	if panel.scrollTop != 15 {
		t.Errorf("expected scrollTop=15 after PgDn, got %d", panel.scrollTop)
	}
}

func TestCommentPanelHandleScrollUpClampsAtZero(t *testing.T) {
	panel := NewCommentPanelWidget("Test")
	panel.scrollTop = 0

	ev := tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	panel.HandleEvent(ev)
	if panel.scrollTop != 0 {
		t.Errorf("expected scrollTop to stay at 0, got %d", panel.scrollTop)
	}
}

func TestCommentPanelComposeMode(t *testing.T) {
	panel := NewCommentPanelWidget("Test")

	// Press 'c' to enter compose mode
	ev := tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone)
	panel.HandleEvent(ev)
	if !panel.composing {
		t.Error("expected composing to be true after pressing 'c'")
	}

	// Type a character
	ev = tcell.NewEventKey(tcell.KeyRune, 'H', tcell.ModNone)
	panel.HandleEvent(ev)
	if panel.Input.Text != "H" {
		t.Errorf("expected Input.Text='H', got %q", panel.Input.Text)
	}

	// Escape exits compose mode
	ev = tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
	panel.HandleEvent(ev)
	if panel.composing {
		t.Error("expected composing to be false after Escape in compose mode")
	}
}

func TestCommentPanelSubmitComment(t *testing.T) {
	panel := NewCommentPanelWidget("Test")
	submitted := ""
	panel.OnSubmit = func(body string) { submitted = body }

	// Enter compose mode
	ev := tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone)
	panel.HandleEvent(ev)

	// Type text
	for _, ch := range "test comment" {
		ev = tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone)
		panel.HandleEvent(ev)
	}

	// Submit
	ev = tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	panel.HandleEvent(ev)
	if submitted != "test comment" {
		t.Errorf("expected submitted='test comment', got %q", submitted)
	}
	if panel.composing {
		t.Error("expected composing to be false after submit")
	}
	if panel.Input.Text != "" {
		t.Error("expected input to be cleared after submit")
	}
}

func TestCommentPanelSubmitEmptyIgnored(t *testing.T) {
	panel := NewCommentPanelWidget("Test")
	called := false
	panel.OnSubmit = func(body string) { called = true }

	// Enter compose mode
	ev := tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone)
	panel.HandleEvent(ev)

	// Submit empty
	ev = tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	panel.HandleEvent(ev)
	if called {
		t.Error("expected OnSubmit not to be called for empty text")
	}
}

func TestCommentPanelQuitKey(t *testing.T) {
	panel := NewCommentPanelWidget("Test")
	closed := false
	panel.OnClose = func() { closed = true }

	ev := tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone)
	panel.HandleEvent(ev)
	if !closed {
		t.Error("expected OnClose to be called on 'q'")
	}
}

func TestCommentPanelWidth(t *testing.T) {
	panel := NewCommentPanelWidget("Test")

	// Normal screen
	w := panel.panelWidth(200)
	if w != 80 {
		t.Errorf("expected width=80 for 200-wide screen, got %d", w)
	}

	// 40% of 100 = 40
	w = panel.panelWidth(100)
	if w != 40 {
		t.Errorf("expected width=40 for 100-wide screen, got %d", w)
	}

	// Small screen
	w = panel.panelWidth(50)
	if w > 40 {
		t.Errorf("expected width<=40 for 50-wide screen, got %d", w)
	}
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		text  string
		width int
		want  int // expected number of lines
	}{
		{"short", 80, 1},
		{"hello world", 5, 2},
		{"", 80, 1}, // empty string produces one empty line
		{"line1\nline2", 80, 2},
		{"a b c d e f", 5, 2},
	}
	for _, tt := range tests {
		got := wrapText(tt.text, tt.width)
		if len(got) != tt.want {
			t.Errorf("wrapText(%q, %d) = %d lines, want %d (got: %v)",
				tt.text, tt.width, len(got), tt.want, got)
		}
	}
}

func TestWrapTextZeroWidth(t *testing.T) {
	result := wrapText("text", 0)
	if result != nil {
		t.Errorf("expected nil for zero width, got %v", result)
	}
}

func TestFormatTimestamp(t *testing.T) {
	// Valid ISO 8601
	ts := formatTimestamp("2020-01-01T00:00:00Z")
	if ts == "" || ts == "2020-01-01T00:00:00Z" {
		// Should have been formatted (it's old enough to show a date)
		if ts != "Jan 1, 2020" {
			t.Errorf("unexpected timestamp format: %q", ts)
		}
	}

	// Invalid timestamp returns as-is
	ts = formatTimestamp("not a date")
	if ts != "not a date" {
		t.Errorf("expected invalid timestamp to pass through, got %q", ts)
	}
}

func TestCommentPanelInlineComment(t *testing.T) {
	panel := NewCommentPanelWidget("Test PR")
	panel.SetComments([]CommentItem{
		{
			ID:       1,
			Author:   "reviewer",
			Body:     "Fix this line",
			FilePath: "main.go",
			Line:     42,
			IsInline: true,
		},
	})

	w, h := 80, 24
	panel.SetRect(Rect{X: 0, Y: 0, W: w, H: h})
	cells := make([][]term.Cell, h)
	for y := range cells {
		cells[y] = make([]term.Cell, w)
	}
	surface := NewRenderSurface(cells, Rect{X: 0, Y: 0, W: w, H: h})
	panel.Render(surface)

	// Verify file reference is rendered
	found := false
	for y := 0; y < h; y++ {
		var line []rune
		for x := 0; x < w; x++ {
			if cells[y][x].Ch != 0 {
				line = append(line, cells[y][x].Ch)
			}
		}
		if strings.Contains(string(line), "main.go:42") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected inline comment to show 'main.go:42' file reference")
	}
}

func TestCommentPanelCursorPosition(t *testing.T) {
	panel := NewCommentPanelWidget("Test")
	panel.SetRect(Rect{X: 0, Y: 0, W: 80, H: 24})

	// Not composing - no cursor
	_, _, visible := panel.CursorPosition()
	if visible {
		t.Error("expected cursor to be invisible when not composing")
	}

	// Enter compose mode
	ev := tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone)
	panel.HandleEvent(ev)

	_, _, visible = panel.CursorPosition()
	if !visible {
		t.Error("expected cursor to be visible when composing")
	}
}

func TestCommentPanelFocusedInput(t *testing.T) {
	panel := NewCommentPanelWidget("Test")

	// Not composing
	if inp := panel.FocusedInput(); inp != nil {
		t.Error("expected nil FocusedInput when not composing")
	}

	// Composing
	panel.composing = true
	if inp := panel.FocusedInput(); inp == nil {
		t.Error("expected non-nil FocusedInput when composing")
	}
}

func TestRenderCommentsMultiple(t *testing.T) {
	panel := NewCommentPanelWidget("Test")
	panel.SetComments([]CommentItem{
		{Author: "alice", Body: "First comment", Timestamp: "2024-01-01T00:00:00Z"},
		{Author: "bob", Body: "Second comment", Timestamp: "2024-01-02T00:00:00Z"},
	})

	lines := panel.renderComments(40)
	if len(lines) == 0 {
		t.Fatal("expected rendered lines")
	}

	// Should contain separator between comments
	hasSeparator := false
	for _, rl := range lines {
		allBorder := true
		nonEmpty := false
		for _, cell := range rl.cells {
			if cell.Ch != 0 {
				nonEmpty = true
				if cell.Style != term.StyleBorder {
					allBorder = false
				}
			}
		}
		if nonEmpty && allBorder {
			hasSeparator = true
			break
		}
	}
	if !hasSeparator {
		t.Error("expected separator line between comments")
	}
}
