package app

import (
	"net/http"

	"github.com/shibukawa/httpbind-go"
)

type APIRequest struct{}
type APIResponse struct{}

func apiHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbinder.Bind[APIRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = httpbinder.Write[APIResponse](w, r, APIResponse{})
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
