package app

import (
	"net/http"

	"github.com/shibukawa/httpbind-go"
)

type SearchRequest struct {
	Q string
}
type SearchResponse struct{}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbinder.Bind[SearchRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = httpbinder.Write[SearchResponse](w, r, SearchResponse{})
}

func register(mux *http.ServeMux) {
	mux.Handle(
		"GET /search",
		http.AllowQuerySemicolons(
			http.HandlerFunc(searchHandler),
		),
	)
}
