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

func chatHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbinder.Bind[ChatRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = input
	var stream httpbinder.Stream[ChatEvent]
	_ = httpbinder.Write[httpbinder.Stream[ChatEvent]](w, r, stream)
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("POST /chat", chatHandler)
}
