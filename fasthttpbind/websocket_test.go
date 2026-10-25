package fasthttpbind_test

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/fasthttpbind"
	"github.com/shibukawa/tinybind-go/httpbind"
	"github.com/shibukawa/tinybind-go/internal/bindcore"
	"github.com/shibukawa/tinybind-go/jsonbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
	"github.com/shibukawa/tinygodriver/websocket"
)

type sockIn struct {
	Text string
}

type sockOut struct {
	Text string
}

func init() {
	jsonbind.RegisterDecode(func(data []byte) (sockIn, error) {
		var v struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return sockIn{}, err
		}
		return sockIn{Text: v.Text}, nil
	})
	jsonbind.RegisterEncode(func(w io.Writer, v sockOut) error {
		_, err := w.Write([]byte(`{"text":"` + v.Text + `"}`))
		return err
	})
}

// echoSocket is one function value used as the callback on both transports.
// That it type-checks against each surface's Socket alias is the callback
// shape paying off: a rewrite changes the entry call, never this body.
func echoSocket(s *bindcore.Socket[sockIn, sockOut]) error {
	for {
		in, err := s.Read()
		if err != nil {
			// A peer that went away ends the loop; it is not a failure to
			// route anywhere.
			if errors.Is(err, io.EOF) || websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			return err
		}
		if err := s.Write(sockOut{Text: "echo:" + in.Text}); err != nil {
			return err
		}
	}
}

// netHTTPSocketServer serves the callback on net/http. httptest's writer
// implements http.Hijacker, which is what the entry asserts before upgrading.
func netHTTPSocketServer(t *testing.T, opts fasthttpbind.SocketOptions) (url string, refusals chan error) {
	t.Helper()
	refusals = make(chan error, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := httpbind.WebSocketWith(w, r, opts, echoSocket); err != nil {
			refusals <- err
		}
	}))
	t.Cleanup(server.Close)
	return "ws" + strings.TrimPrefix(server.URL, "http"), refusals
}

// fasthttpSocketServer serves the same callback on fasthttp, over a real
// listener because the upgrade hijacks the connection.
func fasthttpSocketServer(t *testing.T, opts fasthttpbind.SocketOptions) (url string, refusals chan error) {
	t.Helper()
	refusals = make(chan error, 4)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &fasthttp.Server{Handler: func(ctx *fasthttp.RequestCtx) {
		if err := fasthttpbind.WebSocketWith(ctx, opts, echoSocket); err != nil {
			refusals <- err
		}
	}}
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(func() { _ = server.Shutdown() })
	return "ws://" + ln.Addr().String(), refusals
}

type socketBackend struct {
	name  string
	serve func(*testing.T, fasthttpbind.SocketOptions) (string, chan error)
}

func socketBackends() []socketBackend {
	return []socketBackend{
		{"net/http", netHTTPSocketServer},
		{"fasthttp", fasthttpSocketServer},
	}
}

