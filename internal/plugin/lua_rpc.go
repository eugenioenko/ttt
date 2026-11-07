package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"

	"github.com/eugenioenko/ttt/internal/jsonrpc"
	lua "github.com/yuin/gopher-lua"
)

const rpcConnTypeName = "ttt.rpc.conn"

const rpcMaxMessageSize = 32 << 20

const rpcOutboundQueueSize = 64
const rpcInboundQueueSize = 256

var errRPCClosed = errors.New("connection closed")
var errRPCInboxFull = errors.New("RPC incoming queue is full")

type rpcCallback struct {
	run  func()
	size int // Negative for lifecycle callbacks, which must survive closing.
}

type rpcConn struct {
	p        *Plugin
	endpoint string
	ctx      context.Context
	cancel   context.CancelFunc

	onRequest      *lua.LFunction
	onNotification *lua.LFunction
	onClose        *lua.LFunction

	mu         sync.Mutex
	transport  io.ReadWriteCloser
	conn       *jsonrpc.Conn
	inbox      []rpcCallback
	inboxCount int
	inboxBytes int
	draining   bool
	closedByUs bool
	closed     bool
	closeOnce  sync.Once
	outbox     *rpcOutbox
}

func setupRPCModule(L *lua.LState, p *Plugin) {
	loader := func(L *lua.LState) int {
		mod := L.NewTable()
		if p.Granted.Check("network.rpc") == nil {
			L.SetField(mod, "connect", L.NewFunction(rpcConnect(p)))
		}
		L.Push(mod)
		return 1
	}
	L.PreloadModule("ttt.rpc", loader)

	mt := L.NewTypeMetatable(rpcConnTypeName)
	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"request": rpcRequest,
		"notify":  rpcNotify,
		"close":   rpcClose,
	}))
}

func rpcConnect(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		endpoint := L.CheckString(1)
		opts := L.CheckTable(2)
		callback := L.CheckFunction(3)
		if err := p.Granted.CheckRPCEndpoint(endpoint); err != nil {
			L.ArgError(1, err.Error())
			return 0
		}
		p.mu.Lock()
		dialRPC := p.DialRPC
		p.mu.Unlock()
		if dialRPC == nil {
			L.RaiseError("rpc API not available")
			return 0
		}
		c := &rpcConn{p: p, endpoint: endpoint}
		c.onRequest, _ = L.GetField(opts, "on_request").(*lua.LFunction)
		c.onNotification, _ = L.GetField(opts, "on_notification").(*lua.LFunction)
		c.onClose, _ = L.GetField(opts, "on_close").(*lua.LFunction)
		c.ctx, c.cancel = context.WithCancel(context.Background())
		p.registerRPCConn(c)
		go c.dial(callback, dialRPC)
		return 0
	}
}

func (c *rpcConn) dial(callback *lua.LFunction, dialRPC func(context.Context, string) (io.ReadWriteCloser, error)) {
	p := c.p
	transport, err := dialRPC(c.ctx, c.endpoint)
	if err != nil {
		p.unregisterRPCConn(c)
		c.enqueue(func() {
			if p.State != nil {
				_ = p.CallLuaFunc(callback, lua.LNil, lua.LString(err.Error()))
			}
		})
		return
	}
	connected := func() {
		if p.State == nil {
			return
		}
		ud := p.State.NewUserData()
		ud.Value = c
		p.State.SetMetatable(ud, p.State.GetTypeMetatable(rpcConnTypeName))
		_ = p.CallLuaFunc(callback, ud, lua.LNil)
	}

	// Holding c.mu until the connected callback is queued keeps the read
	// loop from queueing a peer message ahead of it.
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		transport.Close()
		return
	}
	c.transport = transport
	outbox := newRPCOutbox(c.ctx, rpcWriter{transport: transport, timeout: rpcWriteTimeout, onError: c.handleClose})
	conn := jsonrpc.NewConn(transport, outbox, jsonrpc.Options{
		OnNotification:  c.handleNotification,
		DispatchRequest: c.handleRequest,
		OnReplyError:    c.handleClose,
		OnClose:         c.handleClose,
		MaxMessageSize:  rpcMaxMessageSize,
	})
	c.conn = conn
	c.outbox = outbox
	post := c.enqueueLocked(connected, -1)
	c.mu.Unlock()
	go outbox.run()
	if post {
		c.postDrain()
	}
}

