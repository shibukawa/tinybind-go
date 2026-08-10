package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type ClientMsg struct {
	Type string
	Text string
}

type ServerMsg struct {
	Type string
	Text string
}

// The socket entry, spelled the way a handler actually writes it: with neither
// type argument named. Discovery has to recover ClientMsg from index 0 and
// ServerMsg from index 1 of the same instantiation, which is why the entry
// carries two call patterns against one target.
func chatHandler(w http.ResponseWriter, r *http.Request) {
	_ = httpbind.WebSocket(w, r, func(s *httpbind.Socket[ClientMsg, ServerMsg]) error {
		for {
			in, err := s.Read()
			if err != nil {
				return err
			}
			if err := s.Write(ServerMsg{Type: "echo", Text: in.Text}); err != nil {
				return err
			}
		}
	})
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("GET /ws", chatHandler)
}
