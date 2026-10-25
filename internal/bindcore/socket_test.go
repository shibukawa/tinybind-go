package bindcore_test

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/internal/bindcore"
	"github.com/shibukawa/tinybind-go/jsonbind"
)

type msg struct {
	Text string
}

func init() {
	jsonbind.RegisterEncode(func(w io.Writer, v msg) error {
		// Written in two pieces on purpose: a serialization defect shows up as
		// interleaving, and one Write call could hide it.
		if _, err := io.WriteString(w, `{"text":"`); err != nil {
			return err
		}
		_, err := io.WriteString(w, v.Text+`"}`)
		return err
	})
	jsonbind.RegisterDecode(func(data []byte) (msg, error) {
		text := string(data)
		const prefix = `{"text":"`
		if !strings.HasPrefix(text, prefix) || !strings.HasSuffix(text, `"}`) {
			return msg{}, errors.New("bad test document")
		}
		return msg{Text: text[len(prefix) : len(text)-2]}, nil
	})
}

// fakeConn records what a socket did to a connection and reports a frame
// opened while another was still open.
type fakeConn struct {
	mu sync.Mutex

	inbound      []inboundFrame
	readIndex    int
	readDeadline []time.Time
	frames       []string
	controls     []control
	readLimit    int64
	pong         func(string) error
	closed       bool

	openFrames  atomic.Int32
	interleaved atomic.Bool
}

type inboundFrame struct {
	messageType int
	payload     string
	err         error
}

type control struct {
	messageType int
	payload     []byte
}

func (c *fakeConn) NextReader() (int, io.Reader, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.readIndex >= len(c.inbound) {
		return 0, nil, io.EOF
	}
	frame := c.inbound[c.readIndex]
	c.readIndex++
	if frame.err != nil {
		return 0, nil, frame.err
	}
	// A reader with no Len method, so the socket cannot size its buffer from
	// the payload and has to count what it is handed, the way it must with a
	// decompressing reader in front of the wire.
	return frame.messageType, io.MultiReader(strings.NewReader(frame.payload)), nil
}

func (c *fakeConn) NextWriter(int) (io.WriteCloser, error) {
	if c.openFrames.Add(1) > 1 {
		c.interleaved.Store(true)
	}
	return &fakeFrame{conn: c}, nil
}

func (c *fakeConn) WriteControl(messageType int, data []byte, _ time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.controls = append(c.controls, control{messageType: messageType, payload: data})
	return nil
}

func (c *fakeConn) SetReadLimit(limit int64) { c.readLimit = limit }

func (c *fakeConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readDeadline = append(c.readDeadline, t)
	return nil
}

func (c *fakeConn) SetWriteDeadline(time.Time) error { return nil }

func (c *fakeConn) SetPongHandler(h func(string) error) { c.pong = h }

func (c *fakeConn) Subprotocol() string { return "chat.v1" }

func (c *fakeConn) Close() error { c.closed = true; return nil }

func (c *fakeConn) writtenFrames() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.frames...)
}

type fakeFrame struct {
	conn *fakeConn
	buf  strings.Builder
}

func (f *fakeFrame) Write(p []byte) (int, error) {
	// Widen the window a real encoder would leave between its pieces.
	time.Sleep(time.Microsecond)
	return f.buf.Write(p)
}

func (f *fakeFrame) Close() error {
	f.conn.mu.Lock()
	f.conn.frames = append(f.conn.frames, f.buf.String())
	f.conn.mu.Unlock()
	f.conn.openFrames.Add(-1)
	return nil
}

func testOptions() bindcore.SocketOptions {
	return bindcore.ResolveSocketOptions(bindcore.SocketOptions{})
}

