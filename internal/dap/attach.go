package dap

import (
	"encoding/json"
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
	port, err := ProcessID(endpoint["port"])
	if err != nil || port > 65535 {
		return "", fmt.Errorf("%s: expected a port from 1 to 65535", t.Name)
	}
	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

func ProcessID(value any) (int, error) {
	var n int
	switch v := value.(type) {
	case json.Number:
		n, _ = strconv.Atoi(v.String())
	case string:
		n, _ = strconv.Atoi(v)
	case float64:
		n = int(v)
		if float64(n) != v {
			n = 0
		}
	case int:
		n = v
	}
	if n < 1 || int64(n) > 2147483647 {
		return 0, fmt.Errorf("invalid processId %v", value)
	}
	return n, nil
}
