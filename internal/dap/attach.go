package dap

import (
	"context"
	"fmt"
	"net"
	"strconv"
)

func attachEndpoint(t Target) (string, error) {
	endpoint := t.Arguments
	if t.Type == "debugpy" {
		var ok bool
		endpoint, ok = t.Arguments["connect"].(map[string]any)
		if !ok {
			return "", fmt.Errorf("%s: expected a connect object", t.Name)
		}
		for k := range endpoint {
			if k != "host" && k != "port" {
				return "", fmt.Errorf("%s: unsupported connect field %q", t.Name, k)
			}
		}
	}
	host := "127.0.0.1"
	if val, ok := endpoint["host"]; ok {
		var valid bool
		host, valid = val.(string)
		if !valid || host == "" {
			return "", fmt.Errorf("%s: invalid host", t.Name)
		}
	}
	port, ok := endpoint["port"].(float64)
	if !ok || port < 1 || port > 65535 || float64(int(port)) != port {
		return "", fmt.Errorf("%s: expected a port from 1 to 65535", t.Name)
	}
	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

func (t Target) addressAttachArguments() (map[string]any, error) {
	if _, err := attachEndpoint(t); err != nil {
		return nil, err
	}
	out := make(map[string]any)
	for key, val := range t.Arguments {
		switch key {
		case "name", "type", "request", "presentation":
			continue
		case "host", "port":
			if t.Type == "go" {
				continue
			}
		case "mode":
			if t.Type == "go" && val == "remote" {
				continue
			}
		case "connect":
			if t.Type == "debugpy" {
				out[key] = val
				continue
			}
		case "justMyCode", "pathMappings":
			if t.Type == "debugpy" {
				x, err := expand(val, t.Folder)
				if err != nil {
					return nil, err
				}
				out[key] = x
				continue
			}
		case "substitutePath":
			if t.Type == "go" {
				x, err := expand(val, t.Folder)
				if err != nil {
					return nil, err
				}
				out[key] = x
				continue
			}
		}
		return nil, fmt.Errorf("%s: unsupported address attach field %q", t.Name, key)
	}
	if t.Type == "go" {
		out["mode"] = "remote"
	}
	return out, nil
}

func connectAdapter(ctx context.Context, t Target, onEvent func(Event)) (*Session, error) {
	addr, err := attachEndpoint(t)
	if err != nil {
		return nil, err
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	s := newSession(conn, conn, conn, func() {}, onEvent)
	s.remoteDelve = t.Type == "go"
	return s, nil
}
