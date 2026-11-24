package dap

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDelveLaunch(t *testing.T) {
	binary, err := exec.LookPath("dlv")
	if err != nil {
		t.Skip("dlv unavailable")
	}
	folder := t.TempDir()
	path := filepath.Join(folder, "main.go")
	for name, data := range map[string]string{"main.go": "package main\nimport \"fmt\"\nfunc main(){\n value:=42\n fmt.Println(value)\n}\n", "go.mod": "module fixture\ngo 1.25\n"} {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	events := make(chan Event, 256)
	session, err := StartDelve(ctx, binary, folder, func(e Event) { events <- e })
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close(true)
	target := Target{Type: "go", Request: "launch", Folder: folder, Arguments: map[string]any{"program": "."}}
	args, err := target.LaunchArguments()
	if err != nil {
		t.Fatal(err)
	}
	err = session.Start(ctx, "go", "launch", args, func() error {
		points, err := session.SetBreakpoints(ctx, path, []int{5})
		if err == nil && (len(points) != 1 || !points[0].Verified) {
			t.Errorf("breakpoint: %+v", points)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var thread int
	var output string
	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case event := <-events:
			switch event.Name {
			case "output":
				var body struct{ Output string }
				if err := json.Unmarshal(event.Body, &body); err != nil {
					t.Fatal(err)
				}
				output += body.Output
			case "stopped":
				var body struct{ ThreadID int }
				if err := json.Unmarshal(event.Body, &body); err != nil {
					t.Fatal(err)
				}
				thread = body.ThreadID
				var stack struct{ StackFrames []struct{ ID, Line int } }
				if err := session.Call(ctx, "stackTrace", map[string]int{"threadId": thread}, &stack); err != nil {
					t.Fatal(err)
				}
				if len(stack.StackFrames) == 0 || stack.StackFrames[0].Line != 5 {
					t.Fatalf("stack: %+v", stack)
				}
				var scopes struct {
					Scopes []struct{ VariablesReference int }
				}
				if err := session.Call(ctx, "scopes", map[string]int{"frameId": stack.StackFrames[0].ID}, &scopes); err != nil {
					t.Fatal(err)
				}
				found := false
				for _, scope := range scopes.Scopes {
					var vars struct {
						Variables []struct{ Name, Value string }
					}
					if err := session.Call(ctx, "variables", map[string]int{"variablesReference": scope.VariablesReference}, &vars); err != nil {
						t.Fatal(err)
					}
					for _, v := range vars.Variables {
						if v.Name == "value" && v.Value == "42" {
							found = true
						}
					}
				}
				if !found {
					t.Fatal("missing known local")
				}
				if err := session.Call(ctx, "continue", map[string]int{"threadId": thread}, nil); err != nil {
					t.Fatal(err)
				}
			case "terminated":
				if thread == 0 || !strings.Contains(output, "42") {
					t.Fatalf("thread=%d output=%q", thread, output)
				}
				return
			}
		}
	}
}
