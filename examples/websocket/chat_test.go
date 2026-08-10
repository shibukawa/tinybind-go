package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/tinygodriver/websocket"
)

// serveChat runs the example's own handlers on a real listener, because the
// upgrade hijacks the connection.
func serveChat(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", chatHandler)
	mux.HandleFunc("GET /healthz", healthHandler)
	server := httptest.NewUnstartedServer(mux)
	server.Start()
	t.Cleanup(server.Close)
	return server.URL
}

func dial(t *testing.T, base string) *websocket.Conn {
	t.Helper()
	conn, _, err := (&websocket.Dialer{HandshakeTimeout: 5 * time.Second}).
		Dial("ws"+strings.TrimPrefix(base, "http")+"/ws", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func send(t *testing.T, conn *websocket.Conn, document string) {
	t.Helper()
	if err := conn.WriteMessage(websocket.TextMessage, []byte(document)); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func recv(t *testing.T, conn *websocket.Conn) ServerMsg {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	var msg ServerMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("recv %s: %v", data, err)
	}
	return msg
}

func TestChatBroadcastsAcrossConnections(t *testing.T) {
	base := serveChat(t)

	alice := dial(t, base)
	send(t, alice, `{"type":"join","name":"alice"}`)
	if got := recv(t, alice); got.Type != "welcome" || got.Text != "alice" {
		t.Fatalf("alice welcome = %+v", got)
	}
	if got := recv(t, alice); got.Type != "presence" {
		t.Fatalf("alice presence = %+v", got)
	}

	bob := dial(t, base)
	send(t, bob, `{"type":"join","name":"bob"}`)
	if got := recv(t, bob); got.Type != "welcome" || got.Live != 2 {
		t.Fatalf("bob welcome = %+v", got)
	}
	// bob joining reaches alice, which is a write into alice's socket made
	// from bob's goroutine.
	if got := recv(t, alice); got.Type != "presence" || got.Text != "bob joined" {
		t.Fatalf("alice heard = %+v", got)
	}
	_ = recv(t, bob) // bob's own presence

	send(t, alice, `{"type":"say","text":"hello room"}`)
	for name, conn := range map[string]*websocket.Conn{"alice": alice, "bob": bob} {
		got := recv(t, conn)
		if got.Type != "message" || got.From != "alice" || got.Text != "hello room" {
			t.Fatalf("%s got %+v", name, got)
		}
	}
}

func TestSpeakingBeforeJoiningIsRefused(t *testing.T) {
	base := serveChat(t)
	carol := dial(t, base)
	send(t, carol, `{"type":"say","text":"sneaky"}`)
	if got := recv(t, carol); got.Type != "error" || got.Code != "join_first" {
		t.Fatalf("got %+v, want a join_first error", got)
	}
}

func TestUnknownMessageTypeIsAnsweredRatherThanClosing(t *testing.T) {
	base := serveChat(t)
	conn := dial(t, base)
	send(t, conn, `{"type":"shout","text":"?"}`)
	if got := recv(t, conn); got.Code != "unknown_type" {
		t.Fatalf("got %+v, want an unknown_type error", got)
	}
	// The socket survives a protocol error: it is the application's to answer.
	send(t, conn, `{"type":"join","name":"dave"}`)
	if got := recv(t, conn); got.Type != "welcome" {
		t.Fatalf("got %+v, want the socket still usable", got)
	}
}

func TestLeavingIsAnnouncedAndCountedOut(t *testing.T) {
	base := serveChat(t)

	alice := dial(t, base)
	send(t, alice, `{"type":"join","name":"alice"}`)
	_, _ = recv(t, alice), recv(t, alice)

	bob := dial(t, base)
	send(t, bob, `{"type":"join","name":"bob"}`)
	_, _, _ = recv(t, bob), recv(t, alice), recv(t, bob)

	send(t, alice, `{"type":"leave"}`)
	if got := recv(t, bob); got.Type != "presence" || got.Text != "alice left" || got.Live != 1 {
		t.Fatalf("bob heard %+v", got)
	}
}

func TestHealthRouteSharesThePortWithTheSocket(t *testing.T) {
	base := serveChat(t)

	resp, err := http.Get(base + "/healthz")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("health = %d %s", resp.StatusCode, body)
	}
}

func TestCrossOriginHandshakeIsRefusedAsProblemDetails(t *testing.T) {
	base := serveChat(t)

	header := http.Header{}
	header.Set("Origin", "https://evil.example")
	conn, resp, err := (&websocket.Dialer{HandshakeTimeout: 5 * time.Second}).
		Dial("ws"+strings.TrimPrefix(base, "http")+"/ws", header)
	if err == nil {
		_ = conn.Close()
		t.Fatal("a cross-origin handshake should be refused")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "websocket_origin") {
		t.Fatalf("body = %s, want the websocket_origin code", body)
	}
}