// One queue preserves dispatch order when a full event queue could reorder
// separate posts. The read loop queues peer requests and notifications directly.
func (c *rpcConn) enqueue(fn func()) {
	c.mu.Lock()
	post := c.enqueueLocked(fn, -1)
	c.mu.Unlock()
	if post {
		c.postDrain()
	}
}

func (c *rpcConn) enqueueMessage(size int, fn func()) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	if c.inboxCount >= rpcInboundQueueSize || size > rpcMaxMessageSize-c.inboxBytes {
		c.mu.Unlock()
		c.handleClose(errRPCInboxFull)
		return
	}
	c.inboxCount++
	c.inboxBytes += size
	post := c.enqueueLocked(fn, size)
	c.mu.Unlock()
	if post {
		c.postDrain()
	}
}

func (c *rpcConn) enqueueLocked(fn func(), size int) bool {
	c.inbox = append(c.inbox, rpcCallback{run: fn, size: size})
	post := !c.draining
	c.draining = true
	return post
}

func (c *rpcConn) postDrain() {
	c.p.SafePostAsync(&PluginAsyncResult{Plugin: c.p, Callback: c.drain})
}

func (c *rpcConn) drain() {
	c.mu.Lock()
	inbox := c.inbox
	c.inbox = nil
	c.mu.Unlock()
	for i, callback := range inbox {
		c.mu.Lock()
		run := callback.size < 0 || !c.closed
		c.mu.Unlock()
		if run {
			callback.run()
		}
		inbox[i] = rpcCallback{}
		c.mu.Lock()
		if callback.size >= 0 && !c.closed {
			c.inboxCount--
			c.inboxBytes -= callback.size
		}
		c.mu.Unlock()
	}
	c.mu.Lock()
	post := len(c.inbox) > 0
	c.draining = post
	c.mu.Unlock()
	if post {
		c.postDrain()
	}
}

func (c *rpcConn) discardMessagesLocked() {
	kept := c.inbox[:0]
	for _, callback := range c.inbox {
		if callback.size < 0 {
			kept = append(kept, callback)
		}
	}
	clear(c.inbox[len(kept):])
	c.inbox = kept
	c.inboxCount, c.inboxBytes = 0, 0
}

func (c *rpcConn) handleNotification(method string, params json.RawMessage) {
	if c.onNotification == nil {
		return
	}
	c.enqueueMessage(len(method)+len(params), func() {
		if c.p.State != nil {
			_ = c.p.CallLuaFunc(c.onNotification, lua.LString(method), decodeRPCValue(c.p.State, params))
		}
	})
}

func (c *rpcConn) handleRequest(request jsonrpc.Message, reply func(any, error)) {
	method, params := request.Method, request.Params
	c.enqueueMessage(len(request.ID)+len(method)+len(params), func() {
		if c.onRequest == nil {
			reply(nil, &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "method not supported: " + method})
			return
		}
		L := c.p.State
		if L == nil {
			reply(nil, errRPCClosed)
			return
		}
		err := L.CallByParam(lua.P{Fn: c.onRequest, NRet: 1, Protect: true}, lua.LString(method), decodeRPCValue(L, params))
		if err != nil {
			reply(nil, errors.New(luaErrorMessage(err)))
			return
		}
		result := luaToGo(L.Get(-1))
		L.Pop(1)
		reply(result, nil)
	})
}

