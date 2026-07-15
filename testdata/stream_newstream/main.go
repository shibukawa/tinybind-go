package app

import (
	"net/http"

	"github.com/shibukawa/httpbind-go"
)

type ChatRequest struct {
	Message string
}

type ChatEvent struct {
	Type  string
	Delta string
}

// Preferred incremental streaming path: NewStream + repeated Write.
func chatHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbinder.Bind[ChatRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = input

	stream, err := httpbinder.NewStream[ChatEvent](w, r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	defer stream.Close()

	_ = stream.Write(ChatEvent{Type: "delta", Delta: "hi"})
	_ = stream.Write(ChatEvent{Type: "done"})
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("POST /chat", chatHandler)
}
