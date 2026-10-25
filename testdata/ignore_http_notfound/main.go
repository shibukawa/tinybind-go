package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type ItemRequest struct{}
type ItemResponse struct{}

func itemHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbind.Bind[ItemRequest](r)
	if err != nil {
		// stdlib http.NotFound must not be treated as httpbind error discovery
		http.NotFound(w, r)
		return
	}
	_ = httpbind.Write[ItemResponse](w, r, ItemResponse{})
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("GET /items/{id}", itemHandler)
}
