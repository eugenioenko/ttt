package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileHandlerClaimsBinaryFiles(t *testing.T) {
	dir := t.TempDir()
	pdf := filepath.Join(dir, "paper.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.7\n\x00\x01\x02"), 0644); err != nil {
		t.Fatal(err)
	}

	g := NewEditorGroupWidget(nil, 4, true, "relative")
	var gotPath string
	var gotHead []byte
	calls := 0
	claimed := NewBinaryFileWidget("claimed")
	g.FileHandler = func(path string, head []byte) Widget {
		calls++
		gotPath, gotHead = path, head
		return claimed
	}

	g.OpenFile(pdf)
	if tab := g.activeTab(); tab == nil || tab.Content != claimed || tab.FilePath != pdf {
		t.Fatalf("active tab = %+v, want the handler's widget bound to %s", tab, pdf)
	}
	if gotPath != pdf || string(gotHead[:5]) != "%PDF-" {
		t.Errorf("handler got path %q head %q", gotPath, gotHead)
	}

	g.OpenFile(pdf)
	if calls != 1 {
		t.Errorf("reopening an open file called the handler %d times, want 1", calls)
	}
}

func TestFileHandlerDeclineKeepsBinaryCard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blob.bin")
	if err := os.WriteFile(path, []byte{0x00, 0x01, 0xff}, 0644); err != nil {
		t.Fatal(err)
	}
	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.FileHandler = func(string, []byte) Widget { return nil }

	g.OpenFile(path)
	if _, ok := g.activeTab().Content.(*BinaryFileWidget); !ok {
		t.Fatalf("content = %T, want *BinaryFileWidget", g.activeTab().Content)
	}
}

func TestFileHandlerNotConsultedForTextOrImages(t *testing.T) {
	dir := t.TempDir()
	txt := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(txt, []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(dir, "pic.png")
	writeTestPNG(t, img, 4, 4)

	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.FileHandler = func(string, []byte) Widget {
		t.Fatal("handler consulted for a text or image file")
		return nil
	}
	g.OpenFile(txt)
	if g.activeTab().Buf == nil {
		t.Fatal("text file should open as a buffer")
	}
	g.OpenFile(img)
	if _, ok := g.activeTab().Content.(*ImageViewWidget); !ok {
		t.Fatalf("image file content = %T, want *ImageViewWidget", g.activeTab().Content)
	}
}

func TestRetryFileHandlersClaimsOpenBinaryTabs(t *testing.T) {
	dir := t.TempDir()
	pdf := filepath.Join(dir, "paper.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.7\n\x00\x01"), 0644); err != nil {
		t.Fatal(err)
	}
	txt := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(txt, []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}

	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.OpenFile(pdf)
	g.OpenFile(pdf)
	g.OpenFile(txt)
	if _, ok := g.tabs[tabAt(g, pdf)].Content.(*BinaryFileWidget); !ok {
		t.Fatal("without a handler the PDF should open as the binary card")
	}

	claimed := NewBinaryFileWidget("claimed")
	g.FileHandler = func(path string, head []byte) Widget {
		if path != pdf {
			t.Errorf("handler offered %s", path)
		}
		return claimed
	}
	g.RetryFileHandlers()
	if g.tabs[tabAt(g, pdf)].Content != claimed {
		t.Error("the open binary tab was not handed to the file handler")
	}
	if g.tabs[tabAt(g, txt)].Buf == nil {
		t.Error("the text tab lost its buffer")
	}
}

func tabAt(g *EditorGroupWidget, path string) int {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path {
			return i
		}
	}
	panic("no tab for " + path)
}

type focusProbe struct {
	*BinaryFileWidget
	focused bool
}

func (f *focusProbe) SetFocused(focused bool) { f.focused = focused }

func TestFileHandledFiresAfterTabIsActive(t *testing.T) {
	dir := t.TempDir()
	pdf := filepath.Join(dir, "paper.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.7\n\x00\x01\x02"), 0644); err != nil {
		t.Fatal(err)
	}
	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.FileHandler = func(path string, _ []byte) Widget { return NewBinaryFileWidget(path) }
	var activeAtEvent string
	g.OnFileHandled = func(string) { activeAtEvent = g.ActiveFilePath() }

	g.OpenFile(pdf)
	if activeAtEvent != pdf {
		t.Errorf("active file at OnFileHandled = %q, want %q", activeAtEvent, pdf)
	}
}

func TestReplacedContentTakesFocus(t *testing.T) {
	dir := t.TempDir()
	pdf := filepath.Join(dir, "paper.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.7\n\x00\x01\x02"), 0644); err != nil {
		t.Fatal(err)
	}
	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.OpenFile(pdf)
	g.SetFocused(true)

	probe := &focusProbe{BinaryFileWidget: NewBinaryFileWidget(pdf)}
	g.FileHandler = func(string, []byte) Widget { return probe }
	g.RetryFileHandlers()
	if !probe.focused {
		t.Fatal("claimed content in the focused active tab was not focused")
	}

	g.ShowBinaryCard(pdf)
	if probe.focused {
		t.Error("replaced content kept focus")
	}
}
