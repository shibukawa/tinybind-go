package app

import (
	"net/http"

	"github.com/shibukawa/httpbind-go"
)

type ItemRequest struct{}
type ItemResponse struct{}

func itemHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbinder.Bind[ItemRequest](r)
	if err != nil {
		// stdlib http.NotFound must not be treated as httpbinder error discovery
		http.NotFound(w, r)
		return
	}
	_ = httpbinder.Write[ItemResponse](w, r, ItemResponse{})
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("GET /items/{id}", itemHandler)
}
