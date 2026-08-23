package ui

type scrollbarCaptureOwner interface {
	CancelPointerCapture() bool
	OwnsPointerCapture() bool
}

type scrollbarCaptureState struct {
	invalidated func()
}

func (s *scrollbarCaptureState) cancel(bars ...scrollbarCaptureOwner) bool {
	canceled := false
	for _, bar := range bars {
		canceled = bar.CancelPointerCapture() || canceled
	}
	s.notify(canceled)
	return canceled
}

func (s *scrollbarCaptureState) owns(bars ...scrollbarCaptureOwner) bool {
	for _, bar := range bars {
		if bar.OwnsPointerCapture() {
			return true
		}
	}
	return false
}

func (s *scrollbarCaptureState) notify(invalidated bool) {
	if invalidated && s.invalidated != nil {
		s.invalidated()
	}
}
