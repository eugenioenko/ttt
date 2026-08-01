package view

import (
	"sort"
	"time"

	"github.com/eugenioenko/ttt/internal/term"
)

type NotifyLevel int

const (
	NotifyInfo NotifyLevel = iota
	NotifyWarning
	NotifyError
)

func (l NotifyLevel) Style() term.Style {
	switch l {
	case NotifyWarning:
		return term.StyleWarning
	case NotifyError:
		return term.StyleDanger
	default:
		return term.StyleStatusBar
	}
}

type StatusSegment struct {
	ID       string
	Side     string // "left" or "right"
	Priority int
	Text     string
	Style    term.Style
	OnClick  func()
}

type StatusBar struct {
	segments        map[string]*StatusSegment
	sortedLeft      []*StatusSegment
	sortedRight     []*StatusSegment
	dirty           bool
	Notification    string
	NotifyLevel     NotifyLevel
	NotifyExpiry    time.Time
	NotifyAction    func()
	ActionLabel     string
	SecondaryAction func()
	SecondaryLabel  string
	// EchoText, when non-empty, is rendered as the sole content of the status
	// bar. Used by plugins to display prompts (isearch, query-replace) without
	// competing with core mode-line segments for space.
	EchoText string
}

func NewStatusBar() *StatusBar {
	return &StatusBar{
		segments: make(map[string]*StatusSegment),
	}
}

func (s *StatusBar) SetSegment(seg StatusSegment) {
	if existing, ok := s.segments[seg.ID]; ok {
		existing.Text = seg.Text
		existing.Style = seg.Style
		if seg.OnClick != nil {
			existing.OnClick = seg.OnClick
		}
		if seg.Side != "" && seg.Side != existing.Side {
			existing.Side = seg.Side
			s.dirty = true
		}
		if seg.Priority != 0 && seg.Priority != existing.Priority {
			existing.Priority = seg.Priority
			s.dirty = true
		}
		return
	}
	cp := seg
	s.segments[seg.ID] = &cp
	s.dirty = true
}

func (s *StatusBar) RemoveSegment(id string) {
	if _, ok := s.segments[id]; ok {
		delete(s.segments, id)
		s.dirty = true
	}
}

func (s *StatusBar) LeftSegments() []*StatusSegment {
	if s.dirty {
		s.resort()
	}
	return s.sortedLeft
}

func (s *StatusBar) RightSegments() []*StatusSegment {
	if s.dirty {
		s.resort()
	}
	return s.sortedRight
}

func (s *StatusBar) resort() {
	s.sortedLeft = s.sortedLeft[:0]
	s.sortedRight = s.sortedRight[:0]
	for _, seg := range s.segments {
		if seg.Side == "left" {
			s.sortedLeft = append(s.sortedLeft, seg)
		} else {
			s.sortedRight = append(s.sortedRight, seg)
		}
	}
	sort.Slice(s.sortedLeft, func(i, j int) bool {
		return s.sortedLeft[i].Priority < s.sortedLeft[j].Priority
	})
	sort.Slice(s.sortedRight, func(i, j int) bool {
		return s.sortedRight[i].Priority < s.sortedRight[j].Priority
	})
	s.dirty = false
}

func (s *StatusBar) SetNotification(msg string, level NotifyLevel, duration time.Duration) {
	s.Notification = msg
	s.NotifyLevel = level
	s.NotifyExpiry = time.Now().Add(duration)
	s.NotifyAction = nil
	s.ActionLabel = ""
}

func (s *StatusBar) SetNotificationWithAction(msg string, level NotifyLevel, duration time.Duration, label string, action func()) {
	s.Notification = msg
	s.NotifyLevel = level
	s.NotifyExpiry = time.Now().Add(duration)
	s.ActionLabel = label
	s.NotifyAction = action
}

func (s *StatusBar) DismissNotification() {
	s.Notification = ""
	s.NotifyExpiry = time.Time{}
	s.NotifyAction = nil
	s.ActionLabel = ""
	s.SecondaryAction = nil
	s.SecondaryLabel = ""
}

func (s *StatusBar) IsNotificationActive() bool {
	if s.Notification == "" {
		return false
	}
	if !s.NotifyExpiry.IsZero() && time.Now().After(s.NotifyExpiry) {
		s.DismissNotification()
		return false
	}
	return true
}