func (c *rpcConn) handleClose(err error) {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		byUs := c.closedByUs
		c.closed = true
		c.discardMessagesLocked()
		outbox := c.outbox
		transport := c.transport
		c.transport = nil
		c.mu.Unlock()
		c.cancel()
		if outbox != nil {
			outbox.stop()
		}
		if transport != nil {
			transport.Close()
		}
		c.p.unregisterRPCConn(c)
		if c.onClose == nil {
			return
		}
		c.enqueue(func() {
			if c.p.State == nil {
				return
			}
			reason := lua.LValue(lua.LNil)
			if !byUs && err != nil && !errors.Is(err, io.EOF) {
				reason = lua.LString(err.Error())
			}
			_ = c.p.CallLuaFunc(c.onClose, reason)
		})
	})
}

// close is safe from any goroutine and on a connection still dialing.
func (c *rpcConn) close() {
	c.mu.Lock()
	c.closedByUs = true
	c.closed = true
	c.discardMessagesLocked()
	outbox := c.outbox
	transport := c.transport
	c.transport = nil
	c.mu.Unlock()
	c.cancel()
	if outbox != nil {
		outbox.stop()
	}
	if transport != nil {
		transport.Close()
	}
}

func checkRPCConn(L *lua.LState) *rpcConn {
	ud := L.CheckUserData(1)
	c, ok := ud.Value.(*rpcConn)
	if !ok {
		L.ArgError(1, "rpc connection expected")
	}
	return c
}

func rpcRequest(L *lua.LState) int {
	c := checkRPCConn(L)
	method := L.CheckString(2)
	params := luaToGo(L.Get(3))
	callback := L.OptFunction(4, nil)
	go func() {
		result, err := c.conn.Call(c.ctx, method, params)
		if callback == nil {
			return
		}
		errText := ""
		if err != nil {
			errText = c.callError(err)
		}
		closed := c.ctx.Err() != nil || errors.Is(err, jsonrpc.ErrClosed)
		if closed {
			result, errText = nil, errRPCClosed.Error()
		}
		callbackRun := func() {
			if c.p.State == nil {
				return
			}
			if errText != "" {
				_ = c.p.CallLuaFunc(callback, lua.LNil, lua.LString(errText))
				return
			}
			_ = c.p.CallLuaFunc(callback, decodeRPCValue(c.p.State, result), lua.LNil)
		}
		if closed {
			c.enqueue(callbackRun)
		} else {
			c.enqueueMessage(len(result)+len(errText), callbackRun)
		}
	}()
	return 0
}

func (c *rpcConn) callError(err error) string {
	var rpcErr *jsonrpc.Error
	if errors.As(err, &rpcErr) {
		return rpcErr.Message
	}
	if c.ctx.Err() != nil || errors.Is(err, jsonrpc.ErrClosed) {
		return errRPCClosed.Error()
	}
	return err.Error()
}

func rpcNotify(L *lua.LState) int {
	c := checkRPCConn(L)
	method := L.CheckString(2)
	if err := c.conn.Notify(method, luaToGo(L.Get(3))); err != nil {
		L.Push(lua.LFalse)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LTrue)
	return 1
}

func rpcClose(L *lua.LState) int {
	checkRPCConn(L).close()
	return 0
}

func decodeRPCValue(L *lua.LState, raw json.RawMessage) lua.LValue {
	if len(raw) == 0 {
		return lua.LNil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return lua.LNil
	}
	return goToLua(L, v)
}

func luaErrorMessage(err error) string {
	var apiErr *lua.ApiError
	if errors.As(err, &apiErr) && apiErr.Object != nil {
		return apiErr.Object.String()
	}
	return err.Error()
}

func (p *Plugin) registerRPCConn(c *rpcConn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rpcConns == nil {
		p.rpcConns = make(map[*rpcConn]struct{})
	}
	p.rpcConns[c] = struct{}{}
}

func (p *Plugin) unregisterRPCConn(c *rpcConn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.rpcConns, c)
}

// closeRPCConnsLocked closes every connection; caller must hold p.mu.
func (p *Plugin) closeRPCConnsLocked() {
	for c := range p.rpcConns {
		c.close()
	}
	p.rpcConns = nil
}
