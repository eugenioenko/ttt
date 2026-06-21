package view

import (
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

type StatusBar struct {
	FileName        string
	Line            int
	Col             int
	Dirty           bool
	Branch          string
	Blame           string
	Language        string
	LSP             bool
	TabSize         int
	UseTabs         bool
	LineEnding      string
	CursorCount     int
	Notification    string
	NotifyLevel     NotifyLevel
	NotifyExpiry    time.Time
	NotifyAction    func()
	ActionLabel     string
	SecondaryAction func()
	SecondaryLabel  string
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
