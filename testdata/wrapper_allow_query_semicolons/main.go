package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type SearchRequest struct {
	Q string
}
type SearchResponse struct{}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbind.Bind[SearchRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[SearchResponse](w, r, SearchResponse{})
}

func register(mux *http.ServeMux) {
	mux.Handle(
		"GET /search",
		http.AllowQuerySemicolons(
			http.HandlerFunc(searchHandler),
		),
	)
}
