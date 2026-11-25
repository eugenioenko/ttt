package dap

import "context"

type Provider interface {
	Arguments(Target) (map[string]any, error)
	Open(context.Context, Target, map[string]any, func(Event), SessionOptions) (*Session, error)
}
