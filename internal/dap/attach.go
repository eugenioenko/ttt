package dap

import (
	"fmt"
	"net"
	"strconv"
)

func Endpoint(t Target, endpoint map[string]any) (string, error) {
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