func TestWriteIsSafeFromManyGoroutines(t *testing.T) {
	conn := &fakeConn{}
	socket := bindcore.NewSocket[msg, msg](conn, testOptions())

	const writers, each = 8, 20
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < each; j++ {
				if err := socket.Write(msg{Text: fmt.Sprintf("w%d-%d", i, j)}); err != nil {
					t.Errorf("write: %v", err)
					return
				}
			}
		}(i)
	}
	wg.Wait()

	if conn.interleaved.Load() {
		t.Fatal("two frames were open at once: writes are not serialized")
	}
	frames := conn.writtenFrames()
	if len(frames) != writers*each {
		t.Fatalf("frames = %d, want %d", len(frames), writers*each)
	}
	for _, frame := range frames {
		if !strings.HasPrefix(frame, `{"text":"`) || !strings.HasSuffix(frame, `"}`) {
			t.Fatalf("frame is not one whole document: %q", frame)
		}
	}
}

func TestReadArmsTheDeadlineBeforeEveryRead(t *testing.T) {
	conn := &fakeConn{inbound: []inboundFrame{
		{messageType: 1, payload: `{"text":"one"}`},
		{messageType: 1, payload: `{"text":"two"}`},
	}}
	socket := bindcore.NewSocket[msg, msg](conn, testOptions())

	for want := range []string{"one", "two"} {
		_ = want
		if _, err := socket.Read(); err != nil {
			t.Fatalf("read: %v", err)
		}
	}

	// netdev takes a deadline by value when a read begins, so a bound set once
	// at setup would not cover the second read.
	if len(conn.readDeadline) != 2 {
		t.Fatalf("read deadlines set = %d, want one per read", len(conn.readDeadline))
	}
	for i, deadline := range conn.readDeadline {
		if deadline.IsZero() {
			t.Fatalf("deadline %d is zero: an unbounded read cannot be recovered", i)
		}
	}
}

func TestReadDecodesInboundAndRefusesBinary(t *testing.T) {
	conn := &fakeConn{inbound: []inboundFrame{
		{messageType: 1, payload: `{"text":"hello"}`},
		{messageType: 2, payload: "\x00\x01"},
	}}
	socket := bindcore.NewSocket[msg, msg](conn, testOptions())

	got, err := socket.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Text != "hello" {
		t.Fatalf("Text = %q, want hello", got.Text)
	}

	if _, err := socket.Read(); err == nil {
		t.Fatal("a binary frame on a JSON socket should be an error")
	} else if problem, ok := bindcore.AsHTTPError(err); !ok || problem.Problem.Code != "websocket_frame" {
		t.Fatalf("error = %v, want a websocket_frame problem", err)
	}
}

func TestCloseSendsANormalClosureOnceAndStopsTheSocket(t *testing.T) {
	conn := &fakeConn{}
	socket := bindcore.NewSocket[msg, msg](conn, testOptions())

	if err := socket.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := socket.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}

	if len(conn.controls) != 1 {
		t.Fatalf("control frames = %d, want exactly one close", len(conn.controls))
	}
	sent := conn.controls[0]
	if sent.messageType != 8 {
		t.Fatalf("control opcode = %d, want 8 (close)", sent.messageType)
	}
	if len(sent.payload) != 2 || int(sent.payload[0])<<8|int(sent.payload[1]) != 1000 {
		t.Fatalf("close payload = %v, want code 1000", sent.payload)
	}
	// Closing the socket must not close the transport: the entry that opened
	// the connection is what closes it, after the close frame has gone out.
	if conn.closed {
		t.Fatal("Close closed the underlying connection")
	}
	if _, err := socket.Read(); err == nil {
		t.Fatal("read after close should fail")
	}
	if err := socket.Write(msg{Text: "late"}); err == nil {
		t.Fatal("write after close should fail")
	}
}

