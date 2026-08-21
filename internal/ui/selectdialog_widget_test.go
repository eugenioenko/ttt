package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"

	"github.com/gdamore/tcell/v3"
)

func defaultShortcut(commandID string) string {
	for _, binding := range config.DefaultKeybindings() {
		if binding.Command == commandID {
			return binding.Key
		}
	}
	return ""
}

func helpTestCommands() []command.Command {
	return []command.Command{
		{
			ID:       "file.quickOpen",
			Title:    "Open Anything From the Workspace",
			Shortcut: defaultShortcut("file.quickOpen"),
			Keywords: []string{"file", "navigate", "open"},
		},
		{
			ID:       "sidebar.search",
			Title:    "Search Everywhere",
			Shortcut: defaultShortcut("sidebar.search"),
			Keywords: []string{"sidebar", "search", "workspace"},
		},
		{ID: "tab.closeAllSaved", Title: "Close All Saved Tabs"},
	}
}

func testCommands() []command.Command {
	return []command.Command{
		{ID: "file.save", Title: "File: Save"},
		{ID: "file.open", Title: "File: Open"},
		{ID: "editor.split", Title: "Split Editor Right"},
		{ID: "sidebar.toggle", Title: "View: Toggle Sidebar"},
	}
}

func TestPaletteFilter(t *testing.T) {
	p := NewSelectDialogWidget(testCommands())

	if len(p.Items) != 4 {
		t.Fatalf("empty query: expected 4, got %d", len(p.Items))
	}

	// Initial text is "> ", typing appends after it
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "f", 0))
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "i", 0))
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "l", 0))
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "e", 0))

	if len(p.Items) != 2 {
		t.Fatalf("query 'file': expected 2 results, got %d", len(p.Items))
	}
}

func TestPaletteNavigation(t *testing.T) {
	p := NewSelectDialogWidget(testCommands())

	p.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", 0))
	if p.Selected != 1 {
		t.Fatalf("expected selected 1, got %d", p.Selected)
	}

	p.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", 0))
	if p.Selected != 0 {
		t.Fatalf("expected selected 0, got %d", p.Selected)
	}

	// Wraps to last item
	p.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", 0))
	if p.Selected != 3 {
		t.Fatalf("expected selected 3 (wrapped), got %d", p.Selected)
	}
}

func TestPaletteExecute(t *testing.T) {
	p := NewSelectDialogWidget(testCommands())
	executed := ""
	p.OnExecute = func(id string) { executed = id }

	p.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", 0))
	p.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))

	if executed != "file.open" {
		t.Fatalf("expected 'file.open', got '%s'", executed)
	}
}

func TestPaletteDismiss(t *testing.T) {
	p := NewSelectDialogWidget(testCommands())
	dismissed := false
	p.OnDismiss = func() { dismissed = true }

	p.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, "", 0))

	if !dismissed {
		t.Fatal("palette should have been dismissed")
	}
}

func TestPaletteAlwaysConsumes(t *testing.T) {
	p := NewSelectDialogWidget(testCommands())
	result := p.HandleEvent(tcell.NewEventKey(tcell.KeyF1, "", 0))
	if result != EventConsumed {
		t.Fatal("palette should consume all events (modal)")
	}
}

func TestPaletteBackspace(t *testing.T) {
	p := NewSelectDialogWidget(testCommands())

	// Initial text is ">", type "sp" → ">sp"
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "s", 0))
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "p", 0))
	if p.Input.Text != ">sp" {
		t.Fatalf("expected text '>sp', got '%s'", p.Input.Text)
	}

	p.HandleEvent(tcell.NewEventKey(tcell.KeyBackspace2, "", 0))
	if p.Input.Text != ">s" {
		t.Fatalf("expected text '>s' after backspace, got '%s'", p.Input.Text)
	}
}

func TestPaletteFuzzyFilter(t *testing.T) {
	p := NewSelectDialogWidget(testCommands())

	// Type "fs" which should fuzzy match "File: Save" (F + S at word boundaries)
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "f", 0))
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "s", 0))

	found := false
	for _, item := range p.Items {
		if item.ID == "file.save" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected fuzzy query 'fs' to match 'File: Save', got %d results", len(p.Items))
	}
}

