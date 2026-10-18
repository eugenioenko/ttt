package plugin

import (
	"fmt"
	neturl "net/url"

	lua "github.com/yuin/gopher-lua"
)

func setupNetModule(L *lua.LState, p *Plugin) {
	loader := func(L *lua.LState) int {
		mod := L.NewTable()

		if p.Granted.Check("network.http") == nil {
			L.SetField(mod, "get", L.NewFunction(netGet(p)))
			L.SetField(mod, "post", L.NewFunction(netPost(p)))
			L.SetField(mod, "get_async", L.NewFunction(netGetAsync(p)))
			L.SetField(mod, "post_async", L.NewFunction(netPostAsync(p)))
		}

		L.Push(mod)
		return 1
	}

	L.PreloadModule("ttt.net", loader)
}

// checkURL enforces the plugin's network.http host allowlist. Returns
// an error if the URL is malformed or its host is not permitted.
func (p *Plugin) checkURL(rawURL string) error {
	u, err := neturl.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	return p.Granted.CheckURL(u)
}

func extractHeaders(L *lua.LState, tbl *lua.LTable) map[string]string {
	headers := make(map[string]string)
	if tbl == nil {
		return headers
	}
	if h, ok := L.GetField(tbl, "headers").(*lua.LTable); ok {
		h.ForEach(func(k, v lua.LValue) {
			headers[k.String()] = v.String()
		})
	}
	return headers
}

func httpResultToLua(L *lua.LState, status int, body string, headers map[string]string, err error) *lua.LTable {
	tbl := L.NewTable()
	if err != nil {
		L.SetField(tbl, "status", lua.LNumber(0))
		L.SetField(tbl, "body", lua.LString(""))
		L.SetField(tbl, "error", lua.LString(err.Error()))
	} else {
		L.SetField(tbl, "status", lua.LNumber(status))
		L.SetField(tbl, "body", lua.LString(body))
		if headers != nil {
			ht := L.NewTable()
			for k, v := range headers {
				L.SetField(ht, k, lua.LString(v))
			}
			L.SetField(tbl, "headers", ht)
		}
	}
	return tbl
}

func netGet(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Network == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("network API not available"))
			return 2
		}
		url := L.CheckString(1)
		if err := p.checkURL(url); err != nil {
			L.Push(httpResultToLua(L, 0, "", nil, err))
			return 1
		}
		var headers map[string]string
		if opts, ok := L.Get(2).(*lua.LTable); ok {
			headers = extractHeaders(L, opts)
		}
		status, body, respHeaders, err := p.Network.Get(url, headers)
		L.Push(httpResultToLua(L, status, body, respHeaders, err))
		return 1
	}
}

func netPost(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Network == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("network API not available"))
			return 2
		}
		url := L.CheckString(1)
		if err := p.checkURL(url); err != nil {
			L.Push(httpResultToLua(L, 0, "", nil, err))
			return 1
		}
		opts, _ := L.Get(2).(*lua.LTable)
		var headers map[string]string
		var body string
		if opts != nil {
			headers = extractHeaders(L, opts)
			if b := L.GetField(opts, "body"); b != lua.LNil {
				body = b.String()
			}
		}
		status, respBody, respHeaders, err := p.Network.Post(url, headers, body)
		L.Push(httpResultToLua(L, status, respBody, respHeaders, err))
		return 1
	}
}

func netGetAsync(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Network == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("network API not available"))
			return 2
		}
		url := L.CheckString(1)
		var headers map[string]string
		callbackIdx := 2
		if opts, ok := L.Get(2).(*lua.LTable); ok {
			headers = extractHeaders(L, opts)
			callbackIdx = 3
		}
		callback := L.CheckFunction(callbackIdx)

		if err := p.checkURL(url); err != nil {
			p.SafePostAsync(&PluginAsyncResult{Plugin: p, Callback: func() {
				if p.State == nil {
					return
				}
				p.CallLuaFunc(callback, httpResultToLua(p.State, 0, "", nil, err))
			}})
			return 0
		}

		go func() {
			status, body, respHeaders, err := p.Network.Get(url, headers)
			resultFn := func() {
				if p.State == nil {
					return
				}
				tbl := httpResultToLua(p.State, status, body, respHeaders, err)
				if callErr := p.CallLuaFunc(callback, tbl); callErr != nil {
					p.logError("async net get callback", callErr)
				}
			}
			p.SafePostAsync(&PluginAsyncResult{Plugin: p, Callback: resultFn})
		}()

		return 0
	}
}

func netPostAsync(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Network == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("network API not available"))
			return 2
		}
		url := L.CheckString(1)
		opts, _ := L.Get(2).(*lua.LTable)
		var headers map[string]string
		var body string
		if opts != nil {
			headers = extractHeaders(L, opts)
			if b := L.GetField(opts, "body"); b != lua.LNil {
				body = b.String()
			}
		}
		callback := L.CheckFunction(3)

		if err := p.checkURL(url); err != nil {
			p.SafePostAsync(&PluginAsyncResult{Plugin: p, Callback: func() {
				if p.State == nil {
					return
				}
				p.CallLuaFunc(callback, httpResultToLua(p.State, 0, "", nil, err))
			}})
			return 0
		}

		go func() {
			status, respBody, respHeaders, err := p.Network.Post(url, headers, body)
			resultFn := func() {
				if p.State == nil {
					return
				}
				tbl := httpResultToLua(p.State, status, respBody, respHeaders, err)
				if callErr := p.CallLuaFunc(callback, tbl); callErr != nil {
					p.logError("async net post callback", callErr)
				}
			}
			p.SafePostAsync(&PluginAsyncResult{Plugin: p, Callback: resultFn})
		}()

		return 0
	}
}
