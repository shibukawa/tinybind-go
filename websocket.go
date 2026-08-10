package httpbind

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/internal/bindcore"
	"github.com/shibukawa/tinygodriver/websocket"
)

// Socket is a typed WebSocket connection: Read decodes into In, Write encodes
// from Out, both through the jsonbind codec registry.
//
// It is the same type the fasthttp runtime uses, so a callback body compiles
// unchanged on either transport. In and Out must each have a generated codec
// — a decoder for In, an encoder for Out — which discovery emits from the two
// type arguments of the WebSocket call.
//
// Read must be called from one goroutine. Write may be called from any.
type Socket[In, Out any] = bindcore.Socket[In, Out]

// SocketOptions configures one socket. A zero field takes the process default
// installed with SetSocketDefaults, and nothing reaches the driver as zero.
type SocketOptions = bindcore.SocketOptions

// SetSocketDefaults installs the process-wide socket options. It is shared
// with the fasthttp runtime, so installing them once covers both.
func SetSocketDefaults(opts SocketOptions) { bindcore.SetSocketDefaults(opts) }

// SocketDefaults returns the effective process defaults.
func SocketDefaults() SocketOptions { return bindcore.SocketDefaults() }

// WebSocket upgrades the request, runs fn against a typed socket, and closes
// the socket when fn returns.
//
// Handler usage:
//
//	_ = httpbind.WebSocket(w, r, func(s *httpbind.Socket[ClientMsg, ServerMsg]) error {
//		for {
//			in, err := s.Read()
//			if err != nil {
//				return err
//			}
//			if err := s.Write(ServerMsg{Type: "echo", Text: in.Text}); err != nil {
//				return err
//			}
//		}
//	})
//
// The return value is the handshake error and nothing else. A non-nil value
// means the refusal response has already been written, as RFC 9457 Problem
// Details; the handler logs or counts it rather than answering. fn's own error
// is raised after the 101 has gone out, on this transport and on fasthttp, so
// it reaches the handler installed with SetStreamErrorHandler instead.
//
// fn runs before this returns here and after the handler returns on fasthttp.
// Nothing in the callback may read the request, so that one source works on
// both: capture what it needs — the identity, the peer — before calling.
func WebSocket[In, Out any](w http.ResponseWriter, r *http.Request, fn func(*Socket[In, Out]) error) error {
	return WebSocketWith[In, Out](w, r, SocketOptions{}, fn)
}

// WebSocketWith is WebSocket with per-call options, for the endpoint whose
// limits or cadence differ from the process defaults.
func WebSocketWith[In, Out any](w http.ResponseWriter, r *http.Request, opts SocketOptions, fn func(*Socket[In, Out]) error) error {
	if w == nil || r == nil {
		return bindcore.SocketHandshakeError(400, bindcore.ErrNilSocket)
	}
	opts = bindcore.ResolveSocketOptions(opts)
	if err := bindcore.ValidateSocketOptions(opts); err != nil {
		WriteError(w, r, err)
		return err
	}
	// TinyGo's own net/http server cannot complete an upgrade: it starts a
	// background read before the handler and cancels it by moving the read
	// deadline into the past, which netdev cannot do to a recv already in
	// flight. Hijack then blocks forever, with no error and no log line.
	// Asserting here turns that silence into an answer.
	if _, ok := w.(http.Hijacker); !ok {
		err := bindcore.SocketHandshakeError(500, bindcore.ErrNoHijacker)
		WriteError(w, r, err)
		return err
	}

	var refusal error
	upgrader := websocket.Upgrader{
		ReadBufferSize:    opts.ReadBufferSize,
		WriteBufferSize:   opts.WriteBufferSize,
		Subprotocols:      opts.Subprotocols,
		EnableCompression: opts.EnableCompression,
		CheckOrigin: func(r *http.Request) bool {
			return opts.CheckOrigin(r.Header.Get("Origin"), r.Host)
		},
		Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
			refusal = bindcore.SocketHandshakeError(status, reason)
			WriteError(w, r, refusal)
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if refusal != nil {
			return refusal
		}
		return bindcore.SocketHandshakeError(400, err)
	}
	// The connection is hijacked and outlives the handler unless it is closed
	// here. Closing it after ServeSocket is what keeps the two transports the
	// same: fasthttp closes its own when the upgrade callback returns.
	defer conn.Close()

	if err := bindcore.ServeSocket[In, Out](conn, opts, fn); err != nil {
		bindcore.ReportStreamError(err)
	}
	return nil
}
