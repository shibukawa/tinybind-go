package bindcore

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

// The socket lives here whole, for the reason the stream does: the read
// discipline, the deadline placement, the close handshake and the write
// serialization are behaviour two implementations would be two chances to
// disagree about.
//
// Unlike the stream, a socket needs the connection rather than a writer, and
// the two driver Conn types are different types in different packages. They
// are the same upstream API, so this package names the subset it uses and
// imports neither: a fasthttp type must not reach the graph of a net/http
// build.

// MessageConn is the part of a driver WebSocket connection a typed socket
// uses. Both driver Conn types satisfy it structurally, with no adapter.
type MessageConn interface {
	ReadMessage() (messageType int, p []byte, err error)
	NextWriter(messageType int) (io.WriteCloser, error)
	WriteControl(messageType int, data []byte, deadline time.Time) error
	SetReadLimit(limit int64)
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	SetPongHandler(h func(appData string) error)
	Subprotocol() string
	Close() error
}

// Frame opcodes and close codes are RFC 6455's own numbers, not a library's,
// which is why restating them here cannot drift from the driver.
const (
	textMessage   = 1
	binaryMessage = 2
	closeMessage  = 8
	pingMessage   = 9

	closeNormalClosure = 1000
)

var (
	ErrNilSocket    = errors.New("httpbind: nil socket")
	ErrSocketClosed = errors.New("httpbind: socket closed")
	// ErrBinaryMessage reports a binary frame on a socket carrying JSON.
	ErrBinaryMessage = errors.New("httpbind: binary message on a JSON socket")
)

// Socket option defaults. Every one is non-zero: an unset limit is an
// allocation the peer chooses, and an unset read deadline is a read nothing
// can interrupt under TinyGo.
const (
	DefaultSocketReadLimit    int64 = 1 << 20
	DefaultSocketIdleTimeout        = 60 * time.Second
	DefaultSocketPingInterval       = 54 * time.Second
	DefaultSocketWriteTimeout       = 10 * time.Second
	DefaultSocketBufferSize         = 4096
)

// SocketOptions configures one socket. A zero field takes the process default
// installed with SetSocketDefaults, and a process default left zero takes the
// constant above, so nothing reaches the driver as zero.
type SocketOptions struct {
	ReadLimit       int64
	IdleTimeout     time.Duration
	PingInterval    time.Duration
	WriteTimeout    time.Duration
	ReadBufferSize  int
	WriteBufferSize int

	// Subprotocols are offered in preference order; the negotiated one is
	// readable from the socket.
	Subprotocols []string

	// EnableCompression asks for permessage-deflate. It is off by default:
	// it pulls flate into the binary, and only no-context-takeover mode is
	// supported, so the saving is smaller than the cost.
	EnableCompression bool

	// CheckOrigin decides whether a handshake is allowed, from the Origin and
	// Host header values. It takes two strings rather than a request so one
	// policy serves both transports, the same reason stream negotiation does.
	// A nil value takes CheckOriginDefault.
	CheckOrigin func(origin, host string) bool
}

var socketDefaults atomic.Pointer[SocketOptions]

// SetSocketDefaults installs the process-wide socket options. It is shared
// with the fasthttp runtime, so installing them once covers both.
func SetSocketDefaults(opts SocketOptions) {
	copied := opts
	socketDefaults.Store(&copied)
}

// SocketDefaults returns the installed process defaults, with every unset
// field resolved to its constant.
func SocketDefaults() SocketOptions {
	var opts SocketOptions
	if stored := socketDefaults.Load(); stored != nil {
		opts = *stored
	}
	return resolveSocketOptions(opts)
}

// ResolveSocketOptions fills opts from the process defaults, then from the
// constants. Both surfaces call it so one options value means the same thing
// on either.
func ResolveSocketOptions(opts SocketOptions) SocketOptions {
	if stored := socketDefaults.Load(); stored != nil {
		fallback := *stored
		if opts.ReadLimit == 0 {
			opts.ReadLimit = fallback.ReadLimit
		}
		if opts.IdleTimeout == 0 {
			opts.IdleTimeout = fallback.IdleTimeout
		}
		if opts.PingInterval == 0 {
			opts.PingInterval = fallback.PingInterval
		}
		if opts.WriteTimeout == 0 {
			opts.WriteTimeout = fallback.WriteTimeout
		}
		if opts.ReadBufferSize == 0 {
			opts.ReadBufferSize = fallback.ReadBufferSize
		}
		if opts.WriteBufferSize == 0 {
			opts.WriteBufferSize = fallback.WriteBufferSize
		}
		if opts.Subprotocols == nil {
			opts.Subprotocols = fallback.Subprotocols
		}
		if !opts.EnableCompression {
			opts.EnableCompression = fallback.EnableCompression
		}
		if opts.CheckOrigin == nil {
			opts.CheckOrigin = fallback.CheckOrigin
		}
	}
	return resolveSocketOptions(opts)
}

