package fasthttpbind

import (
	"github.com/shibukawa/tinybind-go/internal/bindcore"
	"github.com/shibukawa/tinygodriver/fasthttp"
	websocket "github.com/shibukawa/tinygodriver/fasthttpwebsocket"
)

// Socket is a typed WebSocket connection. It is the same type the net/http
// runtime uses, so a callback body compiles unchanged on either transport.
type Socket[In, Out any] = bindcore.Socket[In, Out]

// SocketOptions configures one socket. A zero field takes the process default
// installed with SetSocketDefaults.
type SocketOptions = bindcore.SocketOptions

// SetSocketDefaults installs the process-wide socket options. It is shared
// with the net/http runtime, so installing them once covers both.
func SetSocketDefaults(opts SocketOptions) { bindcore.SetSocketDefaults(opts) }

// SocketDefaults returns the effective process defaults.
func SocketDefaults() SocketOptions { return bindcore.SocketDefaults() }

// WebSocket upgrades the request, runs fn against a typed socket, and closes
// the socket when fn returns.
//
// The return value is the handshake error and nothing else. A non-nil value
// means the refusal response has already been written, as RFC 9457 Problem
// Details. fn's own error is raised after the 101 has gone out, so it reaches
// the handler installed with SetStreamErrorHandler instead.
//
// fn runs after the handler has returned, from the hijacked connection, so it
// must not read ctx: everything it needs is captured before WebSocket returns.
// fasthttp closes the connection when fn returns, which is what the callback
// shape wants and why KeepHijackedConns stays off.
func WebSocket[In, Out any](ctx *fasthttp.RequestCtx, fn func(*Socket[In, Out]) error) error {
	return WebSocketWith[In, Out](ctx, SocketOptions{}, fn)
}

// WebSocketWith is WebSocket with per-call options.
func WebSocketWith[In, Out any](ctx *fasthttp.RequestCtx, opts SocketOptions, fn func(*Socket[In, Out]) error) error {
	if ctx == nil {
		return bindcore.SocketHandshakeError(400, bindcore.ErrNilSocket)
	}
	opts = bindcore.ResolveSocketOptions(opts)
	if err := bindcore.ValidateSocketOptions(opts); err != nil {
		WriteError(ctx, err)
		return err
	}

	var refusal error
	upgrader := websocket.FastHTTPUpgrader{
		ReadBufferSize:    opts.ReadBufferSize,
		WriteBufferSize:   opts.WriteBufferSize,
		Subprotocols:      opts.Subprotocols,
		EnableCompression: opts.EnableCompression,
		CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
			return opts.CheckOrigin(
				string(ctx.Request.Header.Peek("Origin")),
				string(ctx.Host()),
			)
		},
		Error: func(ctx *fasthttp.RequestCtx, status int, reason error) {
			refusal = bindcore.SocketHandshakeError(status, reason)
			WriteError(ctx, refusal)
		},
	}

	// Upgrade returns as soon as the 101 is queued; the callback runs later,
	// on the hijacked connection, and fasthttp closes it when that returns.
	err := upgrader.Upgrade(ctx, func(conn *websocket.Conn) {
		if err := bindcore.ServeSocket[In, Out](conn, opts, fn); err != nil {
			bindcore.ReportStreamError(err)
		}
	})
	if err != nil {
		if refusal != nil {
			return refusal
		}
		return bindcore.SocketHandshakeError(400, err)
	}
	return nil
}