func TestPaletteFuzzyFilterInitials(t *testing.T) {
	p := NewSelectDialogWidget(testCommands())

	// Type "se" which should fuzzy match "Split Editor Right" (s...e in Editor)
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "s", 0))
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "e", 0))

	found := false
	for _, item := range p.Items {
		if item.ID == "editor.split" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected fuzzy query 'se' to match 'Split Editor Right'")
	}
}

func TestPaletteFuzzyScoreOrdering(t *testing.T) {
	cmds := []command.Command{
		{ID: "a", Title: "View: Toggle Sidebar"},
		{ID: "b", Title: "To Do List"},
		{ID: "c", Title: "Top Level"},
	}
	p := NewSelectDialogWidget(cmds)

	// Type "to" - "Toggle Sidebar" and "Top Level" start with "to" (substring at position 0),
	// "To Do List" starts with "To" too. All are substring matches.
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "t", 0))
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "o", 0))

	if len(p.Items) < 3 {
		t.Fatalf("expected at least 3 results, got %d", len(p.Items))
	}
}

func TestPaletteModeSwitch(t *testing.T) {
	p := NewSelectDialogWidget(testCommands())

	if p.mode != paletteCommandMode {
		t.Fatal("expected command mode initially")
	}

	// Clear input to switch to file mode
	p.Input.Clear()
	if p.mode != paletteFileMode {
		t.Fatal("expected file mode after clearing input")
	}

	// Type ">" to switch back to command mode
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, ">", 0))
	if p.mode != paletteCommandMode {
		t.Fatal("expected command mode after typing '>'")
	}
	if len(p.Items) != 4 {
		t.Fatalf("expected 4 commands, got %d", len(p.Items))
	}
}

func testCommandsWithKeywords() []command.Command {
	return []command.Command{
		{ID: "fold.toggle", Title: "Toggle Fold", Keywords: []string{"editor", "collapse", "expand", "hide", "lines"}},
		{ID: "fold.collapseAll", Title: "Fold All", Keywords: []string{"editor", "collapse", "expand", "hide", "lines"}},
		{ID: "fold.expandAll", Title: "Unfold All", Keywords: []string{"editor", "collapse", "expand", "hide", "lines"}},
		{ID: "editor.undo", Title: "Undo", Keywords: []string{"editor", "revert"}},
		{ID: "editor.redo", Title: "Redo", Keywords: []string{"editor"}},
		{ID: "search.find", Title: "Find", Keywords: []string{"search", "find", "locate"}},
		{ID: "search.replace", Title: "Find and Replace", Keywords: []string{"search", "find", "replace", "substitute"}},
		{ID: "terminal.toggle", Title: "Terminal: Toggle Terminal", Keywords: []string{"terminal", "shell", "console", "bash"}},
		{ID: "settings.open", Title: "Preferences: Open Settings", Keywords: []string{"preferences", "settings", "configuration", "options"}},
	}
}

func TestPaletteKeywordMatch(t *testing.T) {
	p := NewSelectDialogWidget(testCommandsWithKeywords())

	// Type "collapse" which should match fold commands via keywords
	for _, r := range "collapse" {
		p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), 0))
	}

	found := false
	for _, item := range p.Items {
		if item.ID == "fold.toggle" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected keyword 'collapse' to match 'Toggle Fold'")
	}
}

func TestPaletteKeywordEditorCategory(t *testing.T) {
	p := NewSelectDialogWidget(testCommandsWithKeywords())

	// Type "editor" which should return editor-related commands via keywords
	for _, r := range "editor" {
		p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), 0))
	}

	if len(p.Items) == 0 {
		t.Fatal("expected 'editor' to return results via keyword matching")
	}

	// All fold commands and editor commands should be in results
	foundFold := false
	foundUndo := false
	for _, item := range p.Items {
		if item.ID == "fold.toggle" {
			foundFold = true
		}
		if item.ID == "editor.undo" {
			foundUndo = true
		}
	}
	if !foundFold {
		t.Fatal("expected 'editor' keyword to match fold.toggle")
	}
	if !foundUndo {
		t.Fatal("expected 'editor' keyword to match editor.undo")
	}
}