func resolveSocketOptions(opts SocketOptions) SocketOptions {
	if opts.ReadLimit == 0 {
		opts.ReadLimit = DefaultSocketReadLimit
	}
	if opts.IdleTimeout == 0 {
		opts.IdleTimeout = DefaultSocketIdleTimeout
	}
	if opts.PingInterval == 0 {
		opts.PingInterval = DefaultSocketPingInterval
	}
	if opts.WriteTimeout == 0 {
		opts.WriteTimeout = DefaultSocketWriteTimeout
	}
	if opts.ReadBufferSize == 0 {
		opts.ReadBufferSize = DefaultSocketBufferSize
	}
	if opts.WriteBufferSize == 0 {
		opts.WriteBufferSize = DefaultSocketBufferSize
	}
	if opts.CheckOrigin == nil {
		opts.CheckOrigin = CheckOriginDefault
	}
	return opts
}

// ValidateSocketOptions rejects a combination that would serve a socket which
// dies on schedule.
func ValidateSocketOptions(opts SocketOptions) error {
	// A ping at or above the idle bound only ever fires after the read that
	// was supposed to be kept alive has already timed out.
	if opts.PingInterval > 0 && opts.PingInterval >= opts.IdleTimeout {
		return BadRequest(Problem{
			Code:    "websocket_options",
			Message: "ping interval must be shorter than the idle timeout",
		})
	}
	return nil
}

// CheckOriginDefault refuses a handshake whose Origin names a host other than
// the one the request was addressed to, and admits one carrying no Origin.
//
// A socket accepting any origin is cross-site request forgery with a
// persistent connection, so this is the default rather than the opt-in.
func CheckOriginDefault(origin, host string) bool {
	if origin == "" {
		return true
	}
	return equalFoldASCII(originHost(origin), host)
}

