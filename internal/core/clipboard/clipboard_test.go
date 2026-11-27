package clipboard

import "testing"

func init() {
	DisableSystem()
}

func TestSetGet(t *testing.T) {
	Set("hello")
	if got := Get(); got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
}

func TestOverwrite(t *testing.T) {
	Set("first")
	Set("second")
	if got := Get(); got != "second" {
		t.Fatalf("expected 'second', got %q", got)
	}
}

func TestEmpty(t *testing.T) {
	content = ""
	if got := Get(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
