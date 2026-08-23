package command

import "testing"

func TestRegisterAndExecute(t *testing.T) {
	r := NewRegistry()
	called := false
	r.Register(Command{ID: "test.run", Title: "Run Tests", Handler: func() { called = true }})

	if !r.Execute("test.run") {
		t.Fatal("Execute returned false for registered command")
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestExecuteUnknown(t *testing.T) {
	r := NewRegistry()
	if r.Execute("nonexistent") {
		t.Fatal("Execute should return false for unknown command")
	}
}

func TestGet(t *testing.T) {
	r := NewRegistry()
	r.Register(Command{ID: "file.save", Title: "Save File", Handler: func() {}})

	cmd, ok := r.Get("file.save")
	if !ok {
		t.Fatal("Get returned false for registered command")
	}
	if cmd.Title != "Save File" {
		t.Fatalf("expected title 'Save File', got '%s'", cmd.Title)
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Fatal("Get should return false for unknown command")
	}
}

func TestList(t *testing.T) {
	r := NewRegistry()
	r.Register(Command{ID: "a", Title: "A", Handler: func() {}})
	r.Register(Command{ID: "b", Title: "B", Handler: func() {}})
	hiddenCalled := false
	r.Register(Command{ID: "legacy", Title: "Legacy", Hidden: true, Handler: func() { hiddenCalled = true }})

	cmds := r.List()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
	if !r.Execute("legacy") || !hiddenCalled {
		t.Fatal("hidden compatibility command was not executable by ID")
	}
}
