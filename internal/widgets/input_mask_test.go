package widgets

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

func TestBorderedInputTitleAndMask(t *testing.T) {
	inp := NewInputWidget(InputConfig{Bordered: true, Title: "Secret", Mask: true})
	inp.SetText("hunter2")
	s := renderWidget(inp, 0, 0, 20, 3)
	if got := rowText(s, 0); !strings.HasPrefix(got, "╭ Secret ─") {
		t.Fatalf("top border = %q", got)
	}
	if got := rowText(s, 1); !strings.Contains(got, "•••••••") || strings.Contains(got, "hunter2") {
		t.Fatalf("masked text row = %q", got)
	}
}

func TestMaskedInputKeepsTextOffTheClipboard(t *testing.T) {
	clipboard.DisableSystem()
	clipboard.Set("before")
	inp := NewInputWidget(InputConfig{Mask: true})
	inp.SetText("hunter2")
	inp.selectAll()
	inp.CopySelection()
	inp.CutSelection()
	if got := clipboard.Get(); got != "before" {
		t.Fatalf("clipboard = %q", got)
	}
	if inp.Text() != "hunter2" {
		t.Fatalf("cut removed masked text: %q", inp.Text())
	}
}

func TestMaskedInputCursorFollowsTheBullets(t *testing.T) {
	inp := NewInputWidget(InputConfig{Mask: true})
	inp.SetText("日本")
	inp.SetFocused(true)
	renderWidget(inp, 0, 0, 20, 1)
	want := textwidth.String(inp.Config.Prefix) + 2
	if x, _, _ := inp.CursorPosition(); x != want {
		t.Fatalf("cursor x = %d, want %d: one column per bullet", x, want)
	}
}

func TestMaskedInputIsOneLine(t *testing.T) {
	inp := NewInputWidget(InputConfig{Mask: true, Wrap: true})
	inp.SetText(strings.Repeat("x", 50))
	renderWidget(inp, 0, 0, 10, 5)
	if h := inp.Height(); h != 1 {
		t.Fatalf("height = %d, want 1", h)
	}
	inp.SetText("")
	inp.PasteText("a\nb")
	if inp.Text() != "a b" {
		t.Fatalf("pasted text = %q, want the line break replaced", inp.Text())
	}
}

func TestMaskedInputWordMotionTreatsTextAsOneWord(t *testing.T) {
	inp := NewInputWidget(InputConfig{Mask: true})
	inp.SetText("correct horse")
	inp.SetFocused(true)
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, "", tcell.ModCtrl))
	if inp.cursorPos != 0 {
		t.Fatalf("ctrl+left stopped at %d, want 0", inp.cursorPos)
	}
}