// originHost extracts the host[:port] of an origin without net/url, which
// would be a parser and an allocation for a value this shape.
func originHost(origin string) string {
	rest := origin
	if i := indexByte(rest, ':'); i >= 0 && hasPrefixAt(rest, i, "://") {
		rest = rest[i+3:]
	}
	if i := indexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func hasPrefixAt(s string, at int, prefix string) bool {
	return len(s) >= at+len(prefix) && s[at:at+len(prefix)] == prefix
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

// SocketHandshakeError shapes an upgrader's refusal as a Problem-carrying
// error, so a refused socket looks like every other refusal in the application
// rather than like the driver's own plain text. Both surfaces install it on
// their upgrader's Error hook, which is the only place a refusal can be
// reshaped: the driver writes the response itself, before any 101.
//
// The 500 case keeps its named code for the returned error and the log only;
// ProblemResponse hides a 5xx code from the body, which is what should happen
// to a server misconfiguration.
func SocketHandshakeError(status int, reason error) error {
	switch {
	case status == 403:
		return statusError(403, "Forbidden", Problem{
			Code:    "websocket_origin",
			Message: "origin not allowed",
		}, reason)
	case status >= 500:
		return statusError(500, "Internal Server Error", Problem{
			Code:    "websocket_hijack",
			Message: "the server cannot hand over the connection",
		}, reason)
	default:
		if status == 0 {
			status = 400
		}
		return statusError(status, StatusText(status), Problem{
			Code:    "websocket_upgrade",
			Message: "not a WebSocket upgrade request",
		}, reason)
	}
}

// ErrNoHijacker reports a ResponseWriter that cannot hand over the connection,
// which under TinyGo means the server is not the one an upgrade needs.
var ErrNoHijacker = errors.New("httpbind: ResponseWriter is not an http.Hijacker; serve through tinygodriver/httpserver")

// Socket is a typed WebSocket connection carrying one JSON value per message.
//
// Read must be called from one goroutine. Write may be called from any: it
// takes a lock the control frames share, so a broadcast goroutine cannot
// interleave its frame with a message or with a lifecycle ping.
type Socket[In, Out any] struct {
	conn   MessageConn
	opts   SocketOptions
	mu     sync.Mutex
	closed atomic.Bool
}

// NewSocket builds a typed socket over conn. ServeSocket is the entry that
// applies the lifecycle; this exists for tests and for a caller assembling one
// by hand.
func NewSocket[In, Out any](conn MessageConn, opts SocketOptions) *Socket[In, Out] {
	return &Socket[In, Out]{conn: conn, opts: opts}
}

// Subprotocol returns the subprotocol the handshake negotiated, or "".
func (s *Socket[In, Out]) Subprotocol() string {
	if s == nil || s.conn == nil {
		return ""
	}
	return s.conn.Subprotocol()
}

// Read returns the next message decoded into In.
//
// The read deadline is set here rather than once at setup: netdev takes a
// deadline by value when a read begins, so the bound has to be in place before
// the call rather than pushed into it afterwards.
//
// A decode failure is returned without closing the socket, because a message
// this application cannot read is the application's to answer.
func (s *Socket[In, Out]) Read() (In, error) {
	var zero In
	if s == nil || s.conn == nil {
		return zero, Internal(ErrNilSocket)
	}
	if s.closed.Load() {
		return zero, Internal(ErrSocketClosed)
	}
	if err := s.conn.SetReadDeadline(time.Now().Add(s.opts.IdleTimeout)); err != nil {
		return zero, err
	}
	messageType, data, err := s.conn.ReadMessage()
	if err != nil {
		return zero, err
	}
	if messageType != textMessage {
		return zero, BadRequest(Problem{
			Code:    "websocket_frame",
			Message: "expected a text frame carrying JSON",
		}, ErrBinaryMessage)
	}
	return jsonbind.DecodeJSONBytes[In](data)
}

// Write encodes v as one text frame. Safe to call from any goroutine.
func (s *Socket[In, Out]) Write(v Out) error {
	if s == nil || s.conn == nil {
		return Internal(ErrNilSocket)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return Internal(ErrSocketClosed)
	}
	if err := s.conn.SetWriteDeadline(time.Now().Add(s.opts.WriteTimeout)); err != nil {
		return err
	}
	w, err := s.conn.NextWriter(textMessage)
	if err != nil {
		return err
	}
	// Encoding straight into the frame writer keeps the message out of an
	// intermediate buffer; the frame is only complete once Close runs.
	if err := jsonbind.EncodeJSON(w, v); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}

// Close ends the socket with a normal-closure handshake. Idempotent.
//
// It does not close the underlying connection: the entry that opened it does
// that, so the close frame is always sent before the transport goes away.
//
// A goroutine calling this to end someone else's read loop does not interrupt
// a read already blocked — nothing can, under TinyGo. The reader returns when
// the peer answers the close frame, or when its own deadline expires.
func (s *Socket[In, Out]) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteControl(
		closeMessage,
		closeFrame(closeNormalClosure),
		time.Now().Add(s.opts.WriteTimeout),
	)
}

// closeFrame builds the two-byte payload of a close control frame.
func closeFrame(code int) []byte {
	return []byte{byte(code >> 8), byte(code)}
}

// ping sends one control frame under the write lock, so it cannot interleave
// with an application message.
func (s *Socket[In, Out]) ping() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrSocketClosed
	}
	return s.conn.WriteControl(pingMessage, nil, time.Now().Add(s.opts.WriteTimeout))
}

// ServeSocket applies the lifecycle to conn, runs fn, and closes the socket
// whatever fn returns.
//
// Both surfaces call this, so the limits, the pong accounting, the ping
// cadence and the close handshake are one implementation rather than two.
// Closing the connection stays with the caller: fasthttp closes it when its
// upgrade callback returns, and net/http has to be told.
func ServeSocket[In, Out any](conn MessageConn, opts SocketOptions, fn func(*Socket[In, Out]) error) error {
	socket := NewSocket[In, Out](conn, opts)
	conn.SetReadLimit(opts.ReadLimit)

	// A pong does not return from ReadMessage, so without this a connection
	// answering every ping still expires on the deadline Read installed. The
	// handler runs between reads, which is where a deadline can still land.
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(opts.IdleTimeout))
	})

	if opts.PingInterval > 0 {
		stop := make(chan struct{})
		defer close(stop)
		go func() {
			ticker := time.NewTicker(opts.PingInterval)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					if err := socket.ping(); err != nil {
						return
					}
				}
			}
		}()
	}

	err := fn(socket)
	if cerr := socket.Close(); err == nil {
		err = cerr
	}
	return err
}