func TestPaletteKeywordNotDisplayed(t *testing.T) {
	p := NewSelectDialogWidget(testCommandsWithKeywords())

	// Type "collapse" to trigger keyword match
	for _, r := range "collapse" {
		p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), 0))
	}

	// Verify the label shows the Title, not the keyword
	for _, item := range p.Items {
		if item.ID == "fold.toggle" {
			if item.Label != "Toggle Fold" {
				t.Fatalf("expected label 'Toggle Fold', got '%s'", item.Label)
			}
			return
		}
	}
	t.Fatal("fold.toggle not found in results")
}

func TestPaletteKeywordCollapseMatchesFoldCommands(t *testing.T) {
	p := NewSelectDialogWidget(testCommandsWithKeywords())

	for _, r := range "collapse" {
		p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), 0))
	}

	foldIDs := map[string]bool{"fold.toggle": false, "fold.collapseAll": false, "fold.expandAll": false}
	for _, item := range p.Items {
		if _, ok := foldIDs[item.ID]; ok {
			foldIDs[item.ID] = true
		}
	}
	for id, found := range foldIDs {
		if !found {
			t.Fatalf("expected 'collapse' keyword to match %s", id)
		}
	}
}

func TestPaletteKeywordSubstitute(t *testing.T) {
	p := NewSelectDialogWidget(testCommandsWithKeywords())

	// "substitute" should match Find and Replace via keywords
	for _, r := range "substitute" {
		p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), 0))
	}

	found := false
	for _, item := range p.Items {
		if item.ID == "search.replace" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected keyword 'substitute' to match 'Find and Replace'")
	}
}

func TestPaletteTitleMatchPreferredOverKeyword(t *testing.T) {
	cmds := []command.Command{
		{ID: "a", Title: "Find", Keywords: []string{"search"}},
		{ID: "b", Title: "Search Panel", Keywords: []string{"find"}},
	}
	p := NewSelectDialogWidget(cmds)

	// "find" should match both, but "a" has it in the title (exact substring = higher score)
	for _, r := range "find" {
		p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), 0))
	}

	if len(p.Items) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(p.Items))
	}
	if p.Items[0].ID != "a" {
		t.Fatalf("expected title match 'Find' (id=a) to rank first, got id=%s", p.Items[0].ID)
	}
}

func TestPaletteTitleMatchRanksAboveKeywordOnly(t *testing.T) {
	cmds := []command.Command{
		{ID: "linenumbers", Title: "Toggle Line Numbers", Keywords: []string{"settings", "preferences"}},
		{ID: "settings", Title: "Preferences: Open Settings", Keywords: []string{"preferences", "configuration"}},
	}
	p := NewSelectDialogWidget(cmds)

	// "settings" is in the title of the second command, but only a keyword of the first.
	// The title match should rank higher.
	for _, r := range "settings" {
		p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), 0))
	}

	if len(p.Items) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(p.Items))
	}
	if p.Items[0].ID != "settings" {
		t.Fatalf("expected title match 'Preferences: Open Settings' (id=settings) to rank first, got id=%s (%s)", p.Items[0].ID, p.Items[0].Label)
	}
}

func TestPaletteHelpStartsWithOrientationTopics(t *testing.T) {
	p := NewSelectDialogWidget(helpTestCommands())

	// A question mark typed into the initial command prompt switches prefixes;
	// the user should not have to erase ">" first.
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "?", tcell.ModNone))

	if p.Input.Text != "?" {
		t.Fatalf("expected question-mark prefix, got %q", p.Input.Text)
	}
	if len(p.Items) == 0 {
		t.Fatal("expected orientation topics")
	}
	for _, item := range p.Items {
		if item.ID == "file.quickOpen" || item.ID == "sidebar.search" {
			t.Fatalf("empty help should show topics, not command %q", item.ID)
		}
	}
	if p.Items[0].Description == "" {
		t.Fatal("expected the selected topic to have orientation detail")
	}
}

