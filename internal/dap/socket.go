package dap

import (
	"context"
	"encoding/json"
	"net"
)

type OpenChild func(context.Context, func(Event), SessionOptions) (*Session, error)
type SessionOptions struct {
	RunInTerminal      RunInTerminal
	StartDebugging     func(context.Context, string, map[string]any, OpenChild) error
	openChild          OpenChild
	ResumeBeforeDetach bool
}

func (s *Session) SupportsStartDebugging() bool {
	return s.options.StartDebugging != nil && s.options.openChild != nil
}
func (s *Session) ResumeBeforeDetach() bool { return s.options.ResumeBeforeDetach }
func (s *Session) OnCleanup(clean func()) {
	s.cleanupMu.Lock()
	if s.cleaned {
		s.cleanupMu.Unlock()
		clean()
		return
	}
	s.cleanups = append(s.cleanups, clean)
	s.cleanupMu.Unlock()
}

func SocketSession(conn net.Conn, cleanup func(), event func(Event), options SessionOptions) *Session {
	addr := conn.RemoteAddr()
	if addr.Network() == "tcp" || (addr.Network() == "unix" && addr.String() != "" && addr.String() != "@") {
		allowTerminal := options.RunInTerminal != nil
		options.openChild = func(ctx context.Context, event func(Event), options SessionOptions) (*Session, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, addr.Network(), addr.String())
			if err != nil {
				return nil, err
			}
			if !allowTerminal {
				options.RunInTerminal = nil
			}
			return SocketSession(conn, func() {}, event, options), nil
		}
	}
	return newSession(conn, conn, func() { _ = conn.Close(); cleanup() }, event, options)
}
func Connect(ctx context.Context, address string, event func(Event), options SessionOptions) (*Session, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	options.RunInTerminal = nil
	return SocketSession(conn, func() {}, event, options), nil
}

type adapterStderr func(Event)

func (output adapterStderr) Write(data []byte) (int, error) {
	if output != nil {
		body, err := json.Marshal(map[string]string{"category": "stderr", "output": string(data)})
		if err != nil {
			return 0, err
		}
		output(Event{Name: "output", Body: body})
	}
	return len(data), nil
}
