package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type ChatRequest struct {
	Message string
}

type ChatEvent struct {
	Type  string
	Delta string
}

// The callback entry, spelled the way a handler actually writes it: with the
// element type inferred from the closure parameter rather than named. Discovery
// has to recover ChatEvent from the instantiation, because nothing in the call
// says it.
func chatHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbind.Bind[ChatRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = input

	httpbind.WriteStream(w, r, func(s *httpbind.Stream[ChatEvent]) error {
		if err := s.Write(ChatEvent{Type: "delta", Delta: "hi"}); err != nil {
			return err
		}
		return s.Write(ChatEvent{Type: "done"})
	})
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("POST /chat", chatHandler)
}
