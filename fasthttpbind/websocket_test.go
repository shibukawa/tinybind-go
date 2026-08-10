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

	httpbind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/fasthttpbind"
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
