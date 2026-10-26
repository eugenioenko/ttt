package plugin

import (
	"math"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	lua "github.com/yuin/gopher-lua"
)

const streamScript = `
	local sys = require("ttt.system")
	chunks, errs, order = {}, {}, {}
	handle = sys.exec_async("psql", {}, OPTS, function(r)
		order[#order + 1] = "done"
		result = r
	end)
`

func startStream(t *testing.T, mock *mockSystemAPI, opts string) (*Plugin, chan func(), func()) {
	t.Helper()
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"psql"}}, mock)
	ch := asyncResults(p)
	if err := p.State.DoString(strings.ReplaceAll(streamScript, "OPTS", opts)); err != nil {
		cleanup()
		t.Fatal(err)
	}
	return p, ch, cleanup
}

// runUntilDone runs posted callbacks on this goroutine, as the main loop would.
func runUntilDone(t *testing.T, p *Plugin, ch chan func()) *lua.LTable {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for p.State.GetGlobal("result") == lua.LNil {
		select {
		case fn := <-ch:
			fn()
		case <-deadline:
			t.Fatal("exec did not finish")
		}
	}
	return p.State.GetGlobal("result").(*lua.LTable)
}

func luaStrings(p *Plugin, name string) []string {
	var out []string
	if tbl, ok := p.State.GetGlobal(name).(*lua.LTable); ok {
		tbl.ForEach(func(_, v lua.LValue) { out = append(out, v.String()) })
	}
	return out
}

const collectOpts = `{
	on_stdout = function(c) chunks[#chunks + 1] = c; order[#order + 1] = "out" end,
	on_stderr = function(c) errs[#errs + 1] = c end,
}`

func TestExecAsyncStreamsWholeLinesInOrder(t *testing.T) {
	mock := &mockSystemAPI{writes: []string{"a\nb", "c\n", "partial"}, stderr: "warn\n"}
	p, ch, cleanup := startStream(t, mock, collectOpts)
	defer cleanup()
	r := runUntilDone(t, p, ch)

	if got := strings.Join(luaStrings(p, "chunks"), "|"); got != "a\n|bc\n|partial" {
		t.Errorf("chunks = %q", got)
	}
	if got := strings.Join(luaStrings(p, "errs"), "|"); got != "warn\n" {
		t.Errorf("stderr chunks = %q", got)
	}
	if got := strings.Join(luaStrings(p, "order"), ","); got != "out,out,out,done" {
		t.Errorf("callback order = %s, want every chunk before the result", got)
	}
	if r.RawGetString("stdout").String() != "" || r.RawGetString("stderr").String() != "" {
		t.Errorf("streamed output is also in the result: %q %q", r.RawGetString("stdout"), r.RawGetString("stderr"))
	}
	if r.RawGetString("exit_code").String() != "0" {
		t.Errorf("exit_code = %s", r.RawGetString("exit_code"))
	}
}

func TestExecAsyncStreamSplitsLongLinesAtRuneBoundaries(t *testing.T) {
	line := strings.Repeat("日", 30000)
	mock := &mockSystemAPI{writes: []string{line[:40000], line[40000:]}}
	p, ch, cleanup := startStream(t, mock, collectOpts)
	defer cleanup()
	runUntilDone(t, p, ch)

	chunks := luaStrings(p, "chunks")
	if len(chunks) < 2 {
		t.Fatalf("got %d chunks, want the line split", len(chunks))
	}
	for i, c := range chunks {
		if len(c) > execChunkMax || !utf8.ValidString(c) {
			t.Errorf("chunk %d: %d bytes, valid UTF-8 %v", i, len(c), utf8.ValidString(c))
		}
	}
	if strings.Join(chunks, "") != line {
		t.Error("chunks do not add up to the output")
	}
}

func TestExecMaxOutputBytesKillsAndTruncates(t *testing.T) {
	mock := &mockSystemAPI{writes: []string{"12345", "67890"}, block: true}
	p, ch, cleanup := startStream(t, mock, `{ max_output_bytes = 7 }`)
	defer cleanup()
	r := runUntilDone(t, p, ch)

	if r.RawGetString("stdout").String() != "1234567" {
		t.Errorf("stdout = %q, want the first 7 bytes", r.RawGetString("stdout"))
	}
	if r.RawGetString("truncated") != lua.LTrue || r.RawGetString("exit_code").String() != "-1" {
		t.Errorf("truncated = %v, exit_code = %v", r.RawGetString("truncated"), r.RawGetString("exit_code"))
	}
	if r.RawGetString("cancelled") != lua.LNil {
		t.Error("a truncated exec reported cancelled")
	}
}

func TestExecMaxOutputBytesWithStreaming(t *testing.T) {
	mock := &mockSystemAPI{writes: []string{"row1\nrow2\n", "row3\nrow4\n"}, block: true}
	p, ch, cleanup := startStream(t, mock, `{ max_output_bytes = 12, on_stdout = function(c) chunks[#chunks + 1] = c end }`)
	defer cleanup()
	r := runUntilDone(t, p, ch)

	if got := strings.Join(luaStrings(p, "chunks"), "|"); got != "row1\nrow2\n|ro" {
		t.Errorf("chunks = %q", got)
	}
	if r.RawGetString("truncated") != lua.LTrue {
		t.Error("truncated not set")
	}
}

