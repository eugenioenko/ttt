package ui

import (
	"fmt"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

type ProblemItem struct {
	File     string
	Line     int
	Col      int
	Severity DiagnosticSeverity
	Message  string
	Source   string
}

type ProblemsWidget struct {
	Items      []ProblemItem
	OnNavigate func(file string, line, col int)
	tree       *widgets.TreeWidget
}

func NewProblemsWidget() *ProblemsWidget {
	p := &ProblemsWidget{}
	p.tree = widgets.NewListWidgetFromConfig(widgets.ListConfig{
		EmptyText: i18n.Text("problems.noProblemsDetected"),
		RenderItem: func(surface widgets.Surface, node *widgets.TreeNode, idx, y, w int, selected bool) {
			p.renderLine(surface, idx, y, w, selected)
		},
		OnCommand: func(command string, node *widgets.TreeNode) {
			if command == "activate" {
				p.navigateSelected()
			}
		},
	})
	return p
}

func (p *ProblemsWidget) Focusable() bool                 { return true }
func (p *ProblemsWidget) SetFocused(f bool)               { p.tree.SetFocused(f) }
func (p *ProblemsWidget) IsFocused() bool                 { return p.tree.IsFocused() }
func (p *ProblemsWidget) GetRect() Rect                   { return Rect(p.tree.GetRect()) }
func (p *ProblemsWidget) SetRect(r Rect)                  { p.tree.SetRect(widgets.Rect(r)) }
func (p *ProblemsWidget) Height() int                     { return 0 }
func (p *ProblemsWidget) Width() int                      { return 0 }
func (p *ProblemsWidget) SetBoxModel(bm widgets.BoxModel) { p.tree.SetBoxModel(bm) }
func (p *ProblemsWidget) Render(surface Surface)          { p.tree.Render(surface) }
func (p *ProblemsWidget) HandleEvent(ev tcell.Event) EventResult {
	return EventResult(p.tree.HandleEvent(ev))
}

func (p *ProblemsWidget) PointerShapeAt(mx, my int) string {
	if p.tree == nil {
		return ""
	}
	return p.tree.PointerShapeAt(mx, my)
}

func (p *ProblemsWidget) SetItems(items []ProblemItem) {
	p.Items = items
	nodes := make([]*widgets.TreeNode, len(items))
	for i, item := range items {
		nodes[i] = &widgets.TreeNode{
			ID:    fmt.Sprintf("%s:%d:%d", item.File, item.Line, item.Col),
			Label: item.Message,
		}
	}
	p.tree.SetItems(nodes)
}

func (p *ProblemsWidget) HasProblems() bool {
	for _, item := range p.Items {
		if item.Severity <= DiagWarning {
			return true
		}
	}
	return false
}

func (p *ProblemsWidget) navigateSelected() {
	idx := p.tree.SelectedIndex()
	if idx >= 0 && idx < len(p.Items) && p.OnNavigate != nil {
		item := p.Items[idx]
		p.OnNavigate(item.File, item.Line, item.Col)
	}
}

func (p *ProblemsWidget) renderLine(surface widgets.Surface, idx, y, w int, selected bool) {
	if idx >= len(p.Items) {
		return
	}
	item := p.Items[idx]

	style := term.StyleDefault
	if selected {
		style = term.StyleSidebarSelected
	}

	for x := 0; x < w; x++ {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
	}

	x := 1

	icon := 'E'
	iconStyle := term.StyleDanger
	switch item.Severity {
	case DiagWarning:
		icon = 'W'
		iconStyle = term.StyleWarning
	case DiagInformation:
		icon = 'I'
		iconStyle = term.StyleMuted
	case DiagHint:
		icon = 'H'
		iconStyle = term.StyleMuted
	}
	surface.SetCell(x, y, term.Cell{Ch: icon, Style: iconStyle})
	x += 2

	loc := fmt.Sprintf("%s:%d", filepath.Base(item.File), item.Line+1)
	for _, ch := range loc {
		if x >= w {
			break
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: style})
		x++
	}
	x++

	msgStyle := style
	if !selected {
		msgStyle = term.StyleMuted
	}
	for _, ch := range item.Message {
		if x >= w {
			break
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: msgStyle})
		x++
	}
}
