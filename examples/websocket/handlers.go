package main

import (
	"net/http"

	httpbind "github.com/shibukawa/tinybind-go"
)

// socketOf names the socket type once so the hub and the handler agree.
type socketOf = httpbind.Socket[ClientMsg, ServerMsg]

var chat = newHub()

// healthHandler is an ordinary REST route, sharing the port with the socket.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	_ = httpbind.Write[HealthResponse](w, r, HealthResponse{
		Status: "ok",
		Live:   chat.size(),
	})
}

// chatHandler upgrades to a WebSocket and runs the room protocol.
//
// Neither type argument is spelled at the call: generation recovers ClientMsg
// and ServerMsg from the closure parameter and emits a decoder for the first
// and an encoder for the second.
func chatHandler(w http.ResponseWriter, r *http.Request) {
	// Anything from the request has to be read here. On the fasthttp backend
	// the callback runs after the handler has returned, so reaching for r
	// inside it would read whichever request occupies that slot next.
	peer := r.RemoteAddr

	err := httpbind.WebSocket(w, r, func(s *socketOf) error {
		defer func() {
			if name, ok := chat.name(s); ok {
				chat.leave(s)
				chat.broadcast(ServerMsg{Type: "presence", Text: name + " left", Live: chat.size()})
			}
		}()

		for {
			in, err := s.Read()
			if err != nil {
				// A peer that went away, or one that idled past the timeout,
				// ends the loop. Returning the error routes it to the handler
				// installed with SetStreamErrorHandler; returning nil says
				// this was an ordinary goodbye.
				return nil
			}

			switch in.Type {
			case "join":
				if in.Name == "" {
					if err := s.Write(ServerMsg{Type: "error", Code: "name_required"}); err != nil {
						return err
					}
					continue
				}
				chat.join(s, in.Name)
				if err := s.Write(ServerMsg{Type: "welcome", Text: in.Name, Live: chat.size()}); err != nil {
					return err
				}
				chat.broadcast(ServerMsg{Type: "presence", Text: in.Name + " joined", Live: chat.size()})

			case "say":
				name, ok := chat.name(s)
				if !ok {
					if err := s.Write(ServerMsg{Type: "error", Code: "join_first"}); err != nil {
						return err
					}
					continue
				}
				// Every other member is written to from this goroutine. That
				// is the whole payoff of the socket serializing writes.
				chat.broadcast(ServerMsg{Type: "message", From: name, Text: in.Text})

			case "leave":
				return nil

			default:
				if err := s.Write(ServerMsg{Type: "error", Code: "unknown_type"}); err != nil {
					return err
				}
			}
		}
	})
	if err != nil {
		// The refusal response is already written; this is for the log.
		logf("handshake from %s refused: %v", peer, err)
	}
}
