package app

import "net/http"

func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("GET /ping", pingHandler)
}

// registerAgain repeats register's pattern, so the two routes differ only by
// their registration site.
func registerAgain(mux *http.ServeMux) {
	mux.HandleFunc("GET /ping", pingHandler)
}
