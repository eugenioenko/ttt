package term

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

type shiftCaptureTty struct {
	tcell.Tty
	strings.Builder
}

func (tty *shiftCaptureTty) Write(p []byte) (int, error) { return tty.Builder.Write(p) }

type shiftCaptureScreen struct {
	*SimScreen
	tty *shiftCaptureTty
}

func (s *shiftCaptureScreen) Tty() (tcell.Tty, bool) { return s.tty, true }
func (s *shiftCaptureScreen) Fini() {
	s.tty.WriteString("<fini>")
	s.SimScreen.Fini()
}
func (s *shiftCaptureScreen) Suspend() error {
	s.tty.WriteString("<suspend>")
	return nil
}
func (s *shiftCaptureScreen) Resume() error {
	s.tty.WriteString("<resume>")
	return nil
}

func TestTcellScreenMouseShiftCaptureLifecycle(t *testing.T) {
	tty := &shiftCaptureTty{}
	s := &shiftCaptureScreen{SimScreen: NewSimScreen(), tty: tty}
	tcell.ShimScreen(s)
	screen, err := NewTcellScreen()
	if err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	if !strings.Contains(tty.String(), "\x1b[>1s") {
		t.Error("startup did not request Shift mouse reporting")
	}
	tty.Reset()
	lifecycle, ok := any(screen).(interface {
		Suspend() error
		Resume() error
	})
	if !ok {
		t.Fatal("screen cannot restore Shift capture across suspend/resume")
	}
	if err := lifecycle.Suspend(); err != nil {
		t.Fatal(err)
	}
	if got := tty.String(); got != "\x1b[>0s<suspend>" {
		t.Fatalf("suspend output = %q", got)
	}
	tty.Reset()
	if err := lifecycle.Resume(); err != nil {
		t.Fatal(err)
	}
	if got := tty.String(); got != "<resume>\x1b[>1s" {
		t.Fatalf("resume output = %q", got)
	}
	tty.Reset()
	screen.Fini()
	if got := tty.String(); got != "\x1b[>0s<fini>" {
		t.Fatalf("exit output = %q", got)
	}
}