func TestServeSocketAppliesTheLifecycleAndClosesWhateverTheCallbackReturns(t *testing.T) {
	conn := &fakeConn{}
	opts := testOptions()
	opts.PingInterval = 0 // no pinger; the cadence has its own coverage below

	want := errors.New("callback failed")
	err := bindcore.ServeSocket(conn, opts, func(s *bindcore.Socket[msg, msg]) error {
		if s.Subprotocol() != "chat.v1" {
			t.Errorf("Subprotocol = %q", s.Subprotocol())
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want the callback's own", err)
	}
	if conn.readLimit != bindcore.DefaultSocketReadLimit {
		t.Fatalf("read limit = %d, want the default applied", conn.readLimit)
	}
	if conn.pong == nil {
		t.Fatal("no pong handler installed: a live connection would idle out")
	}
	if len(conn.controls) != 1 || conn.controls[0].messageType != 8 {
		t.Fatalf("controls = %v, want the close frame sent despite the failure", conn.controls)
	}
}

func TestThePongHandlerPushesTheReadDeadline(t *testing.T) {
	conn := &fakeConn{}
	opts := testOptions()
	opts.PingInterval = 0

	_ = bindcore.ServeSocket(conn, opts, func(*bindcore.Socket[msg, msg]) error { return nil })

	before := len(conn.readDeadline)
	if err := conn.pong(""); err != nil {
		t.Fatalf("pong handler: %v", err)
	}
	// A pong never returns from ReadMessage, so without this a connection
	// answering every ping still expires on the deadline Read installed.
	if len(conn.readDeadline) != before+1 {
		t.Fatal("the pong handler did not push the read deadline forward")
	}
}

func TestOptionDefaultsAndValidation(t *testing.T) {
	opts := bindcore.ResolveSocketOptions(bindcore.SocketOptions{})
	if opts.ReadLimit == 0 || opts.IdleTimeout == 0 || opts.WriteTimeout == 0 || opts.ReadBufferSize == 0 {
		t.Fatalf("a zero survived resolution: %+v", opts)
	}
	if opts.PingInterval >= opts.IdleTimeout {
		t.Fatalf("default ping %v is not shorter than idle %v", opts.PingInterval, opts.IdleTimeout)
	}
	if err := bindcore.ValidateSocketOptions(opts); err != nil {
		t.Fatalf("the defaults do not validate: %v", err)
	}

	// A ping at or above the idle bound only fires after the read it was
	// meant to keep alive has already timed out.
	bad := opts
	bad.PingInterval = opts.IdleTimeout
	if err := bindcore.ValidateSocketOptions(bad); err == nil {
		t.Fatal("a ping interval at the idle timeout should be refused")
	}
}

func TestCheckOriginDefault(t *testing.T) {
	cases := []struct {
		origin, host string
		want         bool
	}{
		{"", "example.com", true},
		{"https://example.com", "example.com", true},
		{"https://EXAMPLE.com", "example.com", true},
		{"https://example.com:8443", "example.com:8443", true},
		{"https://example.com/some/path", "example.com", true},
		{"https://evil.example", "example.com", false},
		{"https://example.com", "example.com:8443", false},
	}
	for _, tc := range cases {
		if got := bindcore.CheckOriginDefault(tc.origin, tc.host); got != tc.want {
			t.Errorf("CheckOriginDefault(%q, %q) = %v, want %v", tc.origin, tc.host, got, tc.want)
		}
	}
}

// A payload of exactly n bytes that the test codec decodes.
func documentOfLength(n int) string {
	const overhead = len(`{"text":"`) + len(`"}`)
	return `{"text":"` + strings.Repeat("a", n-overhead) + `"}`
}

func closeCode(c control) int {
	if c.messageType != 8 || len(c.payload) != 2 {
		return -1
	}
	return int(c.payload[0])<<8 | int(c.payload[1])
}

func TestReadBoundsTheMessageByTheBytesItIsHanded(t *testing.T) {
	// The fake hands the socket a reader with no length, the position a
	// decompressing reader puts it in: the only size it can enforce is the one
	// it counts. A driver limit on the wire bytes never sees this length.
	opts := testOptions()
	opts.ReadLimit = 64
	conn := &fakeConn{inbound: []inboundFrame{
		{messageType: 1, payload: documentOfLength(64)},
		{messageType: 1, payload: documentOfLength(65)},
		{messageType: 1, payload: documentOfLength(16)},
	}}
	socket := bindcore.NewSocket[msg, msg](conn, opts)

	if _, err := socket.Read(); err != nil {
		t.Fatalf("a message exactly at the limit must be delivered: %v", err)
	}

	_, err := socket.Read()
	if err == nil {
		t.Fatal("a message one byte over the limit was delivered")
	}
	if !errors.Is(err, bindcore.ErrReadLimit) {
		t.Fatalf("err = %v, want ErrReadLimit as its cause", err)
	}
	problem, ok := bindcore.AsHTTPError(err)
	if !ok || problem.Problem.Code != "websocket_read_limit" || problem.Status != 413 {
		t.Fatalf("err = %v, want a 413 websocket_read_limit problem", err)
	}

	// The peer is told why, with the code the driver itself uses, exactly once.
	if len(conn.controls) != 1 || closeCode(conn.controls[0]) != 1009 {
		t.Fatalf("controls = %v, want one close frame carrying 1009", conn.controls)
	}
	// The socket is over: the next message, though small, is not read, and
	// Close adds no second close frame to the one already sent.
	if _, err := socket.Read(); err == nil {
		t.Fatal("read after a refused message should fail")
	}
	if err := socket.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if len(conn.controls) != 1 {
		t.Fatalf("controls = %v, want no normal-closure frame after the 1009", conn.controls)
	}
}

func TestServeSocketReportsAnOversizeMessageAndSendsOneCloseFrame(t *testing.T) {
	opts := testOptions()
	opts.ReadLimit = 64
	opts.PingInterval = 0
	conn := &fakeConn{inbound: []inboundFrame{
		{messageType: 1, payload: documentOfLength(65)},
	}}

	err := bindcore.ServeSocket(conn, opts, func(s *bindcore.Socket[msg, msg]) error {
		_, err := s.Read()
		return err
	})
	// A refused message is a failure the stream error handler should see, not
	// a normal close to be swallowed.
	if !errors.Is(err, bindcore.ErrReadLimit) {
		t.Fatalf("ServeSocket returned %v, want the read-limit error", err)
	}
	if bindcore.IsNormalClose(err) {
		t.Fatal("a read-limit refusal was classified as a normal close")
	}
	if len(conn.controls) != 1 || closeCode(conn.controls[0]) != 1009 {
		t.Fatalf("controls = %v, want exactly one 1009 close frame", conn.controls)
	}
}

var errFakeDriverLimit = errors.New("fake driver: read limit exceeded")

func TestTheDriverOwnReadLimitRefusalIsTheSameError(t *testing.T) {
	// The transport registers the driver's sentinel, the way it registers the
	// driver's CloseError for IsNormalClose. The fake stands in for that
	// transport here.
	bindcore.RegisterReadLimit(func(err error) bool { return errors.Is(err, errFakeDriverLimit) })

	conn := &fakeConn{inbound: []inboundFrame{{err: errFakeDriverLimit}}}
	socket := bindcore.NewSocket[msg, msg](conn, testOptions())

	_, err := socket.Read()
	if !errors.Is(err, bindcore.ErrReadLimit) {
		t.Fatalf("err = %v, want ErrReadLimit as its cause", err)
	}
	if problem, ok := bindcore.AsHTTPError(err); !ok || problem.Problem.Code != "websocket_read_limit" {
		t.Fatalf("err = %v, want the websocket_read_limit problem", err)
	}
	// The driver sent its own 1009 before returning, so the socket sends
	// nothing: not a second 1009, and not a normal closure from Close.
	if err := socket.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if len(conn.controls) != 0 {
		t.Fatalf("controls = %v, want none after the driver's own refusal", conn.controls)
	}
}
