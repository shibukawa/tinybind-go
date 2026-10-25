package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type SearchRequest struct {
	Q string
}
type SearchResponse struct {
	Hits int
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	req, err := httpbind.Bind[SearchRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = req
	_ = httpbind.Write[SearchResponse](w, r, SearchResponse{Hits: 0})
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("GET /search", searchHandler)
}
