package ui

import (
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func (e *EditorPaneWidget) handleKey(kev *tcell.EventKey) EventResult {
	e.Cursor.Line = e.Buf.ClampLine(e.Cursor.Line)

	switch kev.Key() {
	case tcell.KeyRune, tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete:
	default:
		if e.Undo != nil {
			e.Undo.BreakGroup()
		}
	}

	mods := kev.Modifiers()
	if mods&tcell.ModAlt != 0 || mods&tcell.ModCtrl != 0 {
		return EventIgnored
	}

	shift := mods&tcell.ModShift != 0
	multi := e.isMultiActive()

	switch kev.Key() {
	case tcell.KeyUp:
		if multi {
			e.multiMoveAll(func(cs *multicursor.CursorState) {
				if cs.Line > 0 {
					cs.Line--
					if e.Folds != nil {
						if r := e.Folds.ContainingFold(cs.Line); r != nil {
							cs.Line = r.StartLine
						}
					}
					lineLen := len([]rune(e.Buf.Lines[cs.Line]))
					if cs.Col > lineLen {
						cs.Col = lineLen
					}
				}
			})
		} else {
			e.startOrExtendSelection(shift)
			if e.Cursor.Line > 0 {
				e.Cursor.Line--
				e.skipHiddenLineUp()
				lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
				if e.Cursor.Col > lineLen {
					e.Cursor.Col = lineLen
				}
			}
		}
	case tcell.KeyDown:
		if multi {
			e.multiMoveAll(func(cs *multicursor.CursorState) {
				if cs.Line < len(e.Buf.Lines)-1 {
					cs.Line++
					if e.Folds != nil {
						if r := e.Folds.ContainingFold(cs.Line); r != nil {
							cs.Line = e.Buf.ClampLine(r.EndLine + 1)
						}
					}
					lineLen := len([]rune(e.Buf.Lines[cs.Line]))
					if cs.Col > lineLen {
						cs.Col = lineLen
					}
				}
			})
		} else {
			e.startOrExtendSelection(shift)
			if e.Cursor.Line < len(e.Buf.Lines)-1 {
				e.Cursor.Line++
				e.skipHiddenLineDown()
				lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
				if e.Cursor.Col > lineLen {
					e.Cursor.Col = lineLen
				}
			}
		}
	case tcell.KeyLeft:
		if multi {
			e.multiMoveAll(func(cs *multicursor.CursorState) {
				if cs.Col > 0 {
					cs.Col--
				} else if cs.Line > 0 {
					cs.Line--
					cs.Col = len([]rune(e.Buf.Lines[cs.Line]))
				}
			})
		} else {
			e.startOrExtendSelection(shift)
			if e.Cursor.Col > 0 {
				e.Cursor.Col--
			} else if e.Cursor.Line > 0 {
				e.Cursor.Line--
				e.Cursor.Col = len([]rune(e.Buf.Lines[e.Cursor.Line]))
			}
		}
	case tcell.KeyRight:
		if multi {
			e.multiMoveAll(func(cs *multicursor.CursorState) {
				lineLen := len([]rune(e.Buf.Lines[cs.Line]))
				if cs.Col < lineLen {
					cs.Col++
				} else if cs.Line < len(e.Buf.Lines)-1 {
					cs.Line++
					cs.Col = 0
				}
			})
		} else {
			e.startOrExtendSelection(shift)
			lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
			if e.Cursor.Col < lineLen {
				e.Cursor.Col++
			} else if e.Cursor.Line < len(e.Buf.Lines)-1 {
				e.Cursor.Line++
				e.Cursor.Col = 0
			}
		}
	case tcell.KeyHome:
		if multi {
			e.multiMoveAll(func(cs *multicursor.CursorState) {
				runes := []rune(e.Buf.Lines[cs.Line])
				firstNonSpace := 0
				for firstNonSpace < len(runes) && (runes[firstNonSpace] == ' ' || runes[firstNonSpace] == '\t') {
					firstNonSpace++
				}
				if cs.Col == firstNonSpace {
					cs.Col = 0
				} else {
					cs.Col = firstNonSpace
				}
			})
		} else {
			e.startOrExtendSelection(shift)
			e.SmartHome()
		}
	case tcell.KeyEnd:
		if multi {
			e.multiMoveAll(func(cs *multicursor.CursorState) {
				cs.Col = len([]rune(e.Buf.Lines[cs.Line]))
			})
		} else {
			e.startOrExtendSelection(shift)
			e.Cursor.Col = len([]rune(e.Buf.Lines[e.Cursor.Line]))
		}
	case tcell.KeyPgUp:
		e.startOrExtendSelection(shift)
		e.Cursor.Line = e.Buf.ClampLine(e.Cursor.Line - e.Viewport.Height)
		e.skipHiddenLineUp()
		lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
		if e.Cursor.Col > lineLen {
			e.Cursor.Col = lineLen
		}
	case tcell.KeyPgDn:
		e.startOrExtendSelection(shift)
		e.Cursor.Line = e.Buf.ClampLine(e.Cursor.Line + e.Viewport.Height)
		e.skipHiddenLineDown()
		lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
		if e.Cursor.Col > lineLen {
			e.Cursor.Col = lineLen
		}
	case tcell.KeyEnter:
		if e.ReadOnly {
			return EventConsumed
		}
		e.expandFoldAtCursor()
		if multi {
			e.multiExecEnter()
		} else {
			e.execEnter()
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if e.ReadOnly {
			return EventConsumed
		}
		e.expandFoldAtCursor()
		if multi {
			e.multiExecBackspace()
		} else {
			e.execBackspace()
		}
	case tcell.KeyDelete:
		if e.ReadOnly {
			return EventConsumed
		}
		e.expandFoldAtCursor()
		if multi {
			e.multiExecDelete()
		} else {
			e.execDelete()
		}
	case tcell.KeyRune:
		if kev.Modifiers() != 0 {
			return EventIgnored
		}
		if e.ReadOnly {
			return EventConsumed
		}
		e.expandFoldAtCursor()
		if r := term.KeyRune(kev); r != 0 {
			if multi {
				e.multiExecRune(r)
			} else {
				e.execRune(r)
			}
		}
	case tcell.KeyBacktab:
		if e.ReadOnly {
			return EventConsumed
		}
		if multi {
			// Outdent under multiple cursors is a no-op (see #371); backspace
			// covers per-cursor de-indentation.
			break
		}
		tabSize := e.resolveTabSize()
		if startLine, endLine, ok := e.selectedLineRange(); ok {
			var cmds []undo.EditCommand
			for line := startLine; line <= endLine; line++ {
				remove := leadingIndentWidth(e.Buf.Lines[line], tabSize)
				if remove > 0 {
					cmd := &undo.DeleteSelectionCommand{
						StartLine: line, StartCol: 0,
						EndLine: line, EndCol: remove,
					}
					cmd.Apply(e.Buf)
					cmds = append(cmds, cmd)
				}
			}
			if len(cmds) > 0 && e.Undo != nil {
				e.Undo.Push(&undo.BatchCommand{Commands: cmds})
				e.bufferDirty = true
			}
		} else {
			remove := leadingIndentWidth(e.Buf.Lines[e.Cursor.Line], tabSize)
			if remove > 0 {
				e.exec(&undo.DeleteSelectionCommand{
					StartLine: e.Cursor.Line, StartCol: 0,
					EndLine: e.Cursor.Line, EndCol: remove,
				})
				e.Cursor.Col -= remove
				if e.Cursor.Col < 0 {
					e.Cursor.Col = 0
				}
			}
		}
	case tcell.KeyTab:
		if e.ReadOnly {
			return EventConsumed
		}
		if multi {
			e.multiExecTab()
			break
		}
		indent := e.indentUnit()
		if startLine, endLine, ok := e.selectedLineRange(); ok {
			var cmds []undo.EditCommand
			for line := startLine; line <= endLine; line++ {
				cmd := &undo.InsertStringCommand{Line: line, Col: 0, Text: indent}
				cmd.Apply(e.Buf)
				cmds = append(cmds, cmd)
			}
			if len(cmds) > 0 && e.Undo != nil {
				e.Undo.Push(&undo.BatchCommand{Commands: cmds})
				e.bufferDirty = true
			}
		} else {
			e.exec(&undo.InsertStringCommand{Line: e.Cursor.Line, Col: e.Cursor.Col, Text: indent})
			e.Cursor.Col += len([]rune(indent))
		}
	default:
		return EventIgnored
	}

	e.clampCursor()
	e.scrollViewport()
	return EventConsumed
}

// execEnter splits the current line at the cursor for a single cursor, applying
// auto-indentation to the new line when enabled.
func (e *EditorPaneWidget) execEnter() {
	if e.Selection != nil && e.Selection.Active {
		e.deleteSelection()
	}
	col := e.Cursor.Col
	if col < 0 {
		col = 0
	}
	lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
	if col > lineLen {
		col = lineLen
	}
	line := e.Buf.Lines[e.Cursor.Line]

	var cmds []undo.EditCommand
	splitCmd := &undo.SplitLineCommand{Line: e.Cursor.Line, Col: col}
	splitCmd.Apply(e.Buf)
	cmds = append(cmds, splitCmd)
	e.Cursor.Line++
	e.Cursor.Col = 0

	// Auto-indent (indent preservation plus a shiftwidth after an opening
	// bracket) is gated on AutoIndent. With it off, Enter opens a column-1 line,
	// matching an editor with `noautoindent` — which the Vim layer relies on.
	if e.AutoIndent {
		indent := leadingWhitespace(line)
		newIndent := indent
		runes := []rune(line)
		charBefore := ' '
		if col > 0 && col <= len(runes) {
			charBefore = runes[col-1]
		}
		charAfter := ' '
		if col < len(runes) {
			charAfter = runes[col]
		}
		extraIndent := indentOpeners[charBefore]
		if extraIndent {
			newIndent += e.indentUnit()
		}
		if len(newIndent) > 0 {
			indentCmd := &undo.InsertStringCommand{Line: e.Cursor.Line, Col: 0, Text: newIndent}
			indentCmd.Apply(e.Buf)
			cmds = append(cmds, indentCmd)
			e.Cursor.Col = len([]rune(newIndent))
		}
		if extraIndent && closingBrackets[charAfter] {
			bracketSplit := &undo.SplitLineCommand{Line: e.Cursor.Line, Col: e.Cursor.Col}
			bracketSplit.Apply(e.Buf)
			cmds = append(cmds, bracketSplit)
			bracketIndent := &undo.InsertStringCommand{Line: e.Cursor.Line + 1, Col: 0, Text: indent}
			bracketIndent.Apply(e.Buf)
			cmds = append(cmds, bracketIndent)
		}
	}
	if e.Undo != nil {
		e.Undo.Push(&undo.BatchCommand{Commands: cmds})
	}
	e.bufferDirty = true
	if e.Folds != nil {
		e.Folds.SetRanges(fold.ComputeIndentRanges(e.Buf.Lines))
	}
}

// execBackspace deletes the selection, the previous character (with soft-tab
// awareness in leading whitespace), or joins with the previous line.
func (e *EditorPaneWidget) execBackspace() {
	if e.Selection != nil && e.Selection.Active {
		e.deleteSelection()
	} else if e.Cursor.Col > 0 {
		runes := []rune(e.Buf.Lines[e.Cursor.Line])
		if e.Cursor.Col > len(runes) {
			e.Cursor.Col = len(runes)
		}
		inLeadingWhitespace := true
		for i := 0; i < e.Cursor.Col && i < len(runes); i++ {
			if runes[i] != ' ' && runes[i] != '\t' {
				inLeadingWhitespace = false
				break
			}
		}
		tabSize := e.resolveTabSize()
		if inLeadingWhitespace && e.Cursor.Col > 1 && runes[e.Cursor.Col-1] == ' ' {
			target := ((e.Cursor.Col - 1) / tabSize) * tabSize
			if target == e.Cursor.Col {
				target -= tabSize
			}
			if target < 0 {
				target = 0
			}
			e.exec(&undo.DeleteSelectionCommand{
				StartLine: e.Cursor.Line, StartCol: target,
				EndLine: e.Cursor.Line, EndCol: e.Cursor.Col,
			})
			e.Cursor.Col = target
		} else {
			e.exec(&undo.DeleteRuneCommand{Line: e.Cursor.Line, Col: e.Cursor.Col - 1})
			e.Cursor.Col--
		}
	} else if e.Cursor.Line > 0 {
		cmd := &undo.JoinLineCommand{Line: e.Cursor.Line}
		e.exec(cmd)
		e.Cursor.Line--
		e.Cursor.Col = cmd.PrevLen
	}
}

// execDelete deletes the selection, the character under the cursor, or joins the
// next line when at end of line.
func (e *EditorPaneWidget) execDelete() {
	if e.Selection != nil && e.Selection.Active {
		e.deleteSelection()
		return
	}
	lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
	if e.Cursor.Col < lineLen {
		e.exec(&undo.DeleteRuneCommand{Line: e.Cursor.Line, Col: e.Cursor.Col})
	} else if e.Cursor.Line < len(e.Buf.Lines)-1 {
		e.exec(&undo.JoinLineCommand{Line: e.Cursor.Line + 1})
	}
}

// execRune inserts a rune for a single cursor, replacing the selection if one is
// active and dedenting first when a closing bracket is typed.
func (e *EditorPaneWidget) execRune(r rune) {
	if e.Selection != nil && e.Selection.Active {
		start, _ := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
		e.replaceSelection(&undo.InsertRuneCommand{Line: start.Line, Col: start.Col, Rune: r})
		e.Cursor.Col = start.Col + 1
		return
	}
	if e.AutoDedent && closingBrackets[r] {
		e.dedentForCloser()
	}
	e.exec(&undo.InsertRuneCommand{Line: e.Cursor.Line, Col: e.Cursor.Col, Rune: r})
	e.Cursor.Col++
}
