package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type ChatRequest struct {
	Message string
}

type ChatEvent struct {
	Type  string
	Delta string
}

func chatHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbind.Bind[ChatRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = input
	var stream httpbind.Stream[ChatEvent]
	_ = httpbind.Write[httpbind.Stream[ChatEvent]](w, r, stream)
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("POST /chat", chatHandler)
}