func TestPaletteHelpSearchDerivesCommandsAndShortcuts(t *testing.T) {
	p := NewSelectDialogWidget(helpTestCommands())
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "?", tcell.ModNone))
	for _, r := range "Open Anything" {
		p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
	}

	if len(p.Items) == 0 {
		t.Fatal("expected command search result")
	}
	got := p.Items[0]
	if got.ID != "file.quickOpen" {
		t.Fatalf("expected derived command ID, got %q", got.ID)
	}
	if got.Label != "Open Anything From the Workspace" {
		t.Fatalf("expected registered title, got %q", got.Label)
	}
	if got.Detail != defaultShortcut("file.quickOpen") {
		t.Fatalf("expected shortcut derived from DefaultKeybindings, got %q", got.Detail)
	}
}

func TestPaletteHelpTopicDrillsIntoCommands(t *testing.T) {
	p := NewSelectDialogWidget(helpTestCommands())
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "?", tcell.ModNone))

	// The first topic is the workspace orientation topic. Enter reveals the
	// commands related to that topic instead of trying to execute the topic.
	p.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))

	if len(p.Items) == 0 {
		t.Fatal("expected commands after entering a help topic")
	}
	foundQuickOpen := false
	for _, item := range p.Items {
		if item.ID == "file.quickOpen" {
			foundQuickOpen = true
		}
	}
	if !foundQuickOpen {
		t.Fatal("workspace topic should drill into the derived quick-open command")
	}
}

func TestPaletteHelpRelevanceFloorRejectsFuzzyNoise(t *testing.T) {
	commands := []command.Command{
		{ID: "editor.undo", Title: "Undo", Shortcut: defaultShortcut("editor.undo")},
		{ID: "multicursor.undoCursor", Title: "Undo Last Cursor", Shortcut: defaultShortcut("multicursor.undoCursor")},
		{ID: "changes.discard", Title: "Git: Discard Changes", Keywords: []string{"git", "changes", "revert", "restore", "undo"}},
		{ID: "sourceAction.formatDocument", Title: "Source Action: Format Document"},
		{ID: "about", Title: "About TTT Editor"},
		{ID: "multicursor.selectNext", Title: "Add Next Occurrence"},
		{ID: "editor.indentation", Title: "Change Indentation"},
	}
	p := NewSelectDialogWidget(commands)
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "?", tcell.ModNone))
	for _, r := range "undo" {
		p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
	}

	if len(p.Items) != 2 {
		labels := make([]string, len(p.Items))
		for i, item := range p.Items {
			labels[i] = item.Label
		}
		t.Fatalf("expected only two relevant undo matches, got %d: %v", len(p.Items), labels)
	}
	if p.Items[0].ID != "editor.undo" || p.Items[1].ID != "multicursor.undoCursor" {
		t.Fatalf("unexpected undo result order: %q, %q", p.Items[0].ID, p.Items[1].ID)
	}

	// A metadata match remains discoverable when it is the best available
	// evidence; the relative floor only suppresses it beside stronger titles.
	p.Input.SetText("?revert")
	if len(p.Items) != 1 || p.Items[0].ID != "changes.discard" {
		t.Fatalf("expected exact keyword-only match, got %#v", p.Items)
	}
}

func TestPaletteHelpNoMatchesRendersGuidance(t *testing.T) {
	p := NewSelectDialogWidget(helpTestCommands())
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "?", tcell.ModNone))
	for _, r := range "qxzvjk" {
		p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
	}
	if len(p.Items) != 0 {
		t.Fatalf("expected no matches, got %d", len(p.Items))
	}

	cells := makeGrid(80, 24)
	p.Render(NewRenderSurface(cells, Rect{X: 0, Y: 0, W: 80, H: 24}))
	var rendered strings.Builder
	for _, row := range cells {
		for _, cell := range row {
			if cell.Ch == 0 {
				rendered.WriteByte(' ')
			} else {
				rendered.WriteRune(cell.Ch)
			}
		}
		rendered.WriteByte('\n')
	}
	text := rendered.String()
	if !strings.Contains(text, `No help entries match "qxzvjk"`) {
		t.Fatalf("missing no-results message:\n%s", text)
	}
	if !strings.Contains(text, "Try > for all commands") {
		t.Fatalf("missing command-mode hint:\n%s", text)
	}
}