func dialSocket(t *testing.T, url string, header http.Header) *websocket.Conn {
	t.Helper()
	dialer := &websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, resp, err := dialer.Dial(url, header)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("dial %s: %v (status %d, body %s)", url, err, resp.StatusCode, body)
		}
		t.Fatalf("dial %s: %v", url, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestSocketRoundTripIsIdenticalOnBothTransports(t *testing.T) {
	for _, backend := range socketBackends() {
		t.Run(backend.name, func(t *testing.T) {
			url, _ := backend.serve(t, fasthttpbind.SocketOptions{})
			conn := dialSocket(t, url, nil)

			for _, text := range []string{"hello", "again", ""} {
				if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"text":"`+text+`"}`)); err != nil {
					t.Fatalf("client write: %v", err)
				}
				_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				messageType, data, err := conn.ReadMessage()
				if err != nil {
					t.Fatalf("client read: %v", err)
				}
				if messageType != websocket.TextMessage {
					t.Fatalf("opcode = %d, want text", messageType)
				}
				if want := `{"text":"echo:` + text + `"}`; string(data) != want {
					t.Fatalf("got %s, want %s", data, want)
				}
			}
		})
	}
}

func TestSocketSendsACloseFrameWhenTheCallbackReturns(t *testing.T) {
	for _, backend := range socketBackends() {
		t.Run(backend.name, func(t *testing.T) {
			url, _ := backend.serve(t, fasthttpbind.SocketOptions{})
			conn := dialSocket(t, url, nil)

			// The client closing makes the server's Read fail, which ends the
			// callback; the runtime is what has to answer with a close frame.
			if err := conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
				t.Fatalf("client close: %v", err)
			}
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, _, err := conn.ReadMessage()
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				t.Fatalf("err = %v, want a normal-closure close frame back", err)
			}
		})
	}
}

func TestRefusedOriginIsProblemDetailsOnBothTransports(t *testing.T) {
	for _, backend := range socketBackends() {
		t.Run(backend.name, func(t *testing.T) {
			url, refusals := backend.serve(t, fasthttpbind.SocketOptions{})

			header := http.Header{}
			header.Set("Origin", "https://evil.example")
			dialer := &websocket.Dialer{HandshakeTimeout: 5 * time.Second}
			conn, resp, err := dialer.Dial(url, header)
			if err == nil {
				_ = conn.Close()
				t.Fatal("a cross-origin handshake should be refused")
			}
			if resp == nil {
				t.Fatalf("no response for the refusal: %v", err)
			}
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", resp.StatusCode)
			}
			if got := resp.Header.Get("Content-Type"); got != bindcore.ProblemContentType {
				t.Fatalf("Content-Type = %q, want %q", got, bindcore.ProblemContentType)
			}
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), "websocket_origin") {
				t.Fatalf("body = %s, want the websocket_origin code", body)
			}

			// The handler is handed the refusal so it can log or count it; the
			// response is already written by then.
			select {
			case refusal := <-refusals:
				problem, ok := bindcore.AsHTTPError(refusal)
				if !ok || problem.Problem.Code != "websocket_origin" {
					t.Fatalf("returned error = %v, want a websocket_origin problem", refusal)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("the entry returned no handshake error")
			}
		})
	}
}

func TestAPlainRequestIsRefusedAsProblemDetails(t *testing.T) {
	url, _ := netHTTPSocketServer(t, fasthttpbind.SocketOptions{})
	resp, err := http.Get("http" + strings.TrimPrefix(url, "ws"))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "websocket_upgrade") {
		t.Fatalf("body = %s, want the websocket_upgrade code", body)
	}
}

func TestANonHijackableWriterIsRefusedRatherThanHanging(t *testing.T) {
	// A TinyGo net/http server cannot hand over the connection, and the
	// handshake would hang with no error. The assertion turns that into an
	// answer; httptest.ResponseRecorder stands in for such a writer.
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Set("Connection", "Upgrade")
	r.Header.Set("Upgrade", "websocket")

	err := httpbind.WebSocket(rec, r, echoSocket)
	if err == nil {
		t.Fatal("a writer that cannot hijack should be refused")
	}
	problem, ok := bindcore.AsHTTPError(err)
	if !ok || problem.Problem.Code != "websocket_hijack" {
		t.Fatalf("error = %v, want a websocket_hijack problem", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// naiveSocket returns the read error straight out, the way the doc example's
// `if err != nil { return err }` does. A normal close must not reach the stream
// error handler even then: ServeSocket recognizes it and ends quietly.
func naiveSocket(s *bindcore.Socket[sockIn, sockOut]) error {
	for {
		in, err := s.Read()
		if err != nil {
			return err
		}
		if err := s.Write(sockOut{Text: in.Text}); err != nil {
			return err
		}
	}
}

func TestNormalCloseIsNotReportedOnEitherTransport(t *testing.T) {
	backends := []socketBackend{
		{"net/http", func(t *testing.T, o fasthttpbind.SocketOptions) (string, chan error) {
			refusals := make(chan error, 4)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := httpbind.WebSocketWith(w, r, o, naiveSocket); err != nil {
					refusals <- err
				}
			}))
			t.Cleanup(server.Close)
			return "ws" + strings.TrimPrefix(server.URL, "http"), refusals
		}},
		{"fasthttp", func(t *testing.T, o fasthttpbind.SocketOptions) (string, chan error) {
			refusals := make(chan error, 4)
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("listen: %v", err)
			}
			server := &fasthttp.Server{Handler: func(ctx *fasthttp.RequestCtx) {
				if err := fasthttpbind.WebSocketWith(ctx, o, naiveSocket); err != nil {
					refusals <- err
				}
			}}
			go func() { _ = server.Serve(ln) }()
			t.Cleanup(func() { _ = server.Shutdown() })
			return "ws://" + ln.Addr().String(), refusals
		}},
	}
	for _, backend := range backends {
		t.Run(backend.name, func(t *testing.T) {
			reported := make(chan error, 4)
			bindcore.SetStreamErrorHandler(func(err error) { reported <- err })
			t.Cleanup(func() { bindcore.SetStreamErrorHandler(nil) })

			url, refusals := backend.serve(t, fasthttpbind.SocketOptions{})
			conn := dialSocket(t, url, nil)
			if err := conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
				t.Fatalf("client close: %v", err)
			}
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, _, _ = conn.ReadMessage() // drain the server's answering close

			select {
			case err := <-reported:
				t.Fatalf("a normal close was reported to the error handler: %v", err)
			case err := <-refusals:
				t.Fatalf("a normal close was returned from the entry: %v", err)
			case <-time.After(300 * time.Millisecond):
			}
		})
	}
}

// IsNormalClose answers the same on both surfaces, which is what lets one Read
// loop end quietly on either transport.
func TestIsNormalCloseParity(t *testing.T) {
	closeErr := &websocket.CloseError{Code: websocket.CloseNormalClosure}
	if !httpbind.IsNormalClose(closeErr) {
		t.Error("net/http IsNormalClose rejected a normal CloseError")
	}
	if !fasthttpbind.IsNormalClose(closeErr) {
		t.Error("fasthttp IsNormalClose rejected a normal CloseError")
	}
	if httpbind.IsNormalClose(errors.New("boom")) || fasthttpbind.IsNormalClose(errors.New("boom")) {
		t.Error("a plain error was classified as a normal close")
	}
}

// dialSocketWith dials with the caller's dialer and also returns the handshake
// response, for a test that has to know what the handshake negotiated.
func dialSocketWith(t *testing.T, url string, dialer *websocket.Dialer) (*websocket.Conn, *http.Response) {
	t.Helper()
	conn, resp, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", url, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, resp
}

// readLimitBackends serve echoSocket with the stream error handler captured,
// so a test can assert what the runtime reported about a refused message.
func readLimitBackends(t *testing.T) (backends []socketBackend, reported chan error) {
	t.Helper()
	reported = make(chan error, 8)
	bindcore.SetStreamErrorHandler(func(err error) { reported <- err })
	t.Cleanup(func() { bindcore.SetStreamErrorHandler(nil) })
	return socketBackends(), reported
}

// The read limit bounds the message the application receives, not the bytes on
// the wire. With permessage-deflate negotiated the two differ by the deflate
// ratio — about a thousand to one for this payload — so a 2 MiB message that
// occupies a few kilobytes on the wire has to be refused by its inflated
// length, which only the socket's own count can see. The plain case is the
// driver's wire-length refusal, reshaped to the same error, on both transports.
func TestAMessageOverTheReadLimitIsRefusedByItsInflatedLengthOnBothTransports(t *testing.T) {
	const readLimit = 64 << 10
	message := `{"text":"` + strings.Repeat("a", 2<<20) + `"}`

	for _, compressed := range []bool{true, false} {
		backends, reported := readLimitBackends(t)
		for _, backend := range backends {
			name := backend.name + "/plain"
			if compressed {
				name = backend.name + "/deflate"
			}
			t.Run(name, func(t *testing.T) {
				url, _ := backend.serve(t, fasthttpbind.SocketOptions{
					ReadLimit:         readLimit,
					EnableCompression: compressed,
				})
				dialer := &websocket.Dialer{HandshakeTimeout: 5 * time.Second, EnableCompression: compressed}
				conn, resp := dialSocketWith(t, url, dialer)
				negotiated := strings.Contains(resp.Header.Get("Sec-WebSocket-Extensions"), "permessage-deflate")
				if negotiated != compressed {
					t.Fatalf("permessage-deflate negotiated = %v, want %v: the test would not exercise the path it names", negotiated, compressed)
				}

				// The server may reset the connection before the whole message
				// is written; what matters is what it did with the part it read.
				_ = conn.WriteMessage(websocket.TextMessage, []byte(message))

				select {
				case err := <-reported:
					if !errors.Is(err, httpbind.ErrReadLimit) || !errors.Is(err, fasthttpbind.ErrReadLimit) {
						t.Fatalf("reported %v, want ErrReadLimit from either surface", err)
					}
					problem, ok := bindcore.AsHTTPError(err)
					if !ok || problem.Problem.Code != "websocket_read_limit" {
						t.Fatalf("reported %v, want the websocket_read_limit problem", err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("the oversize message was not reported: it was read in full")
				}

				// No echo comes back. The peer sees the 1009 close frame when
				// the reset of the unread remainder does not overtake it; a
				// transport error is the other way the same refusal arrives.
				_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				_, data, err := conn.ReadMessage()
				if err == nil {
					t.Fatalf("got a %d-byte message back, want the socket closed", len(data))
				}
				var closeErr *websocket.CloseError
				if errors.As(err, &closeErr) && closeErr.Code != websocket.CloseMessageTooBig {
					t.Fatalf("close code = %d, want 1009", closeErr.Code)
				}
			})
		}
	}
}

// The control for the test above: the same compressed message under a limit
// that admits it is read whole and echoed, so the refusal is the limit and not
// the inflating path.
func TestACompressedMessageWithinTheReadLimitIsDeliveredOnBothTransports(t *testing.T) {
	text := strings.Repeat("a", 2<<20)
	message := `{"text":"` + text + `"}`

	for _, backend := range socketBackends() {
		t.Run(backend.name, func(t *testing.T) {
			url, _ := backend.serve(t, fasthttpbind.SocketOptions{
				ReadLimit:         4 << 20,
				EnableCompression: true,
			})
			dialer := &websocket.Dialer{HandshakeTimeout: 5 * time.Second, EnableCompression: true}
			conn, resp := dialSocketWith(t, url, dialer)
			if !strings.Contains(resp.Header.Get("Sec-WebSocket-Extensions"), "permessage-deflate") {
				t.Fatal("permessage-deflate was not negotiated")
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
				t.Fatalf("client write: %v", err)
			}
			_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			_, data, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("client read: %v", err)
			}
			if want := `{"text":"echo:` + text + `"}`; string(data) != want {
				t.Fatalf("echo is %d bytes, want %d", len(data), len(want))
			}
		})
	}
}
