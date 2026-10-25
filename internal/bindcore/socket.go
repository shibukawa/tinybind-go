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
//
// It takes NextReader rather than ReadMessage on purpose. ReadMessage reads a
// whole message into memory before the caller can see it, and the driver's
// read limit counts the bytes on the wire: with permessage-deflate negotiated
// those are the compressed bytes, so a 33 KB frame can inflate to 32 MiB
// without the limit noticing. A reader lets the socket count the bytes it is
// actually handed and stop at the limit the options state.
type MessageConn interface {
	NextReader() (messageType int, r io.Reader, err error)
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
	closeMessageTooBig = 1009
)

var (
	ErrNilSocket    = errors.New("httpbind: nil socket")
	ErrSocketClosed = errors.New("httpbind: socket closed")
	// ErrBinaryMessage reports a binary frame on a socket carrying JSON.
	ErrBinaryMessage = errors.New("httpbind: binary message on a JSON socket")
	// ErrReadLimit reports an inbound message larger than ReadLimit. It is the
	// cause of the error Read returns whether the driver refused the frame by
	// its wire length or the socket refused the message by its inflated
	// length, so one check covers both transports and both encodings.
	ErrReadLimit = errors.New("httpbind: websocket message exceeds the read limit")
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
// ReadLimit is enforced here, on the bytes the message yields, and not only by
// the driver. The driver's limit sums frame payload lengths as they appear on
// the wire, which with permessage-deflate is the compressed size; a message at
// deflate's best ratio inflates about a thousandfold past it. Counting what
// the reader hands over bounds the message the application receives, which is
// the size the option names. The driver's check stays as the early refusal: a
// wire length over the limit is refused before any payload is read.
//
// A message over the limit ends the socket the way the driver ends one: a
// close frame carrying 1009 (message too big), then an error whose cause is
// ErrReadLimit. The driver's own refusal is reshaped to the same error, so a
// handler tests one sentinel on either transport.
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
	messageType, r, err := s.conn.NextReader()
	if err != nil {
		return zero, s.readError(err)
	}
	if messageType != textMessage {
		return zero, BadRequest(Problem{
			Code:    "websocket_frame",
			Message: "expected a text frame carrying JSON",
		}, ErrBinaryMessage)
	}
	// ReadLimitHint reads one byte past the limit to tell "at it" from "over
	// it", so a message exactly ReadLimit long is still delivered.
	data, err := jsonbind.ReadLimitHint(r, s.opts.ReadLimit, 0)
	if err != nil {
		if errors.Is(err, jsonbind.ErrBodyTooLarge) {
			s.closeTooBig()
			return zero, readLimitError()
		}
		return zero, s.readError(err)
	}
	return jsonbind.DecodeJSONBytes[In](data)
}

// readError reshapes a driver read failure. The driver's own read-limit
// refusal has already sent the 1009 close frame, so the socket is marked
// closed — a second close frame from Close would be a protocol error — and
// the error becomes the one the inflated-length check returns.
func (s *Socket[In, Out]) readError(err error) error {
	if isDriverReadLimit(err) {
		s.closed.Store(true)
		return readLimitError()
	}
	return err
}

// closeTooBig sends the close frame for a message over the limit, once, and
// marks the socket closed so no normal-closure frame follows it.
func (s *Socket[In, Out]) closeTooBig() {
	if !s.closed.CompareAndSwap(false, true) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.conn.WriteControl(
		closeMessage,
		closeFrame(closeMessageTooBig),
		time.Now().Add(s.opts.WriteTimeout),
	)
}

// readLimitError is the error a message over ReadLimit produces. It carries a
// Problem like the socket's other refusals, with the status that close code
// 1009 corresponds to, and ErrReadLimit as its cause.
func readLimitError() error {
	return PayloadTooLarge(Problem{
		Code:    "websocket_read_limit",
		Message: "message exceeds the read limit",
	}, ErrReadLimit)
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
	// A socket ends when the peer closes it, and that is not a failure to route
	// anywhere. The peer-initiated close arrives as a normal CloseError from the
	// read, and our own answering Close then reports that the frame was already
	// sent; neither is reportable. The classifier is transport-owned because the
	// CloseError types are the drivers' and differ between them — which is why a
	// handler comparing against one driver's type misfired on the other.
	if IsNormalClose(err) {
		return nil
	}
	return err
}

// normalCloseClassifiers hold each transport's rule for what counts as a normal
// end of a socket. bindcore names no driver, so the driver-specific check — a
// CloseError of a normal or going-away code, or the sentinel returned when a
// close was already sent — is registered by the transport that imports it.
var normalCloseClassifiers []func(error) bool

// RegisterNormalClose adds a transport's normal-close rule. Each transport
// package calls it once from init, so IsNormalClose answers the same on both
// surfaces for the callback body they share.
func RegisterNormalClose(fn func(error) bool) {
	if fn != nil {
		normalCloseClassifiers = append(normalCloseClassifiers, fn)
	}
}

// readLimitClassifiers hold each transport's rule for recognizing the driver's
// own read-limit refusal, which is a sentinel of the driver's package and so,
// like the CloseError types, is registered by the transport that imports it.
var readLimitClassifiers []func(error) bool

// RegisterReadLimit adds a transport's rule for the driver's read-limit error.
// Each transport package calls it once from init, so Read returns one error
// for a message over the limit whichever transport and whichever check —
// the driver's wire length or the socket's inflated length — refused it.
func RegisterReadLimit(fn func(error) bool) {
	if fn != nil {
		readLimitClassifiers = append(readLimitClassifiers, fn)
	}
}

func isDriverReadLimit(err error) bool {
	if err == nil {
		return false
	}
	for _, fn := range readLimitClassifiers {
		if fn(err) {
			return true
		}
	}
	return false
}

// IsNormalClose reports whether err is the ordinary end of a socket rather than
// a failure — the peer's normal or going-away close, an already-sent close, or
// EOF. A handler's read loop tests it to end without routing the close to the
// error handler, and it answers alike on both transports where a driver's own
// IsCloseError does not.
func IsNormalClose(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	for _, fn := range normalCloseClassifiers {
		if fn(err) {
			return true
		}
	}
	return false
}
