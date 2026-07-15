package app

import (
	"net/http"

	"github.com/shibukawa/httpbind-go"
)

type SearchRequest struct {
	Q string
}
type SearchResponse struct {
	Hits int
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	req, err := httpbinder.Bind[SearchRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = req
	_ = httpbinder.Write[SearchResponse](w, r, SearchResponse{Hits: 0})
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("GET /search", searchHandler)
}
