package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type APIRequest struct{}
type APIResponse struct{}

func apiHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbind.Bind[APIRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[APIResponse](w, r, APIResponse{})
}

func register(mux *http.ServeMux) {
	mux.Handle(
		"POST /api/",
		http.StripPrefix(
			"/api",
			http.HandlerFunc(apiHandler),
		),
	)
}