func TestExecSyncMaxOutputBytes(t *testing.T) {
	mock := &mockSystemAPI{stdout: "abcdef", exitCode: 0}
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"psql"}}, mock)
	defer cleanup()
	if err := p.State.DoString(`r = require("ttt.system").exec("psql", {}, { max_output_bytes = 6 })`); err != nil {
		t.Fatal(err)
	}
	r := p.State.GetGlobal("r").(*lua.LTable)
	if r.RawGetString("stdout").String() != "abcdef" || r.RawGetString("truncated") != lua.LNil {
		t.Errorf("output at exactly the limit: stdout %q, truncated %v", r.RawGetString("stdout"), r.RawGetString("truncated"))
	}
}

func TestExecStreamChunkQueuedBehindCancelIsDropped(t *testing.T) {
	mock := &mockSystemAPI{writes: []string{"first\n"}, block: true}
	p, ch, cleanup := startStream(t, mock, collectOpts)
	defer cleanup()

	chunk := <-ch
	if err := p.State.DoString(`handle:cancel()`); err != nil {
		t.Fatal(err)
	}
	final := <-ch
	final()
	chunk()
	if got := luaStrings(p, "chunks"); len(got) != 0 {
		t.Errorf("chunk ran after the final callback: %q", got)
	}
	if r := p.State.GetGlobal("result").(*lua.LTable); r.RawGetString("cancelled") != lua.LTrue {
		t.Error("result not cancelled")
	}
}

func TestExecStreamChunkQueuedBeforeCancelIsDropped(t *testing.T) {
	mock := &mockSystemAPI{writes: []string{"first\n"}, block: true}
	p, ch, cleanup := startStream(t, mock, collectOpts)
	defer cleanup()

	chunk := <-ch
	if err := p.State.DoString(`handle:cancel()`); err != nil {
		t.Fatal(err)
	}
	final := <-ch
	chunk()
	if got := luaStrings(p, "chunks"); len(got) != 0 {
		t.Errorf("chunk ran after cancel(): %q", got)
	}
	final()
	if r := p.State.GetGlobal("result").(*lua.LTable); r.RawGetString("cancelled") != lua.LTrue {
		t.Error("result not cancelled")
	}
}

func TestExecOutputChunksStayWithinChunkMax(t *testing.T) {
	long := strings.Repeat("a", 70000)
	exact := strings.Repeat("b", execChunkMax)
	for _, tc := range []struct {
		name   string
		writes []string
		want   int
	}{
		{"newline arrives after the line outgrew the max", []string{long[:60000], long[60000:] + "\nshort\n"}, 2},
		{"long line in one write", []string{long + "\n"}, 2},
		{"line of exactly the max", []string{exact + "\n"}, 1},
		{"short lines after a partial one", []string{long[:60000], "x\n" + strings.Repeat("row\n", 4000)}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var chunks []string
			o := &execOutput{deliver: func(c string) { chunks = append(chunks, c) }}
			for _, w := range tc.writes {
				o.Write([]byte(w))
			}
			o.flush()
			if len(chunks) != tc.want {
				t.Errorf("got %d chunks, want %d", len(chunks), tc.want)
			}
			for i, c := range chunks {
				if len(strings.TrimSuffix(c, "\n")) > execChunkMax {
					t.Errorf("chunk %d: %d bytes", i, len(c))
				}
			}
			if strings.Join(chunks, "") != strings.Join(tc.writes, "") {
				t.Error("chunks do not add up to the output")
			}
		})
	}
}

func TestExecStreamOptionsRejected(t *testing.T) {
	for _, code := range []string{
		`sys.exec("psql", {}, { on_stdout = function() end })`,
		`sys.exec_async("psql", {}, { on_stdout = "print" }, function() end)`,
		`sys.exec_async("psql", {}, { on_stderr = 1 }, function() end)`,
		`sys.exec("psql", {}, { max_output_bytes = 0 })`,
		`sys.exec("psql", {}, { max_output_bytes = 0/0 })`,
		`sys.exec("psql", {}, { max_output_bytes = "1MB" })`,
	} {
		mock := &mockSystemAPI{}
		p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"psql"}}, mock)
		if err := p.State.DoString(`local sys = require("ttt.system"); ` + code); err == nil {
			t.Errorf("%s: expected an error", code)
		}
		if mock.execBinary != "" {
			t.Errorf("%s: ran the command", code)
		}
		cleanup()
	}
}

func TestExecMaxOutputBytesBeyondInt64StaysALimit(t *testing.T) {
	for _, v := range []string{"1e20", "math.huge"} {
		L := lua.NewState()
		if err := L.DoString("opts = { max_output_bytes = " + v + " }"); err != nil {
			t.Fatal(err)
		}
		L.Push(L.NewTable())
		L.Push(L.GetGlobal("opts"))
		if got := parseExecArgs(L, "psql", 1).maxOutput; got != math.MaxInt64 {
			t.Errorf("max_output_bytes = %s: limit %d", v, got)
		}
		L.Close()
	}
}
