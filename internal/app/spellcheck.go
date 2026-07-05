package app

import (
	"context"
	"time"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/spell"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v2"
)

// SpellResult carries the async result of an aspell run.
type SpellResult struct {
	Gen   int
	Path  string
	Spans []ui.SpellSpan
	Err   string
}

// SpellTrigger is posted as an EventInterrupt to request a spell check on the
// main thread after a debounce delay.
type SpellTrigger struct{}

// ScheduleSpellCheck debounces spell checking during typing.
func (a *App) ScheduleSpellCheck() {
	if !a.Settings.Spell.IsEnabled() {
		return
	}
	if a.SpellTimer != nil {
		a.SpellTimer.Stop()
	}
	debounce := time.Duration(a.Settings.Spell.Debounce) * time.Millisecond
	a.SpellTimer = time.AfterFunc(debounce, func() {
		a.Screen.PostEvent(tcell.NewEventInterrupt(&SpellTrigger{}))
	})
}

// RequestSpellCheck triggers an async aspell run over the active file. The
// result is posted as a SpellResult via EventInterrupt.
func (a *App) RequestSpellCheck() {
	if !a.Settings.Spell.IsEnabled() || !spell.Available() {
		return
	}
	path := a.EditorGroup.ActiveFilePath()
	buf := a.EditorGroup.ActiveBuffer()
	if path == "" || buf == nil || a.EditorGroup.IsActiveVirtual() {
		return
	}
	lang := ""
	if a.EditorGroup.Editor.Highlighter != nil {
		lang = a.EditorGroup.Editor.Highlighter.Language()
	}
	mode, ok := spell.ModeForLanguage(lang)
	if !ok {
		a.EditorGroup.SetMisspellings(path, nil)
		return
	}

	a.SpellGen++
	gen := a.SpellGen
	linesCopy := make([]string, len(buf.Lines))
	copy(linesCopy, buf.Lines)
	dict := a.Settings.Spell.Lang

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		found, err := spell.Check(ctx, linesCopy, dict, mode)
		if err != nil {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&SpellResult{Gen: gen, Path: path, Err: err.Error()}))
			return
		}
		spans := make([]ui.SpellSpan, len(found))
		for i, m := range found {
			spans[i] = ui.SpellSpan{
				Line:        m.Line,
				Col:         m.Col,
				Len:         len([]rune(m.Word)),
				Word:        m.Word,
				Suggestions: m.Suggestions,
			}
		}
		a.Screen.PostEvent(tcell.NewEventInterrupt(&SpellResult{Gen: gen, Path: path, Spans: spans}))
	}()
}

// HandleSpellResult applies an async spell result on the main thread.
func (a *App) HandleSpellResult(v *SpellResult) {
	if v.Err != "" {
		if !a.spellErrShown {
			a.spellErrShown = true
			a.StatusError("Spell check: " + v.Err)
		}
		return
	}
	if v.Gen == a.SpellGen {
		a.EditorGroup.SetMisspellings(v.Path, v.Spans)
	}
}

func (a *App) spellAtCursor() *ui.SpellSpan {
	if !a.EditorGroup.IsEditorActive() {
		return nil
	}
	editor := a.EditorGroup.Editor
	return editor.MisspellingAt(editor.Cursor.Line, editor.Cursor.Col)
}

// SpellSuggest shows correction suggestions for the misspelled word at the
// cursor in an autocomplete popup.
func (a *App) SpellSuggest() {
	m := a.spellAtCursor()
	if m == nil {
		a.StatusNotify("No misspelling at cursor")
		return
	}
	if len(m.Suggestions) == 0 {
		a.StatusNotify("No suggestions for \"" + m.Word + "\"")
		return
	}
	items := make([]ui.CompletionItem, len(m.Suggestions))
	for i, s := range m.Suggestions {
		items[i] = ui.CompletionItem{Label: s, InsertText: s}
	}
	fix := *m
	// Not routed through ShowAutocomplete: prefix filtering would drop
	// corrections that don't share a prefix with the misspelled word.
	a.CompletionItems = nil
	ac := ui.NewAutocompleteWidget(items, 0, 0)
	ac.OnSelect = func(item ui.CompletionItem) {
		a.DismissAutocomplete()
		a.ApplySpellFix(fix, item.InsertText)
	}
	ac.OnDismiss = func() {
		a.DismissAutocomplete()
	}
	a.EditorGroup.Autocomplete = ac
}

// ApplySpellFix replaces a misspelled word with the chosen correction as a
// single undoable edit. Spans can go stale between the async check and the
// user acting on one, so the word is verified in place first.
func (a *App) ApplySpellFix(m ui.SpellSpan, replacement string) {
	if !a.EditorGroup.IsEditorActive() {
		return
	}
	editor := a.EditorGroup.Editor
	if m.Line >= len(editor.Buf.Lines) {
		return
	}
	runes := []rune(editor.Buf.Lines[m.Line])
	if m.Col+m.Len > len(runes) || string(runes[m.Col:m.Col+m.Len]) != m.Word {
		a.RequestSpellCheck()
		return
	}
	if editor.Undo != nil {
		editor.Undo.BreakGroup()
	}
	editor.ExecCommand(&undo.BatchCommand{Commands: []undo.EditCommand{
		&undo.DeleteSelectionCommand{
			StartLine: m.Line, StartCol: m.Col,
			EndLine: m.Line, EndCol: m.Col + m.Len,
		},
		&undo.PasteCommand{
			Line: m.Line, Col: m.Col,
			Text: replacement, Suffix: string(runes[m.Col+m.Len:]),
		},
	}})
	editor.Cursor.Line = m.Line
	editor.Cursor.Col = m.Col + len([]rune(replacement))
	editor.FlushOnChange()
	a.RequestSpellCheck()
}

// SpellAddWord adds the misspelled word at the cursor to the personal
// aspell dictionary.
func (a *App) SpellAddWord() {
	m := a.spellAtCursor()
	if m == nil {
		a.StatusNotify("No misspelling at cursor")
		return
	}
	a.AddSpellWord(m.Word)
}

func (a *App) AddSpellWord(word string) {
	if err := spell.AddToDictionary(a.Settings.Spell.Lang, word); err != nil {
		a.StatusError("Failed to add \"" + word + "\" to dictionary: " + err.Error())
		return
	}
	a.StatusNotify("Added \"" + word + "\" to dictionary")
	a.RequestSpellCheck()
}

func registerSpellCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "spell.suggest", Title: "Spell: Suggest Corrections",
		Keywords: []string{"spelling", "typo", "aspell", "fix"},
		Handler:  app.SpellSuggest,
	})

	reg.Register(command.Command{
		ID: "spell.addWord", Title: "Spell: Add Word to Dictionary",
		Keywords: []string{"spelling", "personal", "aspell", "ignore"},
		Handler:  app.SpellAddWord,
	})
}
